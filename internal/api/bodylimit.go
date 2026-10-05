package api

import (
	"errors"
	"fmt"
	"net/http"
)

// What a request may weigh (alrayyes/hush-hush#617, ADR 31). A secret is a
// token, a key or a small file, not a blob, so a sealed value is capped at
// 1 MiB. A value travels base64-encoded in a JSON body, which is a third
// larger, so the body cap leaves room for that and for the other fields, and
// is the same on every route: nothing here takes a body bigger than one
// maximal value plus its metadata.
const (
	// MaxValueBytes is the largest sealed value, counted as the ciphertext
	// itself, not its base64 form.
	MaxValueBytes = 1 << 20
	// MaxBodyBytes is the largest request body any route reads.
	MaxBodyBytes = MaxValueBytes*3/2 + 64<<10
)

var errBodyTooLarge = fmt.Errorf("request body is larger than the %d byte limit", MaxBodyBytes)

var errValueTooLarge = fmt.Errorf("value is larger than the %d byte limit", MaxValueBytes)

// limitedMux registers each route behind the body cap, so a route added later
// can't forget it. A body that announces itself as too large is refused
// before a byte is read; one that doesn't (chunked) is cut off at the limit
// by the reader, and the handler's decode error becomes the same 413
// (decodeFailure).
type limitedMux struct{ *http.ServeMux }

func (m limitedMux) HandleFunc(pattern string, h http.HandlerFunc) {
	m.ServeMux.Handle(pattern, limitBody(h))
}

func (m limitedMux) Handle(pattern string, h http.Handler) {
	m.ServeMux.Handle(pattern, limitBody(h))
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > MaxBodyBytes {
			writeError(w, r, http.StatusRequestEntityTooLarge, errBodyTooLarge.Error())

			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// isTooLarge reports whether err came from the body cap.
func isTooLarge(err error) bool {
	_, ok := errors.AsType[*http.MaxBytesError](err)

	return ok
}
