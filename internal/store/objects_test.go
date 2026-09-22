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

func TestUpdateObjectPreservesUsedByWhenNotSpecified(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), []string{"homelab/vps-docker"}, ""))

	require.NoError(t, s.UpdateObject(ctx, "mattermost_deploy_webhook", []byte("new"), nil))

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []byte("new"), obj.Value)
	require.Equal(t, []string{"homelab/vps-docker"}, obj.UsedBy)
}

func TestUpdateObjectReplacesUsedByWhenSpecified(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), []string{"homelab/vps-docker"}, ""))

	usedBy := []string{"ci", "homelab/nas"}
	require.NoError(t, s.UpdateObject(ctx, "mattermost_deploy_webhook", []byte("new"), &usedBy))

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, []string{"ci", "homelab/nas"}, obj.UsedBy)
}

func TestUpdateObjectClearsUsedByWithEmptySlice(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), []string{"homelab/vps-docker"}, ""))

	empty := []string{}
	require.NoError(t, s.UpdateObject(ctx, "mattermost_deploy_webhook", []byte("new"), &empty))

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Empty(t, obj.UsedBy)
}

func TestUpdateObjectPreservesDescription(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "mattermost_deploy_webhook", []byte("old"), nil, "prod deploy webhook"))

	require.NoError(t, s.UpdateObject(ctx, "mattermost_deploy_webhook", []byte("new"), nil))

	obj, err := s.GetObject(ctx, "mattermost_deploy_webhook")
	require.NoError(t, err)
	require.Equal(t, "prod deploy webhook", obj.Description)
}

func TestUpdateObjectUnknownID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.UpdateObject(context.Background(), "nope", []byte("v"), nil)
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

func TestListObjectsReturnsEveryObjectSortedByID(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "zeta", []byte("v"), nil, ""))
	require.NoError(t, s.CreateObject(ctx, "alpha", []byte("v"), []string{"homelab/vps-docker"}, "prod deploy webhook"))

	objs, err := s.ListObjects(ctx, store.ObjectFilter{})
	require.NoError(t, err)
	require.Len(t, objs, 2)
	require.Equal(t, "alpha", objs[0].ID)
	require.Equal(t, []string{"homelab/vps-docker"}, objs[0].UsedBy)
	require.Equal(t, "prod deploy webhook", objs[0].Description)
	require.Equal(t, "zeta", objs[1].ID)
}

func TestListObjectsReturnsEmptyArrayWhenNoneExist(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	objs, err := s.ListObjects(context.Background(), store.ObjectFilter{})
	require.NoError(t, err)
	require.Empty(t, objs)
}

func TestListObjectsFiltersByUsedBy(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "shared_by_two", []byte("v"), []string{"homelab/vps-docker", "homelab/mattermost"}, ""))
	require.NoError(t, s.CreateObject(ctx, "unrelated", []byte("v"), []string{"homelab/mattermost"}, ""))

	objs, err := s.ListObjects(ctx, store.ObjectFilter{UsedBy: "homelab/vps-docker"})
	require.NoError(t, err)
	require.Len(t, objs, 1)
	require.Equal(t, "shared_by_two", objs[0].ID)
}

func TestListConsumersOnFreshStoreIsEmpty(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	consumers, err := s.ListConsumers(context.Background())
	require.NoError(t, err)
	require.Empty(t, consumers)
}

func TestListConsumersReturnsEachDistinctNameOnce(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"homelab/vps-docker", "homelab/mattermost"}, ""))
	require.NoError(t, s.CreateObject(ctx, "b", []byte("v"), []string{"homelab/mattermost"}, ""))

	consumers, err := s.ListConsumers(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/mattermost", "homelab/vps-docker"}, consumers)
}

// seedConsumerFixture stores four objects with overlapping used_by lists -
// homelab/mattermost referenced by three, homelab/vps-docker and
// homelab/nas by one each, work/ci-runner by one - enough overlap to tell
// a per-consumer count apart from an object count.
func seedConsumerFixture(t *testing.T, s *store.Store) {
	t.Helper()

	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"homelab/mattermost", "homelab/vps-docker"}, ""))
	require.NoError(t, s.CreateObject(ctx, "b", []byte("v"), []string{"homelab/mattermost"}, ""))
	require.NoError(t, s.CreateObject(ctx, "c", []byte("v"), []string{"homelab/mattermost", "homelab/nas"}, ""))
	require.NoError(t, s.CreateObject(ctx, "d", []byte("v"), []string{"work/ci-runner"}, ""))
}

func TestListConsumersPageFiltersByNameSubstringCaseInsensitive(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedConsumerFixture(t, s)

	page, err := s.ListConsumersPage(context.Background(), store.ConsumerFilter{Name: "HOMELAB", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, 3, page.Total)
	require.Equal(t, []store.ConsumerEntry{
		{Name: "homelab/mattermost", SecretCount: 3},
		{Name: "homelab/nas", SecretCount: 1},
		{Name: "homelab/vps-docker", SecretCount: 1},
	}, page.Consumers)
}

func TestListConsumersPageReturnsOnePageAndTheTotalCount(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedConsumerFixture(t, s)

	firstPage, err := s.ListConsumersPage(context.Background(), store.ConsumerFilter{Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, 4, firstPage.Total)
	require.Equal(t, []store.ConsumerEntry{
		{Name: "homelab/mattermost", SecretCount: 3},
		{Name: "homelab/nas", SecretCount: 1},
	}, firstPage.Consumers)

	secondPage, err := s.ListConsumersPage(context.Background(), store.ConsumerFilter{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, 4, secondPage.Total)
	require.Equal(t, []store.ConsumerEntry{
		{Name: "homelab/vps-docker", SecretCount: 1},
		{Name: "work/ci-runner", SecretCount: 1},
	}, secondPage.Consumers)
}

func TestListConsumersPagePastTheLastPageIsEmpty(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	seedConsumerFixture(t, s)

	page, err := s.ListConsumersPage(context.Background(), store.ConsumerFilter{Page: 3, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, 4, page.Total)
	require.Empty(t, page.Consumers)
}

func TestListConsumersPageOnAFreshStoreIsEmpty(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	page, err := s.ListConsumersPage(context.Background(), store.ConsumerFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, 0, page.Total)
	require.Empty(t, page.Consumers)
}

func TestListConsumersPageEscapesLikeWildcardsInTheFilter(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"homelab_prod"}, ""))
	require.NoError(t, s.CreateObject(ctx, "b", []byte("v"), []string{"homelabXprod"}, ""))

	page, err := s.ListConsumersPage(ctx, store.ConsumerFilter{Name: "homelab_prod", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, []store.ConsumerEntry{{Name: "homelab_prod", SecretCount: 1}}, page.Consumers)
}

func TestRenameConsumerUpdatesEveryObject(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"homelab/vps-docker"}, ""))
	require.NoError(t, s.CreateObject(ctx, "b", []byte("v"), []string{"homelab/vps-docker"}, ""))
	require.NoError(t, s.CreateObject(ctx, "c", []byte("v"), []string{"other"}, ""))

	entry, err := s.RenameConsumer(ctx, "homelab/vps-docker", "homelab/vps-docker-2")
	require.NoError(t, err)
	require.Equal(t, store.ConsumerEntry{Name: "homelab/vps-docker-2", SecretCount: 2}, entry)

	a, err := s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/vps-docker-2"}, a.UsedBy)

	b, err := s.GetObject(ctx, "b")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/vps-docker-2"}, b.UsedBy)

	c, err := s.GetObject(ctx, "c")
	require.NoError(t, err)
	require.Equal(t, []string{"other"}, c.UsedBy)

	consumers, err := s.ListConsumers(ctx)
	require.NoError(t, err)
	require.NotContains(t, consumers, "homelab/vps-docker")
}

func TestRenameConsumerMergesIntoAnExistingTarget(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	// "b" already records both names - the rename must merge to one
	// entry rather than violating used_by's (object_id, consumer)
	// primary key.
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"old"}, ""))
	require.NoError(t, s.CreateObject(ctx, "b", []byte("v"), []string{"old", "new"}, ""))
	require.NoError(t, s.CreateObject(ctx, "c", []byte("v"), []string{"new"}, ""))

	entry, err := s.RenameConsumer(ctx, "old", "new")
	require.NoError(t, err)
	require.Equal(t, store.ConsumerEntry{Name: "new", SecretCount: 3}, entry)

	b, err := s.GetObject(ctx, "b")
	require.NoError(t, err)
	require.Equal(t, []string{"new"}, b.UsedBy)
}

func TestRenameConsumerToItselfIsANoOp(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"homelab/vps-docker"}, ""))

	entry, err := s.RenameConsumer(ctx, "homelab/vps-docker", "homelab/vps-docker")
	require.NoError(t, err)
	require.Equal(t, store.ConsumerEntry{Name: "homelab/vps-docker", SecretCount: 1}, entry)

	a, err := s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/vps-docker"}, a.UsedBy)
}

func TestRenameConsumerUnknownReturnsError(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, err := s.RenameConsumer(context.Background(), "nonexistent", "new")
	require.ErrorIs(t, err, store.ErrUnknownConsumer)
}

func TestDeleteConsumerRemovesFromEveryObject(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"homelab/vps-docker", "other"}, ""))
	require.NoError(t, s.CreateObject(ctx, "b", []byte("v"), []string{"homelab/vps-docker"}, ""))

	require.NoError(t, s.DeleteConsumer(ctx, "homelab/vps-docker"))

	a, err := s.GetObject(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, []string{"other"}, a.UsedBy)

	b, err := s.GetObject(ctx, "b")
	require.NoError(t, err)
	require.Empty(t, b.UsedBy)

	consumers, err := s.ListConsumers(ctx)
	require.NoError(t, err)
	require.NotContains(t, consumers, "homelab/vps-docker")
}

func TestDeleteConsumerUnknownReturnsError(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	err := s.DeleteConsumer(context.Background(), "nonexistent")
	require.ErrorIs(t, err, store.ErrUnknownConsumer)
}

func TestAddConsumerCreatesEntryWithZeroSecrets(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.AddConsumer(ctx, "homelab/new-device"))

	page, err := s.ListConsumersPage(ctx, store.ConsumerFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, store.ConsumerPage{
		Consumers: []store.ConsumerEntry{{Name: "homelab/new-device", SecretCount: 0}},
		Total:     1,
	}, page)

	consumers, err := s.ListConsumers(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/new-device"}, consumers)
}

func TestAddConsumerRejectsDuplicateAlreadyAddedDirectly(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.AddConsumer(ctx, "homelab/new-device"))

	err := s.AddConsumer(ctx, "homelab/new-device")
	require.ErrorIs(t, err, store.ErrConsumerAlreadyExists)
}

func TestAddConsumerRejectsNameAlreadyUsedByAnObject(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.CreateObject(ctx, "a", []byte("v"), []string{"homelab/vps-docker"}, ""))

	err := s.AddConsumer(ctx, "homelab/vps-docker")
	require.ErrorIs(t, err, store.ErrConsumerAlreadyExists)
}

func TestRenameConsumerAddedDirectlyWithZeroSecrets(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.AddConsumer(ctx, "homelab/typo"))

	entry, err := s.RenameConsumer(ctx, "homelab/typo", "homelab/fixed")
	require.NoError(t, err)
	require.Equal(t, store.ConsumerEntry{Name: "homelab/fixed", SecretCount: 0}, entry)

	consumers, err := s.ListConsumers(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"homelab/fixed"}, consumers)
}

func TestDeleteConsumerAddedDirectlyWithZeroSecrets(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	ctx := context.Background()
	require.NoError(t, s.AddConsumer(ctx, "homelab/unused"))

	require.NoError(t, s.DeleteConsumer(ctx, "homelab/unused"))

	consumers, err := s.ListConsumers(ctx)
	require.NoError(t, err)
	require.Empty(t, consumers)
}
