// The logic behind the shared pager (alrayyes/Hush-Hush#892), kept apart from
// the components so it needs no rendering to test. Pages are 1-based.

export type PageItem = number | 'gap';

// How many pages a list of `total` rows makes. Never less than one, so an
// empty list is still "page 1".
export function totalPages(total: number, pageSize: number): number {
	return Math.max(1, Math.ceil(total / pageSize));
}

// A list that fits on one page needs no pager and no "Showing" text. This is
// the whole rule; each list passes its own page size.
export function showPager(total: number, pageSize: number): boolean {
	return total > pageSize;
}

// The page numbers to show, from the design's spec sheet: the first page, the
// last page, and the current page with a neighbour on each side (three wide,
// kept inside the list at either end). A run of skipped pages becomes a gap,
// except a run of exactly one page, which is shown as itself: "..." in place of
// a single number saves nothing and costs a tap.
export function pageRange(page: number, pages: number): PageItem[] {
	if (pages <= 1) return [1];

	const width = Math.min(3, pages);
	const start = Math.min(Math.max(page - 1, 1), pages - width + 1);
	const shown = new Set([1, pages]);
	for (let n = start; n < start + width; n++) shown.add(n);

	const sorted = [...shown].sort((a, b) => a - b);
	const items: PageItem[] = [];
	sorted.forEach((n, i) => {
		const prev = sorted[i - 1];
		if (prev !== undefined && n - prev === 2) items.push(prev + 1);
		else if (prev !== undefined && n - prev > 2) items.push('gap');
		items.push(n);
	});

	return items;
}

// The first and last row number on a page, for "Showing 21–40 of 137".
export function summaryRange(
	page: number,
	pageSize: number,
	total: number,
): { from: number; to: number } {
	return {
		from: (page - 1) * pageSize + 1,
		to: Math.min(page * pageSize, total),
	};
}

// Keeps a page number from a URL inside the list. A page past the end, a zero
// or a value that isn't a number all land on a page that has rows.
export function clampPage(
	page: number,
	total: number,
	pageSize: number,
): number {
	if (!Number.isFinite(page)) return 1;

	return Math.min(Math.max(Math.trunc(page), 1), totalPages(total, pageSize));
}

// The rows on one page of an already-loaded list. A page past the end gives
// the last page, the same clamp the pager uses, so the rows and the "Showing"
// text always agree.
export function slicePage<T>(items: T[], page: number, pageSize: number): T[] {
	const current = clampPage(page, items.length, pageSize);

	return items.slice((current - 1) * pageSize, current * pageSize);
}
