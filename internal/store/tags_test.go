package store_test

import (
	"context"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestCreateObjectWithTagsRoundTripsThemSorted(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), nil, "", "", store.WithTags([]string{"prod", "homelab"})))

	obj, err := s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab", "prod"}, obj.Tags)
}

func TestObjectWithoutTagsHasNone(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), nil, "", ""))

	obj, err := s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Empty(t, obj.Tags)
}

func TestUpdateObjectPreservesTagsWhenNotGiven(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), nil, "", "", store.WithTags([]string{"prod"})))
	require.NoError(t, s.UpdateObject(ctx, "a", []byte("w"), nil))

	obj, err := s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, []string{"prod"}, obj.Tags)
}

func TestUpdateObjectReplacesTagsWhenGivenAndClearsOnEmpty(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), nil, "", "", store.WithTags([]string{"prod"})))

	require.NoError(t, s.UpdateObject(ctx, "a", []byte("w"), nil, store.WithTags([]string{"homelab"})))

	obj, err := s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab"}, obj.Tags)

	require.NoError(t, s.UpdateObject(ctx, "a", []byte("x"), nil, store.WithTags([]string{})))

	obj, err = s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Empty(t, obj.Tags)
}

func TestListObjectsReturnsTagsAndFiltersByThemRequiringAll(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "both", []byte("v"), nil, "", "", store.WithTags([]string{"prod", "homelab"})))
	require.NoError(t, s.CreateObject(ctx, "prod_only", []byte("v"), nil, "", "", store.WithTags([]string{"prod"})))
	require.NoError(t, s.CreateObject(ctx, "untagged", []byte("v"), nil, "", ""))

	prod, err := s.ListObjects(ctx, store.ObjectFilter{Tags: []string{"prod"}})
	require.NoError(t, err)
	require.Len(t, prod, 2)
	require.Equal(t, []string{"homelab", "prod"}, prod[0].Tags)

	both, err := s.ListObjects(ctx, store.ObjectFilter{Tags: []string{"prod", "homelab"}})
	require.NoError(t, err)
	require.Len(t, both, 1)
	require.Equal(t, "both", both[0].Slug)
}

func TestDeletingAnObjectRemovesItsTags(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), nil, "", "", store.WithTags([]string{"prod"})))
	require.NoError(t, s.DeleteObject(ctx, "a"))
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), nil, "", ""))

	obj, err := s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Empty(t, obj.Tags)
}
