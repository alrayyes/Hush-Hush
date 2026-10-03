package api

// Exposed to the external test package (api_test) so it can exercise the
// unexported ciphertext check directly, the usual way to test an internal
// function without moving the tests into the package.
var (
	ValidateAgeCiphertext = validateAgeCiphertext
	ErrNotAnAgeFile       = errNotAnAgeFile
	ErrNoRecipients       = errNoRecipients
)
