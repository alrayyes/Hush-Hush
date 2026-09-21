package api

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/alrayyes/hush-hush/internal/store"
)

// defaultConsumerPageSize and maxConsumerPageSize match
// api/openapi.yaml's `page_size` parameter.
const (
	defaultConsumerPageSize = 20
	maxConsumerPageSize     = 100
)

// Sentinels for GET /consumers's own query-parameter validation.
var (
	errInvalidConsumerPage     = errors.New("page must be a positive integer")
	errInvalidConsumerPageSize = errors.New("page_size must be an integer between 1 and 100")
)

// ConsumerEntry is one entry in a paginated listConsumers response.
// Matches components.schemas.ConsumerEntry in api/openapi.yaml.
type ConsumerEntry struct {
	Name        string `json:"name"`
	SecretCount int    `json:"secret_count"`
}

// ConsumersPage is listConsumers's paginated response shape, returned
// whenever the request carries q, page, or page_size. Matches
// components.schemas.ConsumersPage in api/openapi.yaml.
type ConsumersPage struct {
	Consumers []ConsumerEntry `json:"consumers"`
	Total     int             `json:"total"`
}

// handleListConsumers returns every distinct consumer name currently
// present in any object's used_by list. Gated the same way GET /objects
// is: a write token or a session, since listing needs no id the caller
// already holds.
//
// Called with none of q, page, or page_size, it returns that original
// plain, unpaginated array of names, sorted, with no duplicates -
// consumer-combobox's own call site (alrayyes/hush-hush#251) depends on
// this shape staying unchanged. Given any of the three, it instead
// returns a ConsumersPage: one page of consumers whose name contains q
// (case-insensitive) when given, each with a count of the secret objects
// that reference it, plus the total matching count for page-number
// navigation (alrayyes/hush-hush#252).
func handleListConsumers(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if q.Get("q") == "" && q.Get("page") == "" && q.Get("page_size") == "" {
			consumers, err := s.ListConsumers(r.Context())
			if err != nil {
				writeInternalError(w, r, err)

				return
			}

			if consumers == nil {
				consumers = []string{}
			}

			writeJSON(w, http.StatusOK, consumers)

			return
		}

		filter, err := consumerFilterFrom(q)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())

			return
		}

		page, err := s.ListConsumersPage(r.Context(), filter)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, consumersPageFrom(page))
	}
}

// consumerFilterFrom parses GET /consumers's query parameters into a
// store.ConsumerFilter, applying page and page_size's documented
// defaults and page_size's cap.
func consumerFilterFrom(q url.Values) (store.ConsumerFilter, error) {
	filter := store.ConsumerFilter{
		Name:     q.Get("q"),
		Page:     1,
		PageSize: defaultConsumerPageSize,
	}

	if page := q.Get("page"); page != "" {
		n, err := strconv.Atoi(page)
		if err != nil || n < 1 {
			return store.ConsumerFilter{}, errInvalidConsumerPage
		}

		filter.Page = n
	}

	if pageSize := q.Get("page_size"); pageSize != "" {
		n, err := strconv.Atoi(pageSize)
		if err != nil || n < 1 || n > maxConsumerPageSize {
			return store.ConsumerFilter{}, errInvalidConsumerPageSize
		}

		filter.PageSize = n
	}

	return filter, nil
}

func consumersPageFrom(page store.ConsumerPage) ConsumersPage {
	entries := make([]ConsumerEntry, len(page.Consumers))
	for i, c := range page.Consumers {
		entries[i] = ConsumerEntry{Name: c.Name, SecretCount: c.SecretCount}
	}

	return ConsumersPage{Consumers: entries, Total: page.Total}
}
