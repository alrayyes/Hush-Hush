package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// auditLogEntry is a raw read of one audit_log row, used only to verify a
// handler recorded what it should have - the query endpoint (alrayyes/
// hush-hush#59) is a separate, not-yet-implemented path.
type auditLogEntry struct {
	ObjectID string
	Action   string
	Caller   *string
	IP       string
}

func auditLogEntries(t *testing.T, s *store.Store) []auditLogEntry {
	t.Helper()

	rows, err := s.DB().Query(`SELECT object_id, action, caller, ip FROM audit_log ORDER BY id`)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rows.Close()) })

	var got []auditLogEntry
	for rows.Next() {
		var e auditLogEntry
		require.NoError(t, rows.Scan(&e.ObjectID, &e.Action, &e.Caller, &e.IP))
		got = append(got, e)
	}
	require.NoError(t, rows.Err())

	return got
}

func TestCreateObjectRecordsAnAuditLogEntry(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "x", Value: []byte("v")}, testWriterToken)
	req.Header.Set("X-Caller", "homelab/vps-docker")
	mux.ServeHTTP(httptest.NewRecorder(), req)

	entries := auditLogEntries(t, s)
	require.Len(t, entries, 1)
	require.Equal(t, "x", entries[0].ObjectID)
	require.Equal(t, "create", entries[0].Action)
	require.Equal(t, "homelab/vps-docker", *entries[0].Caller)
}

func TestCreateObjectRecordsTheRequestsSourceIP(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "x", Value: []byte("v")}, testWriterToken)
	req.RemoteAddr = "203.0.113.1:54321"
	mux.ServeHTTP(httptest.NewRecorder(), req)

	entries := auditLogEntries(t, s)
	require.Len(t, entries, 1)
	require.Equal(t, "203.0.113.1", entries[0].IP)
}

func TestCreateObjectRecordsTheRawRemoteAddrWhenItHasNoPort(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)

	req := createRequest(t, hushhush.CreateObjectRequest{ID: "x", Value: []byte("v")}, testWriterToken)
	req.RemoteAddr = "not-a-host-port-pair"
	mux.ServeHTTP(httptest.NewRecorder(), req)

	entries := auditLogEntries(t, s)
	require.Len(t, entries, 1)
	require.Equal(t, "not-a-host-port-pair", entries[0].IP)
}

func TestGetObjectRecordsAnAuditLogEntryWithNoCaller(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil))

	req := httptest.NewRequest(http.MethodGet, "/objects/x", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	entries := auditLogEntries(t, s)
	require.Len(t, entries, 1)
	require.Equal(t, "read", entries[0].Action)
	require.Nil(t, entries[0].Caller)
}

func TestUpdateObjectRecordsAnAuditLogEntry(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil))

	mux.ServeHTTP(httptest.NewRecorder(), updateRequest(t, "x", []byte("new"), testWriterToken))

	entries := auditLogEntries(t, s)
	require.Len(t, entries, 1)
	require.Equal(t, "update", entries[0].Action)
}

func TestDeleteObjectRecordsAnAuditLogEntry(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	require.NoError(t, s.CreateObject(context.Background(), "x", []byte("v"), nil))

	mux.ServeHTTP(httptest.NewRecorder(), deleteRequest(t, "x", testWriterToken))

	entries := auditLogEntries(t, s)
	require.Len(t, entries, 1)
	require.Equal(t, "delete", entries[0].Action)
}
