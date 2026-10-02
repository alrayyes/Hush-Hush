package api

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/alrayyes/hush-hush/internal/store"
)

const (
	maxTagsPerObject = 10
	maxTagLength     = 32
)

var (
	errTooManyTags = fmt.Errorf("an object takes at most %d tags", maxTagsPerObject)
	errInvalidTag  = fmt.Errorf("a tag is 1 to %d characters from a-z, 0-9, '.', '_', '/' and '-'", maxTagLength)
	errTagsNil     = errors.New("tags is required")

	tagPattern = regexp.MustCompile(`^[a-z0-9._/-]+$`)
)

// normaliseTag lowercases tag and checks it against the allowed shape.
func normaliseTag(tag string) (string, error) {
	tag = strings.ToLower(tag)
	if tag == "" || len(tag) > maxTagLength || !tagPattern.MatchString(tag) {
		return "", errInvalidTag
	}

	return tag, nil
}

// normaliseTags lowercases, validates and de-duplicates tags, returning
// them sorted. A non-nil empty input stays a non-nil empty slice, so a
// caller can still tell "clear them" from "not given".
func normaliseTags(tags []string) ([]string, error) {
	if tags == nil {
		return nil, errTagsNil
	}

	out := make([]string, 0, len(tags))

	for _, tag := range tags {
		tag, err := normaliseTag(tag)
		if err != nil {
			return nil, err
		}

		if !slices.Contains(out, tag) {
			out = append(out, tag)
		}
	}

	if len(out) > maxTagsPerObject {
		return nil, errTooManyTags
	}

	slices.Sort(out)

	return out, nil
}

// tagsOrEmpty is what a response carries: always an array, never null.
func tagsOrEmpty(tags []string) []string {
	if tags == nil {
		return []string{}
	}

	return tags
}

// createTags is the tag set a create request asks for: none given means an
// empty set, and anything given is normalised and validated.
func createTags(tags []string) ([]string, error) {
	if tags == nil {
		return []string{}, nil
	}

	return normaliseTags(tags)
}

// updateTagOptions turns an update request's optional tags into the store
// option that replaces them; absent means no option, so they stay as they
// are.
func updateTagOptions(tags *[]string) ([]store.ObjectOption, error) {
	if tags == nil {
		return nil, nil
	}

	normalised, err := normaliseTags(*tags)
	if err != nil {
		return nil, err
	}

	return []store.ObjectOption{store.WithTags(normalised)}, nil
}
