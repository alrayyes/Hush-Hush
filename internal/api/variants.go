package api

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/alrayyes/hush-hush/internal/store"
)

// One name can hold a different value per consumer (alrayyes/hush-hush#668,
// ADR 33). Objects that share a name are variants, each under its own UUID id.
// A consumer token reads the variant that names its consumer; a write token or
// a session says which one with ?id= or ?consumer=.

var (
	errAmbiguousName   = errors.New("name has several variants; pass id to say which")
	errVariantConflict = errors.New("a consumer can be in only one variant of a name")
	errInvalidObjectID = errors.New("id must be a UUID")
	errIDAndConsumer   = errors.New("pass id or consumer to pick a variant, not both")
)

var objectIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// selectorFrom reads the optional id and consumer query parameters into the
// store option that picks a variant. Neither is no option: a name with one
// variant is addressed as it always was. Naming both is a 400, since they say
// the same thing two ways and could disagree. It has already answered a
// malformed id. The consumer's length is already checked for every route.
func selectorFrom(w http.ResponseWriter, r *http.Request) ([]store.ObjectOption, bool) {
	query := r.URL.Query()
	id, consumer := query.Get("id"), query.Get("consumer")

	switch {
	case id != "" && consumer != "":
		writeError(w, r, http.StatusBadRequest, errIDAndConsumer.Error())

		return nil, false
	case id != "":
		if !objectIDPattern.MatchString(id) {
			writeError(w, r, http.StatusBadRequest, errInvalidObjectID.Error())

			return nil, false
		}

		return []store.ObjectOption{store.WithID(id)}, true
	case consumer != "":
		return []store.ObjectOption{store.ForConsumer(consumer)}, true
	}

	return nil, true
}

// objectLookupFailed answers for the errors a lookup or write by name can
// return, and says whether it did. Anything else is the caller's to handle.
func objectLookupFailed(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "unknown object")
	case errors.Is(err, store.ErrAmbiguousSlug):
		writeError(w, r, http.StatusConflict, errAmbiguousName.Error())
	case errors.Is(err, store.ErrVariantConflict):
		writeError(w, r, http.StatusConflict, errVariantConflict.Error())
	default:
		return false
	}

	return true
}
