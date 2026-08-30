package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestQueryAuditLogReturnsAllEntriesOldestFirst(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1"))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionRead, "", "203.0.113.2"))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "a", entries[0].ObjectID)
	require.Equal(t, "b", entries[1].ObjectID)
}

func TestQueryAuditLogFiltersByObjectID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1"))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "", "203.0.113.2"))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{ObjectID: "a"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
}

func TestQueryAuditLogFiltersByCaller(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "homelab/vps-docker", "203.0.113.1"))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "homelab/other", "203.0.113.2"))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{Caller: "homelab/vps-docker"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
}

func TestQueryAuditLogFiltersByTimeRange(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1"))

	future := time.Now().UTC().Add(time.Hour)
	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{From: future})
	require.NoError(t, err)
	require.Empty(t, entries)

	past := time.Now().UTC().Add(-time.Hour)
	entries, err = s.QueryAuditLog(ctx, store.AuditLogFilter{From: past})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestQueryAuditLogCombinesFiltersWithAnd(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "homelab/vps-docker", "203.0.113.1"))
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionRead, "homelab/other", "203.0.113.2"))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{ObjectID: "a", Caller: "homelab/vps-docker"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, store.AuditActionCreate, entries[0].Action)
}

func TestQueryAuditLogReturnsIP(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1"))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "203.0.113.1", entries[0].IP)
}
