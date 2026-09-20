package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/go-webauthn/webauthn/webauthn"
)

// errPublicURLRequired is what every /auth/* ceremony endpoint answers
// with when PUBLIC_URL is unset - a configuration error, not a runtime
// one, so it's checked per request rather than failing the server at
// startup (existing deployments keep working with no web UI configured
// per openspec/changes/web-ui/design.md's Migration Plan).
var errPublicURLRequired = errors.New("PUBLIC_URL must be set to use the web UI's WebAuthn endpoints")

// adminUserID is the single admin account's WebAuthn user handle. There is
// exactly one admin account (openspec/changes/web-ui/design.md's "single
// admin account, multiple passkeys" decision), so this is a fixed value
// rather than a generated one - nothing distinguishes one admin account
// from another because there is only ever one.
var adminUserID = []byte("admin")

// newWebAuthn builds the relying-party instance from publicURL (the
// server's own PUBLIC_URL config) - RPID and RPOrigins are derived from
// it, never read from request headers, per design.md's "Relying party
// ID/origin" decision: guessing a host from a request is a known WebAuthn
// phishing risk.
func newWebAuthn(publicURL string) (*webauthn.WebAuthn, error) {
	if publicURL == "" {
		return nil, errPublicURLRequired
	}

	u, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("parse PUBLIC_URL: %w", err)
	}

	w, err := webauthn.New(&webauthn.Config{
		RPID:          u.Hostname(),
		RPDisplayName: "Hush Hush",
		RPOrigins:     []string{u.Scheme + "://" + u.Host},
	})
	if err != nil {
		return nil, fmt.Errorf("configure webauthn: %w", err)
	}

	return w, nil
}

// adminUser adapts the store's flat credential list to go-webauthn's User
// interface. Credentials is loaded once by the caller (a ceremony's begin
// and finish steps each need a consistent view) rather than queried lazily
// by this type's own methods.
type adminUser struct {
	credentials []store.Credential
}

func (adminUser) WebAuthnID() []byte          { return adminUserID }
func (adminUser) WebAuthnName() string        { return "admin" }
func (adminUser) WebAuthnDisplayName() string { return "admin" }

func (u adminUser) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, 0, len(u.credentials))

	for _, c := range u.credentials {
		id, err := decodeCredentialID(c.ID)
		if err != nil {
			// A credential this package itself wrote should always
			// decode; skipping rather than failing the whole ceremony
			// keeps one corrupted row from locking every other
			// credential out.
			continue
		}

		creds = append(creds, webauthn.Credential{
			ID:        id,
			PublicKey: c.PublicKey,
			Authenticator: webauthn.Authenticator{
				SignCount: c.SignCount,
			},
			// go-webauthn's own ValidateLogin rejects the ceremony
			// outright if this disagrees with what the live assertion
			// reports (alrayyes/hush-hush#260) - has to be reconstructed
			// here the same way SignCount already is.
			Flags: webauthn.CredentialFlags{BackupEligible: c.BackupEligible},
		})
	}

	return creds
}

// loadAdminUser reads every registered credential and wraps them as the
// single admin account go-webauthn's API expects.
func loadAdminUser(ctx context.Context, s objectStore) (adminUser, error) {
	creds, err := s.ListCredentials(ctx)
	if err != nil {
		return adminUser{}, fmt.Errorf("load admin credentials: %w", err)
	}

	return adminUser{credentials: creds}, nil
}

// encodeCredentialID and decodeCredentialID convert between a WebAuthn
// credential's raw binary id and the base64url TEXT form it's stored and
// exposed over HTTP as (api/openapi.yaml's CredentialId schema).
func encodeCredentialID(id []byte) string {
	return base64.RawURLEncoding.EncodeToString(id)
}

func decodeCredentialID(id string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return nil, fmt.Errorf("decode credential id: %w", err)
	}

	return b, nil
}
