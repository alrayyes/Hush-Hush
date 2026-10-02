import { describe, expect, it } from 'vitest';
import type { AuditLogEntry, AuditLogQuery } from './api';
import { recentEntriesForObject } from './audit-recent';

function entry(id: number, objectId = 'db_password'): AuditLogEntry {
	return {
		id,
		object_id: objectId,
		action: 'read',
		timestamp: `2026-10-02T10:00:${String(id % 60).padStart(2, '0')}Z`,
		ip: '10.0.0.1',
	};
}

// GET /audit-log returns oldest first and `limit` keeps the oldest rows, so
// the newest ones only show up on a later page. This fake follows that
// contract: filter by object_id, drop everything up to `after`, cap at
// `limit`.
function fakeLog(all: AuditLogEntry[]) {
	const calls: AuditLogQuery[] = [];
	const fetchPage = async (query: AuditLogQuery) => {
		calls.push(query);
		return all
			.filter((e) => e.object_id === query.object_id)
			.filter((e) => query.after === undefined || e.id > query.after)
			.slice(0, query.limit ?? 50);
	};
	return { fetchPage, calls };
}

describe('recentEntriesForObject', () => {
	it('returns the newest entries first, not the oldest', async () => {
		const { fetchPage } = fakeLog([1, 2, 3, 4, 5].map((id) => entry(id)));

		const result = await recentEntriesForObject(fetchPage, 'db_password', 3);

		expect(result.map((e) => e.id)).toEqual([5, 4, 3]);
	});

	it('returns everything, newest first, when there are fewer than asked for', async () => {
		const { fetchPage } = fakeLog([entry(1), entry(2)]);

		const result = await recentEntriesForObject(fetchPage, 'db_password', 3);

		expect(result.map((e) => e.id)).toEqual([2, 1]);
	});

	it('returns an empty list for an object with no entries', async () => {
		const { fetchPage } = fakeLog([entry(1, 'other')]);

		expect(await recentEntriesForObject(fetchPage, 'db_password', 3)).toEqual(
			[],
		);
	});

	it('ignores other objects', async () => {
		const { fetchPage } = fakeLog([
			entry(1),
			entry(2, 'other'),
			entry(3),
			entry(4, 'other'),
		]);

		const result = await recentEntriesForObject(fetchPage, 'db_password', 3);

		expect(result.map((e) => e.id)).toEqual([3, 1]);
	});

	it('pages forward with the after cursor until a short page, then keeps the tail', async () => {
		// 1200 entries: three pages at the 500 cap (500, 500, 200).
		const all = Array.from({ length: 1200 }, (_, i) => entry(i + 1));
		const { fetchPage, calls } = fakeLog(all);

		const result = await recentEntriesForObject(fetchPage, 'db_password', 3);

		expect(result.map((e) => e.id)).toEqual([1200, 1199, 1198]);
		expect(calls).toHaveLength(3);
		expect(calls[0]).toMatchObject({ object_id: 'db_password', limit: 500 });
		expect(calls[0].after).toBeUndefined();
		expect(calls[1].after).toBe(500);
		expect(calls[2].after).toBe(1000);
	});

	it('stops after one request when the first page is short', async () => {
		const { fetchPage, calls } = fakeLog([entry(1), entry(2), entry(3)]);

		await recentEntriesForObject(fetchPage, 'db_password', 3);

		expect(calls).toHaveLength(1);
	});
});
