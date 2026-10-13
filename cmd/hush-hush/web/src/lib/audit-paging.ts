export type AuditOrder = 'asc' | 'desc';

// cursorParams is the cursor for the page after the one whose last entry has
// this id. GET /audit-log's `after` means "newer than this id" in either
// order, so a newest-first log pages onward with `before` instead
// (alrayyes/hush-hush#756). No cursor is the first page.
export function cursorParams(
	order: AuditOrder,
	cursor: number | undefined,
): { after?: number; before?: number } {
	if (cursor === undefined) return {};

	return order === 'desc' ? { before: cursor } : { after: cursor };
}

export function flipOrder(order: AuditOrder): AuditOrder {
	return order === 'desc' ? 'asc' : 'desc';
}

// The audit log shows 25 entries at a time (alrayyes/Hush-Hush#895).
export const AUDIT_PAGE_SIZE = 25;

// splitPage turns the rows a request returned into one page and a flag for
// whether another page follows. The log has no total, so a request asks for
// one row more than the page: if it came back, there is more to read, and it is
// dropped from what is shown.
export function splitPage<T>(
	rows: T[],
	size: number,
): { entries: T[]; hasMore: boolean } {
	return { entries: rows.slice(0, size), hasMore: rows.length > size };
}

export type PageStep = 'previous' | 'next';

// pagerState maps reading onward and back (the cursor stack and the "has more"
// flag) onto the Newer and Older buttons a person sees. In a newest-first log,
// onward is older; in an oldest-first log, onward is newer.
export function pagerState(
	order: AuditOrder,
	hasPrevious: boolean,
	hasMore: boolean,
): { canNewer: boolean; canOlder: boolean; newer: PageStep; older: PageStep } {
	if (order === 'desc') {
		return {
			canNewer: hasPrevious,
			canOlder: hasMore,
			newer: 'previous',
			older: 'next',
		};
	}

	return {
		canNewer: hasMore,
		canOlder: hasPrevious,
		newer: 'next',
		older: 'previous',
	};
}
