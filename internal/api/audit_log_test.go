package api_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	require.NoError(t, s.RecordAuditLog(ctx, "a", "create", "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", "read", "", "203.0.113.2", "", ""))

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
	require.NoError(t, s.RecordAuditLog(ctx, "a", "create", "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", "create", "", "203.0.113.2", "", ""))

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

func TestQueryAuditLogLimitCapsResultCount(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", "create", "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", "create", "", "203.0.113.2", "", ""))

	req := httptest.NewRequest(http.MethodGet, "/audit-log?limit=1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var entries []hushhush.AuditLogEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
}

func TestQueryAuditLogAfterAdvancesThePage(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", "create", "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", "create", "", "203.0.113.2", "", ""))

	first := httptest.NewRequest(http.MethodGet, "/audit-log?limit=1", nil)
	firstRec := httptest.NewRecorder()
	mux.ServeHTTP(firstRec, first)

	var firstPage []hushhush.AuditLogEntry
	require.NoError(t, json.Unmarshal(firstRec.Body.Bytes(), &firstPage))
	require.Len(t, firstPage, 1)

	second := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/audit-log?after=%d", firstPage[0].ID), nil)
	secondRec := httptest.NewRecorder()
	mux.ServeHTTP(secondRec, second)

	var secondPage []hushhush.AuditLogEntry
	require.NoError(t, json.Unmarshal(secondRec.Body.Bytes(), &secondPage))
	require.Len(t, secondPage, 1)
	require.Equal(t, "b", secondPage[0].ObjectID)
}

func TestQueryAuditLogMalformedAfterIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/audit-log?after=not-a-number", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestQueryAuditLogLimitOutOfRangeIsRejected(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/audit-log?limit=501", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestQueryAuditLogFilterOptionsReturnsDistinctValues(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", "read", "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "a", "update", "homelab/vps-docker", "203.0.113.2", "session", "admin"))

	req := httptest.NewRequest(http.MethodGet, "/audit-log/filter-options", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body hushhush.AuditLogFilterOptions
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, []string{"a"}, body.ObjectIDs)
	require.Equal(t, []string{"homelab/vps-docker"}, body.Callers)
	require.ElementsMatch(t, []hushhush.AuditActorOption{
		{Value: "none", Label: "none"},
		{Value: "admin", Label: "admin"},
	}, body.Actors)
}

func TestQueryAuditLogFilterOptionsOnAFreshLogReturnsEmptyArrays(t *testing.T) {
	t.Parallel()

	mux, _ := newTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/audit-log/filter-options", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"object_ids":[],"actors":[],"callers":[]}`, rec.Body.String())
}

func TestQueryAuditLogWithActorNoneMatchesUnauthenticatedReads(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", "read", "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", "create", "", "203.0.113.2", "session", "admin"))

	req := httptest.NewRequest(http.MethodGet, "/audit-log?actor=none", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var entries []hushhush.AuditLogEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
}
