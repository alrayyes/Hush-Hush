import { describe, expect, it } from 'vitest';
import { TAGS_MAX_ITEMS } from './api-limits';
import {
	filterByTag,
	formatTags,
	parseTags,
	type Tagged,
	tagCounts,
	validateTags,
} from './tags';

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

describe('parseTags', () => {
	it('splits on commas and trims each tag', () => {
		expect(parseTags('prod, homelab ,backup')).toEqual([
			'prod',
			'homelab',
			'backup',
		]);
	});

	it('lowercases, drops duplicates and empty entries, keeping first-seen order', () => {
		expect(parseTags('Prod,, homelab, prod,')).toEqual(['prod', 'homelab']);
	});

	it('is empty for blank input', () => {
		expect(parseTags('')).toEqual([]);
		expect(parseTags(' , ')).toEqual([]);
	});

	it('keeps a space inside a tag so validation can reject it', () => {
		expect(parseTags('prod homelab')).toEqual(['prod homelab']);
	});
});

describe('validateTags', () => {
	it('accepts tags the spec allows', () => {
		expect(validateTags(['prod', 'a.b_c/d-e', 'x'.repeat(32)])).toBeNull();
		expect(validateTags([])).toBeNull();
	});

	it('names the first tag that breaks the pattern', () => {
		expect(validateTags(['prod', 'prod homelab'])).toBe(
			'"prod homelab" is not a valid tag.',
		);
		expect(validateTags(['pr@d'])).toBe('"pr@d" is not a valid tag.');
	});

	it('rejects a tag over the length the pattern allows', () => {
		expect(validateTags(['x'.repeat(33)])).toBe(
			`"${'x'.repeat(33)}" is not a valid tag.`,
		);
	});

	it('rejects more tags than the spec allows', () => {
		const tooMany = Array.from(
			{ length: TAGS_MAX_ITEMS + 1 },
			(_, i) => `t${i}`,
		);

		expect(validateTags(tooMany)).toBe(
			`A secret can have at most ${TAGS_MAX_ITEMS} tags.`,
		);
	});
});

describe('formatTags', () => {
	it('joins tags the way the field shows them', () => {
		expect(formatTags(['prod', 'homelab'])).toBe('prod, homelab');
		expect(formatTags([])).toBe('');
	});
});
