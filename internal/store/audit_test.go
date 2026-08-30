package store_test

import (
	"context"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// auditLogRow is a raw read of one audit_log row, used only to verify
// RecordAuditLog wrote what it was asked to - QueryAuditLog (alrayyes/
// hush-hush#59) is a separate, not-yet-implemented query path.
type auditLogRow struct {
	ObjectID  string
	Action    string
	Caller    *string
	IP        string
	Timestamp string
}

func auditLogRows(t *testing.T, s *store.Store) []auditLogRow {
	t.Helper()

	rows, err := s.DB().Query(`SELECT object_id, action, caller, ip, timestamp FROM audit_log ORDER BY id`)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rows.Close()) })

	var got []auditLogRow
	for rows.Next() {
		var r auditLogRow
		require.NoError(t, rows.Scan(&r.ObjectID, &r.Action, &r.Caller, &r.IP, &r.Timestamp))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())

	return got
}

func TestRecordAuditLogInsertsAnEntry(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	require.NoError(t, s.RecordAuditLog(context.Background(), "mattermost_deploy_webhook", store.AuditActionCreate, "homelab/vps-docker", "203.0.113.1"))

	rows := auditLogRows(t, s)
	require.Len(t, rows, 1)
	require.Equal(t, "mattermost_deploy_webhook", rows[0].ObjectID)
	require.Equal(t, "create", rows[0].Action)
	require.Equal(t, "homelab/vps-docker", *rows[0].Caller)
	require.Equal(t, "203.0.113.1", rows[0].IP)
	require.NotEmpty(t, rows[0].Timestamp)
}

func TestRecordAuditLogWithNoCallerLeavesCallerNull(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	require.NoError(t, s.RecordAuditLog(context.Background(), "x", store.AuditActionRead, "", "203.0.113.1"))

	rows := auditLogRows(t, s)
	require.Len(t, rows, 1)
	require.Nil(t, rows[0].Caller)
}
