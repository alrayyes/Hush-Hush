import { describe, expect, it } from 'vitest';
import { detailChanges } from './object-details';

const initial = { slug: 'a', description: 'old', tags: ['x', 'y'] };

describe('detailChanges', () => {
	it('is undefined when nothing changed', () => {
		expect(detailChanges(initial, { ...initial })).toBeUndefined();
	});

	it('sends only the fields that changed', () => {
		expect(detailChanges(initial, { ...initial, description: 'new' })).toEqual({
			description: 'new',
		});
		expect(detailChanges(initial, { ...initial, slug: 'b' })).toEqual({
			slug: 'b',
		});
		expect(detailChanges(initial, { ...initial, tags: ['y'] })).toEqual({
			tags: ['y'],
		});
	});

	it('sends an emptied description and emptied tags, which clear them', () => {
		expect(
			detailChanges(initial, { ...initial, description: '', tags: [] }),
		).toEqual({ description: '', tags: [] });
	});

	it('treats a missing description as empty', () => {
		const bare = { slug: 'a', tags: [] };

		expect(detailChanges(bare, { ...bare, description: '' })).toBeUndefined();
	});

	it('treats reordered tags as a change', () => {
		expect(detailChanges(initial, { ...initial, tags: ['y', 'x'] })).toEqual({
			tags: ['y', 'x'],
		});
	});
});
