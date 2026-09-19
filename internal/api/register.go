package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// ceremonyTTL is how long a Begin call's challenge stays valid for the
// matching Finish call - openspec/changes/web-ui/design.md's "a few
// minutes" decision.
const ceremonyTTL = 5 * time.Minute

const (
	sessionCookieName  = "session"
	csrfCookieName     = "csrf_token"
	ceremonyCookieName = "webauthn_ceremony"
	sessionTTL         = 30 * 24 * time.Hour
)

// Credential is a registered passkey's HTTP-facing shape. Matches
// components.schemas.Credential in api/openapi.yaml - never the public
// key material, sign counter, or AAGUID, which stay internal to the
// ceremony logic (store.Credential carries those).
type Credential struct {
	ID         string `json:"id"`
	Nickname   string `json:"nickname"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

// credentialFromStore builds the HTTP-facing shape. store.Credential.ID is
// already the base64url text form (encodeCredentialID's output, written
// by handleFinishRegistration below), so this is a plain field copy, not
// a re-encode.
func credentialFromStore(c store.Credential) Credential {
	return Credential{ID: c.ID, CreatedAt: c.CreatedAt, Nickname: c.Nickname, LastUsedAt: c.LastUsedAt}
}

// RegistrationFinishRequest is the POST /auth/register/finish body.
// Matches components.schemas.RegistrationFinishRequest in
// api/openapi.yaml. Credential is kept as a raw json.RawMessage rather
// than decoded here - go-webauthn's own protocol.ParseCredentialCreationResponseBytes
// is what actually parses a WebAuthn attestation response, and decoding
// it twice with two different parsers is how the two disagree.
type RegistrationFinishRequest struct {
	Credential json.RawMessage `json:"credential"`
	Nickname   string          `json:"nickname"`
}

// handleBeginRegistration starts a WebAuthn registration ceremony.
// Anonymous when no admin account exists yet; requires a valid session
// otherwise (openspec/changes/web-ui/specs/auth/spec.md's "Passkey
// registration" requirement).
func handleBeginRegistration(s objectStore, wa *webauthn.WebAuthn) http.HandlerFunc {
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

		if !registrationIsAuthorized(r, s, user) {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid session")

			return
		}

		creation, session, err := wa.BeginRegistration(user)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "begin registration: "+err.Error())

			return
		}

		if err := saveCeremonyAndSetCookie(w, r, s, "registration", session); err != nil {
			writeInternalError(w, r, err)

			return
		}

		writeJSON(w, http.StatusOK, creation)
	}
}

// handleFinishRegistration completes a WebAuthn registration ceremony.
// The first successful call (no admin account yet) creates the account;
// every later call adds another credential to it.
func handleFinishRegistration(s objectStore, wa *webauthn.WebAuthn) http.HandlerFunc {
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

		if !registrationIsAuthorized(r, s, user) {
			writeError(w, r, http.StatusUnauthorized, "missing or invalid session")

			return
		}

		var req RegistrationFinishRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "malformed request body")

			return
		}

		credential, err := verifyRegistration(r, s, wa, user, req.Credential)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err.Error())

			return
		}

		stored := buildStoredCredential(credential, req.Nickname, len(user.credentials)+1)
		if err := persistAndAuthenticate(w, r, s, stored); err != nil {
			writeInternalError(w, r, err)

			return
		}

		clearCeremonyCookie(w)
		writeJSON(w, http.StatusCreated, credentialFromStore(stored))
	}
}

// persistAndAuthenticate stores a newly verified credential and, if the
// caller isn't already authenticated, issues a session - only the
// bootstrap case (no session yet) issues one, since adding a passkey to
// an already-authenticated session leaves that session untouched
// (auth/spec.md's "Registering an additional passkey" scenario).
func persistAndAuthenticate(w http.ResponseWriter, r *http.Request, s objectStore, stored store.Credential) error {
	if err := s.CreateCredential(r.Context(), stored); err != nil {
		return fmt.Errorf("create credential: %w", err)
	}

	if !hasValidSession(r, s) {
		if err := issueSession(w, r, s); err != nil {
			return err
		}
	}

	return nil
}

// registrationIsAuthorized reports whether a registration ceremony may
// proceed: anonymous when no admin account exists yet, otherwise only
// with a valid session.
func registrationIsAuthorized(r *http.Request, s objectStore, user adminUser) bool {
	return len(user.credentials) == 0 || hasValidSession(r, s)
}

// errRegistrationCeremonyExpired and errMalformedCredentialResponse are
// the two client-error outcomes verifyRegistration can report, as static
// sentinels rather than errors constructed inline (go-lint.md's err113).
var (
	errRegistrationCeremonyExpired = errors.New("registration ceremony expired or not found")
	errMalformedCredentialResponse = errors.New("malformed credential response")
)

// verifyRegistration consumes the in-progress ceremony, parses the
// browser's attestation response, and verifies it - split out of
// handleFinishRegistration so that function's own branching stays
// readable.
func verifyRegistration(r *http.Request, s objectStore, wa *webauthn.WebAuthn, user adminUser, raw json.RawMessage) (*webauthn.Credential, error) {
	session, err := consumeCeremony(r, s, "registration")
	if err != nil {
		return nil, errRegistrationCeremonyExpired
	}

	parsed, err := protocol.ParseCredentialCreationResponseBytes(raw)
	if err != nil {
		return nil, errMalformedCredentialResponse
	}

	credential, err := wa.CreateCredential(user, session, parsed)
	if err != nil {
		return nil, fmt.Errorf("verify registration: %w", err)
	}

	return credential, nil
}

// buildStoredCredential builds the row to persist for a just-verified
// registration, falling back to a generated nickname when the caller
// didn't supply one.
func buildStoredCredential(credential *webauthn.Credential, nickname string, fallbackIndex int) store.Credential {
	if nickname == "" {
		nickname = fmt.Sprintf("passkey %d", fallbackIndex)
	}

	return store.Credential{
		ID: encodeCredentialID(credential.ID), PublicKey: credential.PublicKey,
		SignCount: credential.Authenticator.SignCount, AAGUID: encodeCredentialID(credential.Authenticator.AAGUID),
		Nickname: nickname, CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// randomToken returns n random bytes hex-encoded - the same shape
// internal/store's own randomHex uses for token ids, duplicated here
// rather than exported across the package boundary for one helper.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return hex.EncodeToString(b), nil
}
