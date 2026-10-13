import { describe, expect, it } from 'vitest';
import {
	AUDIT_PAGE_SIZE,
	cursorParams,
	flipOrder,
	pagerState,
	splitPage,
	turnPage,
} from './audit-paging';

describe('cursorParams', () => {
	it('pages an oldest-first log forward with after', () => {
		expect(cursorParams('asc', 42)).toEqual({ after: 42 });
	});

	it('pages a newest-first log onward with before, since after would return newer entries', () => {
		expect(cursorParams('desc', 42)).toEqual({ before: 42 });
	});

	it('sends no cursor for the first page in either order', () => {
		expect(cursorParams('asc', undefined)).toEqual({});
		expect(cursorParams('desc', undefined)).toEqual({});
	});
});

describe('flipOrder', () => {
	it('swaps newest first and oldest first', () => {
		expect(flipOrder('desc')).toBe('asc');
		expect(flipOrder('asc')).toBe('desc');
	});
});

// alrayyes/Hush-Hush#895: the audit log shows 25 entries and pages with Newer
// and Older. The log has no total, so it asks for one row more than a page to
// learn whether another page exists.

describe('AUDIT_PAGE_SIZE', () => {
	it('is 25', () => {
		expect(AUDIT_PAGE_SIZE).toBe(25);
	});
});

describe('splitPage', () => {
	const rows = (n: number) => Array.from({ length: n }, (_, i) => i + 1);

	it('says there is no further page when the rows fit', () => {
		expect(splitPage(rows(25), 25)).toEqual({
			entries: rows(25),
			hasMore: false,
		});
	});

	it('trims the extra row and says there is more', () => {
		expect(splitPage(rows(26), 25)).toEqual({
			entries: rows(25),
			hasMore: true,
		});
	});

	it('handles an empty page', () => {
		expect(splitPage([], 25)).toEqual({ entries: [], hasMore: false });
	});
});

// The cursor stack and the "has more" flag are about reading onward and back.
// What the person sees is Newer and Older, which swap with the sort order: in a
// newest-first log, onward is older, and in an oldest-first log it is newer.
describe('pagerState', () => {
	it('newest first: Newer goes back, Older goes onward', () => {
		expect(pagerState('desc', true, true)).toEqual({
			canNewer: true,
			canOlder: true,
			newer: 'previous',
			older: 'next',
		});
	});

	it('newest first on the first page: Newer is off', () => {
		expect(pagerState('desc', false, true)).toMatchObject({
			canNewer: false,
			canOlder: true,
		});
	});

	it('oldest first: Newer goes onward, Older goes back', () => {
		expect(pagerState('asc', true, true)).toEqual({
			canNewer: true,
			canOlder: true,
			newer: 'next',
			older: 'previous',
		});
	});

	it('oldest first on the first page: Older is off', () => {
		expect(pagerState('asc', false, true)).toMatchObject({
			canNewer: true,
			canOlder: false,
		});
	});

	it('on the last page the onward side is off', () => {
		expect(pagerState('desc', true, false)).toMatchObject({
			canNewer: true,
			canOlder: false,
		});
	});
});

// alrayyes/Hush-Hush#906: a page that fails to load leaves the reader where
// they were, so Newer and Older still mean what they did.
describe('turnPage', () => {
	const row = (id: number) => ({ id });
	const rows = (from: number, count: number) =>
		Array.from({ length: count }, (_, i) => row(from - i));
	const first = {
		entries: rows(100, 25),
		hasMore: true,
		after: undefined,
		stack: [] as (number | undefined)[],
	};
	const size = 25;

	it('goes onward from the last entry, and remembers the cursor it left', async () => {
		const load = async (after: number | undefined) => {
			expect(after).toBe(76);

			return rows(75, 26);
		};
		const next = await turnPage(first, 'next', load, size);

		expect(next.after).toBe(76);
		expect(next.stack).toEqual([undefined]);
		expect(next.entries).toHaveLength(25);
		expect(next.hasMore).toBe(true);
	});

	it('goes back to the cursor it came from', async () => {
		const second = { ...first, after: 76, stack: [undefined] };
		const load = async (after: number | undefined) => {
			expect(after).toBeUndefined();

			return rows(100, 26);
		};
		const back = await turnPage(second, 'previous', load, size);

		expect(back.after).toBeUndefined();
		expect(back.stack).toEqual([]);
	});

	it('starts over for the first page', async () => {
		const deep = { ...first, after: 51, stack: [undefined, 76] };
		const back = await turnPage(deep, 'first', async () => rows(100, 26), size);

		expect(back.stack).toEqual([]);
		expect(back.after).toBeUndefined();
	});

	it('changes nothing when the load fails, so a retry goes where it meant to', async () => {
		const fail = async () => {
			throw new Error('network');
		};

		await expect(turnPage(first, 'next', fail, size)).rejects.toThrow('network');
		expect(first.stack).toEqual([]);
		expect(first.after).toBeUndefined();

		const retry = await turnPage(first, 'next', async () => rows(75, 26), size);

		expect(retry.stack).toEqual([undefined]);
		expect(retry.after).toBe(76);
	});

	it('does not move past the ends', async () => {
		const load = async () => {
			throw new Error('should not load');
		};
		const last = { ...first, hasMore: false };

		await expect(turnPage(last, 'next', load, size)).resolves.toBe(last);
		await expect(turnPage(first, 'previous', load, size)).resolves.toBe(first);
	});
});
