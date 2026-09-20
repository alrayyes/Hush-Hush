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
