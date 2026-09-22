package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
)

// Sentinels for GET /audit-log's own query-parameter validation.
var (
	errFromMustBeRFC3339  = errors.New("from must be RFC 3339")
	errToMustBeRFC3339    = errors.New("to must be RFC 3339")
	errAfterMustBeInteger = errors.New("after must be an integer")
	errInvalidLimit       = errors.New("limit must be an integer between 1 and 500")
)

// AuditLogEntry is one entry in a queryAuditLog response. Matches
// components.schemas.AuditLogEntry in api/openapi.yaml.
type AuditLogEntry struct {
	ID        int64  `json:"id"`
	ObjectID  string `json:"object_id"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
	Caller    string `json:"caller,omitempty"`
	IP        string `json:"ip"`
	ActorType string `json:"actor_type,omitempty"`
	ActorID   string `json:"actor_id,omitempty"`
}

// defaultAuditLogLimit and maxAuditLogLimit match api/openapi.yaml's
// `limit` parameter - design.md's "Audit log UI" decision.
const (
	defaultAuditLogLimit = 50
	maxAuditLogLimit     = 500
)

// handleQueryAuditLog returns one page of audit log entries matching the
// given filters, oldest first. Unauthenticated, same as the other read
// paths - this is itself part of the audit trail's own value, not
// something it needs to protect.
func handleQueryAuditLog(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := auditLogFilterFrom(r)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())

			return
		}

		rows, err := s.QueryAuditLog(r.Context(), filter)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, auditLogEntriesFrom(rows))
	}
}

// auditLogFilterFrom parses GET /audit-log's query parameters into a
// store.AuditLogFilter, applying limit's documented default and cap.
func auditLogFilterFrom(r *http.Request) (store.AuditLogFilter, error) {
	q := r.URL.Query()

	filter := store.AuditLogFilter{
		ObjectID: q.Get("object_id"),
		Caller:   q.Get("caller"),
		Actor:    q.Get("actor"),
		Limit:    defaultAuditLogLimit,
	}

	if from := q.Get("from"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return store.AuditLogFilter{}, errFromMustBeRFC3339
		}

		filter.From = t
	}

	if to := q.Get("to"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return store.AuditLogFilter{}, errToMustBeRFC3339
		}

		filter.To = t
	}

	if after := q.Get("after"); after != "" {
		id, err := strconv.ParseInt(after, 10, 64)
		if err != nil {
			return store.AuditLogFilter{}, errAfterMustBeInteger
		}

		filter.After = id
	}

	if limit := q.Get("limit"); limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil || n < 1 || n > maxAuditLogLimit {
			return store.AuditLogFilter{}, errInvalidLimit
		}

		filter.Limit = n
	}

	return filter, nil
}

// AuditActorOption is one selectable actor value in a
// queryAuditLogFilterOptions response. Matches
// components.schemas.AuditActorOption in api/openapi.yaml.
type AuditActorOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// AuditLogFilterOptions is the queryAuditLogFilterOptions response shape.
// Matches components.schemas.AuditLogFilterOptions in api/openapi.yaml.
type AuditLogFilterOptions struct {
	ObjectIDs []string           `json:"object_ids"`
	Actors    []AuditActorOption `json:"actors"`
	Callers   []string           `json:"callers"`
}

// handleQueryAuditLogFilterOptions returns every distinct object id,
// actor, and caller currently recorded in the audit log - what backs the
// web UI's own filter select boxes with real values instead of a
// free-text guess. Unauthenticated, same as GET /audit-log itself.
func handleQueryAuditLogFilterOptions(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		options, err := s.QueryAuditLogFilterOptions(r.Context())
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, auditLogFilterOptionsFrom(options))
	}
}

func auditLogFilterOptionsFrom(options store.AuditLogFilterOptions) AuditLogFilterOptions {
	objectIDs := options.ObjectIDs
	if objectIDs == nil {
		objectIDs = []string{}
	}

	callers := options.Callers
	if callers == nil {
		callers = []string{}
	}

	actors := make([]AuditActorOption, len(options.Actors))
	for i, a := range options.Actors {
		actors[i] = AuditActorOption{Value: a.Value, Label: a.Label}
	}

	return AuditLogFilterOptions{ObjectIDs: objectIDs, Actors: actors, Callers: callers}
}

func auditLogEntriesFrom(rows []store.AuditLogEntry) []AuditLogEntry {
	entries := make([]AuditLogEntry, len(rows))
	for i, row := range rows {
		entries[i] = AuditLogEntry{
			ID:        row.ID,
			ObjectID:  row.ObjectID,
			Action:    string(row.Action),
			Timestamp: row.Timestamp,
			Caller:    row.Caller,
			IP:        row.IP,
			ActorType: row.ActorType,
			ActorID:   row.ActorID,
		}
	}

	return entries
}
