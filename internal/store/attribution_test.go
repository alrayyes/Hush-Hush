package store_test

import (
	"context"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func recordCreate(t *testing.T, s *store.Store, slug, actorType, actorID string) {
	t.Helper()

	require.NoError(t, s.CreateObject(context.Background(), slug, []byte("v"), nil, "", ""))
	require.NoError(t, s.RecordAuditLog(context.Background(), slug, store.AuditActionCreate, "", "203.0.113.1", actorType, actorID))
}

func listed(t *testing.T, s *store.Store, slug string) store.Object {
	t.Helper()

	objs, err := s.ListObjects(context.Background(), store.ObjectFilter{})
	require.NoError(t, err)

	for _, o := range objs {
		if o.Slug == slug {
			return o
		}
	}

	require.FailNow(t, "object not listed", slug)

	return store.Object{}
}

func TestListObjectsReportsWhoCreatedAndLastUpdatedAnObject(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	recordCreate(t, s, "a", "session", "user-1")
	require.NoError(t, s.UpdateObject(ctx, "a", []byte("w"), nil))
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionUpdate, "", "203.0.113.2", "token", "tok-1"))

	obj := listed(t, s, "a")

	require.Equal(t, store.Actor{Type: "session", ID: "user-1"}, obj.CreatedBy)
	require.Equal(t, store.Actor{Type: "token", ID: "tok-1"}, obj.UpdatedBy)
	require.False(t, obj.CreatedAt.IsZero())
	require.False(t, obj.UpdatedAt.IsZero())
}

func TestUpdatedByEqualsCreatedByOnAnObjectNeverUpdated(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	recordCreate(t, s, "a", "session", "user-1")

	obj := listed(t, s, "a")

	require.Equal(t, obj.CreatedBy, obj.UpdatedBy)
	require.Equal(t, obj.CreatedAt, obj.UpdatedAt)
}

func TestAttributionIsRightPastTheFirstFiftyAuditEvents(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	recordCreate(t, s, "old", "session", "user-1")

	for range 60 {
		require.NoError(t, s.RecordAuditLog(ctx, "old", store.AuditActionRead, "", "203.0.113.3", "", ""))
	}

	recordCreate(t, s, "new", "token", "tok-9")
	require.NoError(t, s.UpdateObject(ctx, "new", []byte("w"), nil))
	require.NoError(t, s.RecordAuditLog(ctx, "new", store.AuditActionUpdate, "", "203.0.113.4", "session", "user-2"))

	obj := listed(t, s, "new")

	require.Equal(t, store.Actor{Type: "token", ID: "tok-9"}, obj.CreatedBy)
	require.Equal(t, store.Actor{Type: "session", ID: "user-2"}, obj.UpdatedBy)
}

func TestAttributionIgnoresAnEarlierObjectThatHeldTheSameSlug(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	recordCreate(t, s, "a", "session", "user-1")
	require.NoError(t, s.RecordAuditLog(ctx, "a", store.AuditActionUpdate, "", "203.0.113.2", "session", "user-1"))
	require.NoError(t, s.DeleteObject(ctx, "a"))
	recordCreate(t, s, "a", "token", "tok-2")

	obj := listed(t, s, "a")

	require.Equal(t, store.Actor{Type: "token", ID: "tok-2"}, obj.CreatedBy)
	require.Equal(t, store.Actor{Type: "token", ID: "tok-2"}, obj.UpdatedBy)
}

func TestAnObjectWithNoAuditEntriesHasTimestampsButNoActors(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	require.NoError(t, s.CreateObject(context.Background(), "a", []byte("v"), nil, "", ""))

	obj := listed(t, s, "a")

	require.False(t, obj.CreatedAt.IsZero())
	require.Equal(t, store.Actor{}, obj.CreatedBy)
	require.Equal(t, store.Actor{}, obj.UpdatedBy)
}
