package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

func TestQueryAuditLogReturnsAllEntriesOldestFirst(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", "create", "", "203.0.113.1"))
	require.NoError(t, s.RecordAuditLog(ctx, "b", "read", "", "203.0.113.2"))

	req := httptest.NewRequest(http.MethodGet, "/audit-log", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var entries []hushhush.AuditLogEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &entries))
	require.Len(t, entries, 2)
	require.Equal(t, "a", entries[0].ObjectID)
	require.Equal(t, "b", entries[1].ObjectID)
}

func TestQueryAuditLogFiltersByObjectID(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", "create", "", "203.0.113.1"))
	require.NoError(t, s.RecordAuditLog(ctx, "b", "create", "", "203.0.113.2"))

	req := httptest.NewRequest(http.MethodGet, "/audit-log?object_id=a", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var entries []hushhush.AuditLogEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
}

func TestQueryAuditLogMalformedFromIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/audit-log?from=not-a-time", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
