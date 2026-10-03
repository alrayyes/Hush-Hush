// Tags are free-form labels on an object (alrayyes/hush-hush#500). The
// secrets list turns the ones in use into filter pills, so these are the two
// pure steps: count them, and narrow a list to one.

import { TAG_PATTERN, TAGS_MAX_ITEMS } from './api-limits';

export interface Tagged {
	tags: string[];
}

export interface TagCount {
	tag: string;
	count: number;
}

// tagCounts counts each tag across objects - once per object, even if an
// object lists a tag twice - most-used first, then alphabetical, so the
// pill order is stable between loads.
export function tagCounts(objects: readonly Tagged[]): TagCount[] {
	const counts = new Map<string, number>();

	for (const object of objects) {
		for (const tag of new Set(object.tags)) {
			counts.set(tag, (counts.get(tag) ?? 0) + 1);
		}
	}

	return [...counts]
		.map(([tag, count]) => ({ tag, count }))
		.sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag));
}

// filterByTag keeps the objects carrying `tag`, or all of them when no tag
// is selected.
export function filterByTag<T extends Tagged>(
	objects: T[],
	tag: string | null,
): T[] {
	if (tag === null) return objects;

	return objects.filter((object) => object.tags.includes(tag));
}

// parseTags turns the Tags field's text into the set the API stores:
// comma-separated, trimmed, lowercased, empties and repeats dropped, in the
// order first written. A space inside a tag is kept, so validateTags can
// name it instead of it silently becoming two tags.
export function parseTags(input: string): string[] {
	const tags = input
		.split(',')
		.map((tag) => tag.trim().toLowerCase())
		.filter((tag) => tag !== '');

	return [...new Set(tags)];
}

const tagPattern = new RegExp(TAG_PATTERN);

// validateTags is the fast-feedback copy of the API's own rule - the pattern
// and the count both come from api/openapi.yaml, never typed here - and
// returns what to tell the user, or null when the set is acceptable. The
// server still decides.
export function validateTags(tags: readonly string[]): string | null {
	const invalid = tags.find((tag) => !tagPattern.test(tag));

	if (invalid !== undefined) return `"${invalid}" is not a valid tag.`;

	if (tags.length > TAGS_MAX_ITEMS) {
		return `A secret can have at most ${TAGS_MAX_ITEMS} tags.`;
	}

	return null;
}

// formatTags is parseTags' inverse, for filling the field from an object's
// current tags.
export function formatTags(tags: readonly string[]): string {
	return tags.join(', ');
}
