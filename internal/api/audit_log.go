package api

import (
	"net/http"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
)

// AuditLogEntry is one entry in a queryAuditLog response. Matches
// components.schemas.AuditLogEntry in api/openapi.yaml.
type AuditLogEntry struct {
	ObjectID  string `json:"object_id"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
	Caller    string `json:"caller,omitempty"`
}

// handleQueryAuditLog returns audit log entries matching the given
// filters, oldest first. Unauthenticated, same as the other read paths -
// this is itself part of the audit trail's own value, not something it
// needs to protect.
func handleQueryAuditLog(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		filter := store.AuditLogFilter{
			ObjectID: q.Get("object_id"),
			Caller:   q.Get("caller"),
		}

		if from := q.Get("from"); from != "" {
			t, err := time.Parse(time.RFC3339, from)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "from must be RFC 3339")

				return
			}

			filter.From = t
		}

		if to := q.Get("to"); to != "" {
			t, err := time.Parse(time.RFC3339, to)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "to must be RFC 3339")

				return
			}

			filter.To = t
		}

		rows, err := s.QueryAuditLog(r.Context(), filter)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		entries := make([]AuditLogEntry, len(rows))
		for i, row := range rows {
			entries[i] = AuditLogEntry{
				ObjectID:  row.ObjectID,
				Action:    string(row.Action),
				Timestamp: row.Timestamp,
				Caller:    row.Caller,
			}
		}

		writeJSON(w, http.StatusOK, entries)
	}
}
