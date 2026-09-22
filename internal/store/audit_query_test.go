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
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionRead, "", "203.0.113.2", "", ""))

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
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "", "203.0.113.2", "", ""))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{ObjectID: "a"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
}

func TestQueryAuditLogFiltersByCaller(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "homelab/vps-docker", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "homelab/other", "203.0.113.2", "", ""))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{Caller: "homelab/vps-docker"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
}

func TestQueryAuditLogFiltersByTimeRange(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "", ""))

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
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "homelab/vps-docker", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionRead, "homelab/other", "203.0.113.2", "", ""))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{ObjectID: "a", Caller: "homelab/vps-docker"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, store.AuditActionCreate, entries[0].Action)
}

func TestQueryAuditLogFiltersByActor(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "token", "a1b2c3d4e5f6a7b8"))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "", "203.0.113.2", "session", "admin"))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{Actor: "a1b2c3d4e5f6a7b8"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
	require.Equal(t, "token", entries[0].ActorType)
}

func TestQueryAuditLogReturnsIDsAssignedInOrder(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "", "203.0.113.2", "", ""))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.NotZero(t, entries[0].ID)
	require.Greater(t, entries[1].ID, entries[0].ID)
}

func TestQueryAuditLogAfterCursorReturnsOnlyLaterEntries(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "", "203.0.113.2", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "c", store.AuditActionCreate, "", "203.0.113.3", "", ""))

	first, err := s.QueryAuditLog(ctx, store.AuditLogFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, "a", first[0].ObjectID)

	rest, err := s.QueryAuditLog(ctx, store.AuditLogFilter{After: first[0].ID})
	require.NoError(t, err)
	require.Len(t, rest, 2)
	require.Equal(t, "b", rest[0].ObjectID)
	require.Equal(t, "c", rest[1].ObjectID)
}

func TestQueryAuditLogLimitCapsResultCount(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	for range 5 {
		require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "", ""))
	}

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, entries, 2)
}

func TestQueryAuditLogReturnsIP(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionCreate, "", "203.0.113.1", "", ""))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "203.0.113.1", entries[0].IP)
}

func TestQueryAuditLogFiltersByActorNoneMatchesUnauthenticatedReads(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionRead, "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "", "203.0.113.2", "session", "admin"))

	entries, err := s.QueryAuditLog(ctx, store.AuditLogFilter{Actor: "none"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "a", entries[0].ObjectID)
	require.Empty(t, entries[0].ActorType)
}

func TestQueryAuditLogFilterOptionsReturnsDistinctValuesThatActuallyAppear(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionRead, "", "203.0.113.1", "", ""))
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionUpdate, "homelab/vps-docker", "203.0.113.2", "session", "admin"))
	require.NoError(t, s.RecordAuditLog(ctx, "b", store.AuditActionCreate, "homelab/vps-docker", "203.0.113.3", "token", "a1b2c3d4e5f6a7b8"))

	options, err := s.QueryAuditLogFilterOptions(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, options.ObjectIDs)
	require.Equal(t, []string{"homelab/vps-docker"}, options.Callers)
	require.ElementsMatch(t, []store.AuditActorOption{
		{Value: "none", Label: "none"},
		{Value: "admin", Label: "admin"},
		{Value: "a1b2c3d4e5f6a7b8", Label: "token:a1b2c3d4e5f6a7b8"},
	}, options.Actors)
}

func TestQueryAuditLogFilterOptionsOnAFreshStoreIsEmpty(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	options, err := s.QueryAuditLogFilterOptions(context.Background())
	require.NoError(t, err)
	require.Empty(t, options.ObjectIDs)
	require.Empty(t, options.Callers)
	require.Empty(t, options.Actors)
}
