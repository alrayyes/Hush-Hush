package store_test

import (
	"fmt"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestAConflictListsEveryClashInOrderAndCapsThemAtTen(t *testing.T) {
	t.Parallel()

	var clashes []store.Clash
	for i := 12; i >= 1; i-- {
		clashes = append(clashes, store.Clash{Consumer: fmt.Sprintf("c%02d", i), Slug: "db", ObjectID: fmt.Sprintf("id-%02d", i)})
	}

	err := &store.ConflictError{Clashes: clashes}

	msg := err.Error()
	require.Contains(t, msg, `consumer "c01" is already in the secret named "db" (id id-01); consumer "c02"`)
	require.Contains(t, msg, `consumer "c10" is already in the secret named "db" (id id-10)`)
	require.NotContains(t, msg, `"c11"`)
	require.Contains(t, msg, "and 2 more")
}
