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

export type CursorPage<T> = {
	entries: T[];
	hasMore: boolean;
	// The cursor that produced this page; undefined for the first.
	after: number | undefined;
	// The cursor of each earlier page, so Newer can go back (the API only pages
	// forward, so "previous" is this bookkeeping).
	stack: (number | undefined)[];
};

export type PageMove = 'first' | 'next' | 'previous';

// turnPage reads one page and returns the state to show. It only returns once
// the load has succeeded, and never changes what it was given, so a failed
// load leaves the caller on the page it was on, with Newer and Older as they
// were (alrayyes/Hush-Hush#906). A move that can't happen (onward with no more
// entries, back from the first page) returns the same state without loading.
export async function turnPage<T extends { id: number }>(
	from: CursorPage<T>,
	move: PageMove,
	load: (after: number | undefined) => Promise<T[]>,
	size: number,
): Promise<CursorPage<T>> {
	let after: number | undefined;
	let stack: (number | undefined)[];

	if (move === 'first') {
		after = undefined;
		stack = [];
	} else if (move === 'next') {
		if (!from.hasMore || from.entries.length === 0) return from;
		after = from.entries[from.entries.length - 1].id;
		stack = [...from.stack, from.after];
	} else {
		if (from.stack.length === 0) return from;
		after = from.stack[from.stack.length - 1];
		stack = from.stack.slice(0, -1);
	}

	const split = splitPage(await load(after), size);

	return { ...split, after, stack };
}

// latestOnly hands out a check for each request started: only the most recent
// one is still current. A slower, older response that arrives after a newer
// request began is stale and must be dropped, or it overwrites what the reader
// last asked for (alrayyes/Hush-Hush#922).
export function latestOnly(): () => () => boolean {
	let latest = 0;

	return () => {
		const mine = ++latest;

		return () => mine === latest;
	};
}
