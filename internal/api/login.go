package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// errLoginCeremonyExpired, errMalformedAssertionResponse, and
// errClonedAuthenticator are the client-error outcomes
// verifyLogin/handleFinishLogin can report, as static sentinels rather
// than errors constructed inline (go-lint.md's err113).
var (
	errLoginCeremonyExpired       = errors.New("login ceremony expired or not found")
	errMalformedAssertionResponse = errors.New("malformed credential response")
	errClonedAuthenticator        = errors.New("signature counter did not advance - possible cloned authenticator")
)

// LoginFinishRequest is the POST /auth/login/finish body. Matches
// components.schemas.LoginFinishRequest in api/openapi.yaml.
type LoginFinishRequest struct {
	Credential json.RawMessage `json:"credential"`
}

// handleBeginLogin starts a WebAuthn login ceremony.
func handleBeginLogin(s objectStore, wa *webauthn.WebAuthn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if wa == nil {
			writeError(w, r, http.StatusBadRequest, errPublicURLRequired.Error())

			return
		}

		user, err := loadAdminUser(r.Context(), s)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		if len(user.credentials) == 0 {
			writeError(w, r, http.StatusBadRequest, "no admin account registered yet")

			return
		}

		assertion, session, err := wa.BeginLogin(user)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "begin login: "+err.Error())

			return
		}

		if err := saveCeremonyAndSetCookie(w, r, s, "login", session); err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, assertion)
	}
}

// handleFinishLogin completes a WebAuthn login ceremony: verifies the
// assertion, rejects a signature-counter regression as a possible cloned
// authenticator, updates the credential's usage, and issues a fresh
// session with a regenerated id.
func handleFinishLogin(s objectStore, wa *webauthn.WebAuthn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if wa == nil {
			writeError(w, r, http.StatusBadRequest, errPublicURLRequired.Error())

			return
		}

		user, err := loadAdminUser(r.Context(), s)
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		var req LoginFinishRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		credential, err := verifyLogin(r, s, wa, user, req.Credential)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, err.Error())

			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		if err := s.UpdateCredentialUsage(r.Context(), encodeCredentialID(credential.ID), credential.Authenticator.SignCount, now); err != nil {
			writeInternalError(w, r, err)

			return
		}

		if err := issueSession(w, r, s); err != nil {
			writeInternalError(w, r, err)

			return
		}

		clearCeremonyCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

// verifyLogin consumes the in-progress ceremony, parses the browser's
// assertion response, verifies it, and rejects a signature-counter
// regression as a possible cloned authenticator (auth/spec.md's "Cloned
// authenticator detected" scenario) - split out of handleFinishLogin so
// that function's own branching stays readable.
func verifyLogin(r *http.Request, s objectStore, wa *webauthn.WebAuthn, user adminUser, raw json.RawMessage) (*webauthn.Credential, error) {
	session, err := consumeCeremony(r, s, "login")
	if err != nil {
		return nil, errLoginCeremonyExpired
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(raw)
	if err != nil {
		return nil, errMalformedAssertionResponse
	}

	credential, err := wa.ValidateLogin(user, session, parsed)
	if err != nil {
		return nil, fmt.Errorf("verify login: %w", err)
	}

	if credential.Authenticator.CloneWarning {
		return nil, errClonedAuthenticator
	}

	return credential, nil
}
