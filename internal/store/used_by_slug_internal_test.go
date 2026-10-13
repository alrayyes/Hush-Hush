package store

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	errIndex  = errors.New("constraint failed: UNIQUE constraint failed: used_by.slug, used_by.consumer (2067)")
	errLocked = errors.New("database is locked")
)

func TestAsSlugConflictTranslatesOnlyTheUniqueIndexViolation(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, asSlugConflict(errIndex, ErrVariantConflict), ErrVariantConflict)
	require.ErrorIs(t, asSlugConflict(errIndex, ErrAlreadyExists), ErrAlreadyExists)
	require.Equal(t, errLocked, asSlugConflict(errLocked, ErrVariantConflict))
	require.NoError(t, asSlugConflict(nil, ErrVariantConflict))
}
