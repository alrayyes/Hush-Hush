// Tags are free-form labels on an object (alrayyes/hush-hush#500). The
// secrets list turns the ones in use into filter pills, so these are the two
// pure steps: count them, and narrow a list to one.

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
