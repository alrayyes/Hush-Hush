package api

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"filippo.io/age"
)

// What the server can check about a value without decrypting it
// (alrayyes/hush-hush#538). The client seals before upload and the server
// never sees plaintext (ADR 0001), so it can't know how many recipients a
// ciphertext was sealed to or whether any key is right. It can read the age
// header: refuse a value that isn't an age file, or that names no recipient
// at all, so a broken or empty ciphertext can't be stored through any client.
var (
	errNotAnAgeFile = errors.New("value is not a well-formed age file")
	errNoRecipients = errors.New("value is an age file with no recipients, so nothing could decrypt it")
	errValueNotB64  = errors.New("value is not valid base64")
)

// neverMatches is an age.Identity that unwraps nothing. Handing it to
// age.Decrypt makes age parse and validate the header with its own parser,
// then report that no identity matched. No key is involved and no payload is
// read, so nothing is decrypted.
type neverMatches struct{}

func (neverMatches) Unwrap([]*age.Stanza) ([]byte, error) { return nil, age.ErrIncorrectIdentity }

// validateAgeCiphertext reports whether value's header is a well-formed age
// header naming at least one recipient. It reads only the header.
func validateAgeCiphertext(value []byte) error {
	_, err := age.Decrypt(bytes.NewReader(value), neverMatches{})

	var noMatch *age.NoIdentityMatchError
	switch {
	case errors.As(err, &noMatch):
		// The header parsed and listed recipients; ours just isn't one.
		return nil
	case err != nil && strings.Contains(err.Error(), "no recipient stanzas"):
		// age has no typed error for this, so the one distinction worth
		// telling a caller is read from its message; a test pins it.
		return errNoRecipients
	default:
		return errNotAnAgeFile
	}
}

// decodeFailure maps a request-body decode error to the status and message a
// handler should answer with. A value that isn't valid base64 is a 422: the
// body is well-formed JSON, the value in it just isn't an age file. Anything
// else is a body that didn't parse at all, a 400.
func decodeFailure(err error) (status int, message string) {
	if _, ok := errors.AsType[base64.CorruptInputError](err); ok {
		return http.StatusUnprocessableEntity, errValueNotB64.Error()
	}

	return http.StatusBadRequest, "malformed request body"
}
