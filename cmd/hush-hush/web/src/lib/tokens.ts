import type { TokenStatus } from './api';
import {
	TOKEN_TTL_SECONDS_DEFAULT,
	TOKEN_TTL_SECONDS_MAX,
	TOKEN_TTL_SECONDS_MIN,
} from './api-limits';
import { formatRemaining } from './datetime';

const SECONDS_PER_DAY = 86_400;

// The TTL dialogs ask for whole days, so this is the API's default and
// limits (TokenTtlSeconds in api/openapi.yaml, through the generated
// limits) rounded inward to days: never a lifetime the server would refuse.
export const TTL_DAYS = {
	min: Math.ceil(TOKEN_TTL_SECONDS_MIN / SECONDS_PER_DAY),
	max: Math.floor(TOKEN_TTL_SECONDS_MAX / SECONDS_PER_DAY),
	default: Math.round(TOKEN_TTL_SECONDS_DEFAULT / SECONDS_PER_DAY),
};

export function ttlDaysToSeconds(days: number): number {
	return days * SECONDS_PER_DAY;
}

// The word the settings page shows for a token's status. The status itself
// is the server's answer (alrayyes/hush-hush#536); this only capitalises it.
// With none sent it shows nothing, because guessing one from the browser
// clock is the thing #536 removed.
export function tokenStatusLabel(status: TokenStatus | undefined): string {
	if (status === undefined) return '';

	return status.charAt(0).toUpperCase() + status.slice(1);
}

// tokenRemaining is the short word under a token's expiry, in the card and
// the table alike: the time left while the server calls the token active,
// otherwise its status. The status is the server's answer (#536); the clock
// only describes how long an active token has left.
export function tokenRemaining(
	token: { status?: TokenStatus; expires_at: string },
	now: Date = new Date(),
): string {
	if (token.status === 'revoked' || token.status === 'expired') {
		return token.status;
	}

	return formatRemaining(token.expires_at, now) ?? token.status ?? '';
}
