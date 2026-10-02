// formatTimestamp renders an ISO 8601 instant as a locale-formatted date
// + time for display - every timestamp in the app (secrets
// Created/Updated, settings tables, audit-log) rendered the raw ISO
// string with no formatting utility anywhere until now
// (alrayyes/hush-hush#426). Absolute, not relative - user-confirmed over
// "3 days ago" style timestamps: it needs no live-refresh timer, and an
// audit log entry is a record, not a feed
// (https://cloudscape.design/patterns/general/timestamps/). Callers
// already hold the original ISO string for a <time datetime>/title
// tooltip, so this only ever needs to return the display text.
//
// options exists so a test can pin the locale/timezone for a
// deterministic assertion - real call sites always omit it and get the
// viewer's own browser locale and local timezone.
export function formatTimestamp(
	iso: string,
	options?: { locale?: string; timeZone?: string },
): string {
	return new Intl.DateTimeFormat(options?.locale, {
		dateStyle: 'medium',
		timeStyle: 'short',
		timeZone: options?.timeZone,
	}).format(new Date(iso));
}

// A write bearer token and a consumer read token both carry `revoked` and
// `expires_at` but no server-computed "is this dead" field - the server
// never needed one before the purge action (#439/#441) needed to gate on
// it too. Revoked wins over expired when both are true - "Revoked" names
// an admin action, "Expired" is just time passing, and the former is the
// more informative label of the two (openspec/changes/token-purge-web-ui/
// design.md).
export function isTokenDead(token: {
	revoked: boolean;
	expires_at: string;
}): boolean {
	return token.revoked || new Date(token.expires_at).getTime() <= Date.now();
}

// formatRemaining turns an expiry instant into a short "time left" label
// for a token's badge: whole days, else whole hours, else whole minutes,
// rounding down so it never promises more time than there is. `now` is a
// parameter so a test can pin the clock; call sites pass `new Date()`.
export function formatRemaining(expiresAt: string, now: Date): string {
	const ms = new Date(expiresAt).getTime() - now.getTime();

	if (ms <= 0) return 'expired';

	const minutes = Math.floor(ms / 60_000);
	if (minutes < 1) return '<1m left';
	if (minutes < 60) return `${minutes}m left`;

	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h left`;

	return `${Math.floor(hours / 24)}d left`;
}
