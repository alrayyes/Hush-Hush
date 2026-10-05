package api

import (
	"errors"
	"math"
	"net/http"
	"strconv"
)

// Paging for the list endpoints that return every row (alrayyes/hush-hush#649).
// limit and offset are opt-in: a request with neither gets every row, as it
// always did, so no existing client changes. With either, the response is one
// page of the same plain array, and X-Total-Count says how many rows there
// are in all. The array stays the response shape on purpose, so the generated
// SDKs' types don't change.
const (
	// DefaultListPageSize is the page size when offset is given without limit.
	DefaultListPageSize = 50
	// MaxListPageSize is the largest limit a request may ask for.
	MaxListPageSize = 500

	maxListOffset = math.MaxInt32
)

var (
	errInvalidListLimit  = errors.New("limit must be an integer from 1 to 500")
	errInvalidListOffset = errors.New("offset must be a non-negative integer")
)

// writePage answers with items, or the page of them the request asked for. It
// has already answered a request whose limit or offset is out of range.
func writePage[T any](w http.ResponseWriter, r *http.Request, items []T) {
	q := r.URL.Query()
	total := len(items)

	if q.Has("limit") || q.Has("offset") {
		limit, offset, err := pageBounds(q.Get("limit"), q.Get("offset"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())

			return
		}

		items = items[min(offset, total):min(offset+limit, total)]
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, items)
}

func pageBounds(limitParam, offsetParam string) (limit, offset int, err error) {
	limit = DefaultListPageSize

	if limitParam != "" {
		limit, err = strconv.Atoi(limitParam)
		if err != nil || limit < 1 || limit > MaxListPageSize {
			return 0, 0, errInvalidListLimit
		}
	}

	if offsetParam != "" {
		offset, err = strconv.Atoi(offsetParam)
		if err != nil || offset < 0 || offset > maxListOffset {
			return 0, 0, errInvalidListOffset
		}
	}

	return limit, offset, nil
}
