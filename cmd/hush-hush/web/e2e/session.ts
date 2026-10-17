import { fileURLToPath } from 'node:url';

// The signed-in session the journey leaves behind. The journey registers the
// only passkey the server accepts, so every spec after it starts from this
// state (cookies, including the CSRF token) rather than signing in again.
// e2e/.auth/ is not committed.
export const SESSION_FILE = fileURLToPath(
	new URL('.auth/session.json', import.meta.url),
);
