package api

import (
	"errors"
	"math"
	"net/http"
	"strconv"
)

// Paging for the list endpoints that used to return every row
// (alrayyes/hush-hush#649, #662). A list is one page: limit (1 to 500, 50 when
// left out) and offset pick which, and X-Total-Count says how many rows there
// are in all, so a client reads the rest by asking for the next offset. The
// array stays the response shape on purpose, so the generated SDKs' types
// don't change.
const (
	// DefaultListPageSize is the page size when a request gives no limit.
	DefaultListPageSize = 50
	// MaxListPageSize is the largest limit a request may ask for.
	MaxListPageSize = 500

	maxListOffset = math.MaxInt32
)

var (
	errInvalidListLimit  = errors.New("limit must be an integer from 1 to 500")
	errInvalidListOffset = errors.New("offset must be a non-negative integer")
)

// writePage answers with the page of items the request asked for, the first
// DefaultListPageSize when it asked for none. It has already answered a
// request whose limit or offset is out of range.
func writePage[T any](w http.ResponseWriter, r *http.Request, items []T) {
	q := r.URL.Query()
	total := len(items)

	limit, offset, err := pageBounds(q.Get("limit"), q.Get("offset"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())

		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, items[min(offset, total):min(offset+limit, total)])
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
