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
