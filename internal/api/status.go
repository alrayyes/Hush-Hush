package api

import "net/http"

// AuthStatus is the body GET /auth/status answers with. Matches
// components.schemas.AuthStatus in api/openapi.yaml.
type AuthStatus struct {
	Bootstrapped bool `json:"bootstrapped"`
	// Authenticated is whether the request carries a valid session. It
	// reports the caller's own state only, and an expired or unknown
	// cookie is false, not a 401.
	Authenticated bool `json:"authenticated"`
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

		writeJSON(w, http.StatusOK, AuthStatus{
			Bootstrapped:  len(user.credentials) > 0,
			Authenticated: hasValidSession(r, s),
		})
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

// handleAuthIdentity reports the owner's escrowed identity public key. It
// takes a write bearer token or a session (requireWriteAccess, no CSRF
// check: it's a read), never an anonymous caller: an age public key isn't
// secret on its own, but every other endpoint that exposes stored data stays
// behind a credential, and this keeps the same posture.
//
// A bearer token is accepted so an SDK client, which holds an API key and
// never a session, can fetch the key it has to add as a sealing recipient
// to honour keep_readable_copy. There is one user row to read the key from,
// so the answer is the same whichever credential asks.
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
