import { describe, expect, it } from 'vitest';
import type { ObjectMetadata } from './api';
import {
	hasActiveFilters,
	matchesFilters,
	objectFiltersToSearch,
	parseObjectFilters,
	unionObjects,
} from './object-filters';

function object(over: Partial<ObjectMetadata> = {}): ObjectMetadata {
	return {
		id: 'i',
		slug: 's',
		tags: [],
		created_at: '2026-10-05T12:00:00Z',
		updated_at: '2026-10-09T12:00:00Z',
		created_by: { type: 'session', id: 'admin' },
		...over,
	} as ObjectMetadata;
}

describe('parseObjectFilters', () => {
	it('reads repeated used_by, the creator and the two dates', () => {
		const filters = parseObjectFilters(
			new URLSearchParams(
				'used_by=a&used_by=b&created_by=admin&created_since=2026-10-01&updated_since=2026-10-08',
			),
		);

		expect(filters).toEqual({
			usedBy: ['a', 'b'],
			createdBy: 'admin',
			createdSince: '2026-10-01',
			updatedSince: '2026-10-08',
		});
	});

	it('is empty for no parameters, and drops blanks and malformed dates', () => {
		expect(parseObjectFilters(new URLSearchParams())).toEqual({ usedBy: [] });
		expect(
			parseObjectFilters(
				new URLSearchParams('used_by=&created_since=yesterday&created_by='),
			),
		).toEqual({ usedBy: [] });
	});
});

describe('objectFiltersToSearch', () => {
	it('round-trips through parseObjectFilters', () => {
		const filters = {
			usedBy: ['a', 'b c'],
			createdBy: 'token:x',
			createdSince: '2026-10-01',
		};

		expect(
			parseObjectFilters(new URLSearchParams(objectFiltersToSearch(filters))),
		).toEqual(filters);
	});

	it('is empty when nothing is set', () => {
		expect(objectFiltersToSearch({ usedBy: [] })).toBe('');
	});
});

describe('matchesFilters', () => {
	it('matches everything with no filters', () => {
		expect(matchesFilters(object(), { usedBy: [] })).toBe(true);
	});

	it('filters by who created it, using the name the table shows', () => {
		expect(matchesFilters(object(), { usedBy: [], createdBy: 'admin' })).toBe(
			true,
		);
		expect(
			matchesFilters(object(), { usedBy: [], createdBy: 'token:abc' }),
		).toBe(false);
		expect(
			matchesFilters(object({ created_by: undefined }), {
				usedBy: [],
				createdBy: 'admin',
			}),
		).toBe(false);
	});

	it('includes the chosen day itself for created and updated since', () => {
		expect(
			matchesFilters(object(), { usedBy: [], createdSince: '2026-10-05' }),
		).toBe(true);
		expect(
			matchesFilters(object(), { usedBy: [], createdSince: '2026-10-06' }),
		).toBe(false);
		expect(
			matchesFilters(object(), { usedBy: [], updatedSince: '2026-10-09' }),
		).toBe(true);
		expect(
			matchesFilters(object(), { usedBy: [], updatedSince: '2026-10-10' }),
		).toBe(false);
	});

	it('needs every set filter to match', () => {
		expect(
			matchesFilters(object(), {
				usedBy: [],
				createdBy: 'admin',
				createdSince: '2026-10-06',
			}),
		).toBe(false);
	});
});

describe('hasActiveFilters', () => {
	it('is true for any set filter', () => {
		expect(hasActiveFilters({ usedBy: [] })).toBe(false);
		expect(hasActiveFilters({ usedBy: ['a'] })).toBe(true);
		expect(hasActiveFilters({ usedBy: [], createdBy: 'admin' })).toBe(true);
		expect(hasActiveFilters({ usedBy: [], updatedSince: '2026-10-01' })).toBe(
			true,
		);
	});
});

describe('unionObjects', () => {
	it('lists an object used by several chosen consumers once, sorted by slug', () => {
		const a = object({ id: '1', slug: 'a' });
		const b = object({ id: '2', slug: 'b' });
		const c = object({ id: '3', slug: 'c' });

		expect(
			unionObjects([
				[b, a],
				[b, c],
			]).map((o) => o.id),
		).toEqual(['1', '2', '3']);
	});
});
