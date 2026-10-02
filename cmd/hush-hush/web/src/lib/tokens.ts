import type { TokenStatus } from './api';

// The word the settings page shows for a token's status. The status itself
// is the server's answer (alrayyes/hush-hush#536); this only capitalises it.
// With none sent it shows nothing, because guessing one from the browser
// clock is the thing #536 removed.
export function tokenStatusLabel(status: TokenStatus | undefined): string {
	if (status === undefined) return '';

	return status.charAt(0).toUpperCase() + status.slice(1);
}
