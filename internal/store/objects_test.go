package store_test

import (
	"context"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	return s
}

func TestCreateObjectRoundTripsUnchanged(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()

	value := []byte("sealed-ciphertext")
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", value, []string{"homelab/vps-docker"}, ""))

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, value, obj.Value)
	require.Equal(t, []string{"homelab/vps-docker"}, obj.UsedBy)
}

func TestCreateObjectWithoutUsedBy(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateObject(ctx, "no_consumers_yet", []byte("v"), nil, ""))

	obj, err := s.GetObject(ctx, "no_consumers_yet")
	require.NoError(t, err)
	require.Empty(t, obj.UsedBy)
}

func TestCreateObjectRejectsDuplicateID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateObject(ctx, "dup", []byte("first"), nil, ""))

	err := s.CreateObject(ctx, "dup", []byte("second"), nil, "")
	require.ErrorIs(t, err, store.ErrAlreadyExists)

	// The original value must survive the rejected create.
	obj, err := s.GetObject(ctx, "dup")
	require.NoError(t, err)
	require.Equal(t, []byte("first"), obj.Value)
}

func TestCreateObjectWithDescriptionRoundTrips(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("v"), nil, "prod deploy webhook"))

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, "prod deploy webhook", obj.Description)
}

func TestCreateObjectWithoutDescription(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateObject(ctx, "no_description_yet", []byte("v"), nil, ""))

	obj, err := s.GetObject(ctx, "no_description_yet")
	require.NoError(t, err)
	require.Empty(t, obj.Description)
}

func TestGetObjectUnknownID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, err := s.GetObject(context.Background(), "nope")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestUpdateObjectReplacesValuePreservingUsedBy(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), []string{"homelab/vps-docker"}, ""))

	require.NoError(t, s.UpdateObject(ctx, "mattermost_deploy_webhook", []byte("new")))

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []byte("new"), obj.Value)
	require.Equal(t, []string{"homelab/vps-docker"}, obj.UsedBy)
}

func TestUpdateObjectPreservesDescription(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), nil, "prod deploy webhook"))

	require.NoError(t, s.UpdateObject(ctx, "mattermost_deploy_webhook", []byte("new")))

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, "prod deploy webhook", obj.Description)
}

func TestUpdateObjectUnknownID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.UpdateObject(context.Background(), "nope", []byte("v"))
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestDeleteObjectRemovesIt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("v"), nil, ""))

	require.NoError(t, s.DeleteObject(ctx, "mattermost_deploy_webhook"))

	_, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestDeleteObjectUnknownID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.DeleteObject(context.Background(), "nope")
	require.ErrorIs(t, err, store.ErrNotFound)
}
