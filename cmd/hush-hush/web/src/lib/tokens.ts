import type { TokenStatus } from './api';
import {
	TOKEN_TTL_SECONDS_DEFAULT,
	TOKEN_TTL_SECONDS_MAX,
	TOKEN_TTL_SECONDS_MIN,
} from './api-limits';

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
