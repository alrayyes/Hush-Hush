package api

import (
	"errors"
	"regexp"
)

// slugPattern is components.schemas.ObjectSlug's pattern in api/openapi.yaml:
// the one rule for a secret's name on create and on rename
// (alrayyes/hush-hush#859). Either case is allowed and names are
// case-sensitive, so a name is whatever its owner chose, within what a URL
// path segment carries safely.
var slugPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// maxSlugLength is the longest name, matching the maxLength on the request
// fields and ObjectSlug.
const maxSlugLength = 128

var errInvalidSlug = errors.New("slug must start with a letter or digit and use only letters, digits, '_' and '-', up to 128 characters")

// validSlug reports whether name follows the naming rule.
func validSlug(name string) bool {
	return len(name) <= maxSlugLength && slugPattern.MatchString(name)
}
