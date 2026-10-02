import type { AuditLogEntry, AuditLogQuery } from './api';

// GET /audit-log returns oldest first, and `limit` keeps the oldest rows -
// so asking for limit=3 yields an object's first three events, never its
// latest. There's no newest-first option, so the latest few come from
// paging forward with the `after` cursor to the end of the object's own
// entries and keeping the tail. An object's log is small next to the whole
// log (the filter is server-side), and a page is capped at 500.
const pageSize = 500;

// recentEntriesForObject returns up to `count` of an object's most recent
// audit entries, newest first. fetchPage is queryAuditLog in the app, passed
// in so the paging can be tested without a network.
export async function recentEntriesForObject(
	fetchPage: (query: AuditLogQuery) => Promise<AuditLogEntry[]>,
	objectId: string,
	count: number,
): Promise<AuditLogEntry[]> {
	let tail: AuditLogEntry[] = [];
	let after: number | undefined;

	for (;;) {
		const page = await fetchPage({
			object_id: objectId,
			limit: pageSize,
			after,
		});

		tail = [...tail, ...page].slice(-count);

		if (page.length < pageSize) {
			break;
		}

		after = page[page.length - 1].id;
	}

	return tail.reverse();
}
