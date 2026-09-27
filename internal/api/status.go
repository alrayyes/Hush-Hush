package api

import "net/http"

// AuthStatus is the body GET /auth/status answers with. Matches
// components.schemas.AuthStatus in api/openapi.yaml.
type AuthStatus struct {
	Bootstrapped bool `json:"bootstrapped"`
}

// handleAuthStatus reports whether an admin account exists yet, with no
// side effect - unauthenticated, no cookie set, no ceremony started
// (openspec/changes/gate-passkey-registration-ui/design.md's "New
// endpoint: GET /auth/status" decision). Reuses the same admin-existence
// check registrationIsAuthorized already applies, so the two can't drift.
func handleAuthStatus(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := loadAdminUser(r.Context(), s)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, AuthStatus{Bootstrapped: len(user.credentials) > 0})
	}
}

// OwnerIdentity is the body GET /auth/identity answers with. Matches
// components.schemas.OwnerIdentity in api/openapi.yaml.
type OwnerIdentity struct {
	// PublicKey is the session's user's escrowed writer identity public
	// key (an age recipient string), empty if that user hasn't completed
	// a first registration yet - openspec/changes/client-side-encryption/
	// specs/secret-objects/spec.md's "Opt-in owner-recipient inclusion at
	// create time" requirement: this is what the web UI offers as an
	// additional sealing recipient when the create/edit dialog's
	// owner-recipient checkbox is checked.
	PublicKey string `json:"public_key,omitempty"`
}

// handleAuthIdentity reports the calling session's own escrowed identity
// public key - session-gated (unlike GET /auth/status) since, although an
// age public key isn't secret on its own, every other endpoint that
// exposes stored data (GET /consumers included) stays behind
// requireWriteAccess/requireSession rather than serving an anonymous
// caller, and this keeps the same posture rather than carving out an
// exception.
func handleAuthIdentity(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		publicKey, err := s.CurrentUserPublicKey(r.Context())
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, OwnerIdentity{PublicKey: publicKey})
	}
}
