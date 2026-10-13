package store

import (
	"fmt"
	"sort"
	"strings"
)

// maxListedClashes caps how many clashes a ConflictError spells out.
const maxListedClashes = 10

// Clash is one consumer that is already in a secret under a name: the reason a
// create, update or rename was refused.
type Clash struct {
	Consumer string
	Slug     string
	ObjectID string
}

// ConflictError is what ErrAlreadyExists and ErrVariantConflict carry when a
// write is refused because of consumers (alrayyes/hush-hush#851): which
// consumers, in which secrets. errors.Is still matches the sentinel, so a
// caller that only wants the status ignores the detail.
type ConflictError struct {
	// Slug is the name the write targeted; it is what the message says when
	// there are no Clashes, only a name that is already taken.
	Slug    string
	Clashes []Clash
	kind    error
}

// Unwrap lets errors.Is match ErrAlreadyExists or ErrVariantConflict.
func (e *ConflictError) Unwrap() error { return e.kind }

// Error lists every clash in consumer order, up to maxListedClashes.
func (e *ConflictError) Error() string {
	if len(e.Clashes) == 0 {
		return fmt.Sprintf("a secret named %q already exists and this one names no consumer", e.Slug)
	}

	clashes := append([]Clash(nil), e.Clashes...)
	sort.SliceStable(clashes, func(i, j int) bool { return clashes[i].Consumer < clashes[j].Consumer })

	listed := clashes
	if len(listed) > maxListedClashes {
		listed = listed[:maxListedClashes]
	}

	parts := make([]string, 0, len(listed)+1)
	for _, c := range listed {
		parts = append(parts, fmt.Sprintf("consumer %q is already in the secret named %q (id %s)", c.Consumer, c.Slug, c.ObjectID))
	}

	if more := len(clashes) - len(listed); more > 0 {
		parts = append(parts, fmt.Sprintf("and %d more", more))
	}

	return strings.Join(parts, "; ")
}
