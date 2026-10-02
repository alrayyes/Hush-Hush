import { describe, expect, it } from 'vitest';
import { filterByTag, type Tagged, tagCounts } from './tags';

const objects: (Tagged & { slug: string })[] = [
	{ slug: 'a', tags: ['prod', 'homelab'] },
	{ slug: 'b', tags: ['prod'] },
	{ slug: 'c', tags: ['backup', 'prod'] },
	{ slug: 'd', tags: [] },
	{ slug: 'e', tags: ['homelab'] },
];

describe('tagCounts', () => {
	it('counts each tag across objects', () => {
		expect(tagCounts(objects)).toContainEqual({ tag: 'prod', count: 3 });
		expect(tagCounts(objects)).toContainEqual({ tag: 'homelab', count: 2 });
		expect(tagCounts(objects)).toContainEqual({ tag: 'backup', count: 1 });
	});

	it('orders by count, then name', () => {
		expect(tagCounts(objects).map((t) => t.tag)).toEqual([
			'prod',
			'homelab',
			'backup',
		]);
		expect(
			tagCounts([{ tags: ['b'] }, { tags: ['a'] }]).map((t) => t.tag),
		).toEqual(['a', 'b']);
	});

	it('is empty when no object has a tag', () => {
		expect(tagCounts([{ tags: [] }])).toEqual([]);
		expect(tagCounts([])).toEqual([]);
	});

	it('counts an object once per tag even if the list repeats it', () => {
		expect(tagCounts([{ tags: ['prod', 'prod'] }])).toEqual([
			{ tag: 'prod', count: 1 },
		]);
	});
});

describe('filterByTag', () => {
	it('keeps only objects carrying the tag', () => {
		expect(filterByTag(objects, 'prod').map((o) => o.slug)).toEqual([
			'a',
			'b',
			'c',
		]);
	});

	it('returns everything when no tag is selected', () => {
		expect(filterByTag(objects, null)).toHaveLength(5);
	});

	it('returns nothing for a tag no object carries', () => {
		expect(filterByTag(objects, 'staging')).toEqual([]);
	});
});
