package api

import (
	"errors"
	"math"
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
// Matches components.schemas.ConsumerEntry in api/openapi.yaml. PublicKey
// is omitted from the JSON body entirely when the consumer has no
// registered key (specs/consumers/spec.md's "Consumer with no registered
// public key" scenario: no field, not an error or a null).
type ConsumerEntry struct {
	Name        string `json:"name"`
	SecretCount int    `json:"secret_count"`
	PublicKey   string `json:"public_key,omitempty"`
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
		if err != nil || n < 1 || n > math.MaxInt32 {
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
		entries[i] = ConsumerEntry{Name: c.Name, SecretCount: c.SecretCount, PublicKey: c.PublicKey}
	}

	return ConsumersPage{Consumers: entries, Total: page.Total}
}

// AddConsumerRequest is the POST /consumers body. Matches
// components.schemas.AddConsumerRequest in api/openapi.yaml.
type AddConsumerRequest struct {
	Name string `json:"name" maxLength:"128"`
}

// handleAddConsumer adds a consumer to the directory with no secret
// referencing it yet (alrayyes/hush-hush#324) - every other consumer
// here only exists because some object's used_by recorded it.
func handleAddConsumer(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AddConsumerRequest
		if !decodeRequest(w, r, &req) {
			return
		}

		if req.Name == "" {
			writeError(w, r, http.StatusBadRequest, "name is required")

			return
		}

		switch err := s.AddConsumer(r.Context(), req.Name); {
		case err == nil:
		case errors.Is(err, store.ErrConsumerAlreadyExists):
			writeError(w, r, http.StatusConflict, "consumer already exists")

			return
		default:
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusCreated, ConsumerEntry{Name: req.Name, SecretCount: 0})
	}
}

// UpdateConsumerRequest is the PATCH /consumers/{name} body. Matches
// components.schemas.UpdateConsumerRequest in api/openapi.yaml. At least
// one of Name or PublicKey must be set; a field left empty (the zero
// value, indistinguishable here from "omitted") leaves that aspect of the
// consumer unchanged - there's no way to clear a registered public key
// through this endpoint, only to set or replace one.
type UpdateConsumerRequest struct {
	Name      string `json:"name,omitempty" maxLength:"128"`
	PublicKey string `json:"public_key,omitempty" maxLength:"512"`
}

// handleUpdateConsumer renames a consumer, registers or updates its
// public key, or both in the same call. Consumers aren't a stored
// resource of their own (alrayyes/hush-hush#282, ADR 0002), so a rename
// is a bulk used_by rewrite, not CRUD on a dedicated resource; a rename
// target that collides with an existing consumer name merges the two
// rather than erroring, carrying over whichever side's public key
// survives the merge (store.RenameConsumer's own doc comment). Setting a
// public key upserts a consumers row via store.SetConsumerPublicKey even
// for a name that has never been added directly or referenced by any
// object's used_by list yet - specs/consumers/spec.md's "Consumer public
// key registration" requirement doesn't restrict registration to an
// already-known consumer, and a name that only exists via used_by has
// nowhere else to persist a key until one is registered.
func handleUpdateConsumer(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req UpdateConsumerRequest
		if !decodeRequest(w, r, &req) {
			return
		}

		if req.Name == "" && req.PublicKey == "" {
			writeError(w, r, http.StatusBadRequest, "name or public_key is required")

			return
		}

		name := r.PathValue("name")
		var entry store.ConsumerEntry

		if req.Name != "" {
			renamed, err := s.RenameConsumer(r.Context(), name, req.Name)
			switch {
			case err == nil:
			case errors.Is(err, store.ErrUnknownConsumer):
				writeError(w, r, http.StatusNotFound, "unknown consumer")

				return
			default:
				writeInternalError(w, r, err)

				return
			}

			entry = renamed
			name = req.Name
		}

		if req.PublicKey != "" {
			keyed, err := s.SetConsumerPublicKey(r.Context(), name, req.PublicKey)
			if err != nil {
				writeInternalError(w, r, err)

				return
			}

			entry = keyed
		}

		writeJSON(w, http.StatusOK, ConsumerEntry{Name: entry.Name, SecretCount: entry.SecretCount, PublicKey: entry.PublicKey})
	}
}

// handleDeleteConsumer strips name from the used_by list of every stored
// object that currently records it. The objects themselves aren't
// deleted, even if this empties their used_by list.
func handleDeleteConsumer(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := s.DeleteConsumer(r.Context(), r.PathValue("name"))
		switch {
		case err == nil:
		case errors.Is(err, store.ErrUnknownConsumer):
			writeError(w, r, http.StatusNotFound, "unknown consumer")

			return
		default:
			writeInternalError(w, r, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
