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

// formatRemaining turns an expiry instant into a short "time left" label
// for a token's badge: whole days, else whole hours, else whole minutes,
// rounding down so it never promises more time than there is. `now` is a
// parameter so a test can pin the clock; call sites pass `new Date()`.
//
// It only ever describes time left on a token the server calls active. Whether
// a token has expired is the server's answer (`status`, alrayyes/hush-hush
// #536), never the browser clock's. If the browser's clock says the time is
// already up while the server says it isn't, the clock is the one that's off,
// so there is no honest duration to show: it returns null and the caller shows
// the status word instead.
export function formatRemaining(expiresAt: string, now: Date): string | null {
	const ms = new Date(expiresAt).getTime() - now.getTime();

	if (ms <= 0) return null;

	const minutes = Math.floor(ms / 60_000);
	if (minutes < 1) return '<1m left';
	if (minutes < 60) return `${minutes}m left`;

	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h left`;

	return `${Math.floor(hours / 24)}d left`;
}
