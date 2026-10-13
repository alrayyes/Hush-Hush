import { describe, expect, it } from 'vitest';
import {
	clampPage,
	pageRange,
	showPager,
	summaryRange,
	totalPages,
} from './pageMath';

// alrayyes/Hush-Hush#892: the logic behind the shared pager. The component
// itself is in Pager.spec.ts; this is what needs no rendering.

describe('totalPages', () => {
	it.each([
		[0, 20, 1],
		[20, 20, 1],
		[21, 20, 2],
		[137, 20, 7],
		[340, 50, 7],
	])('%i rows at %i a page is %i pages', (total, pageSize, pages) => {
		expect(totalPages(total, pageSize)).toBe(pages);
	});
});

describe('showPager', () => {
	it('is false when everything fits on one page', () => {
		expect(showPager(20, 20)).toBe(false);
		expect(showPager(0, 20)).toBe(false);
	});

	it('is true as soon as there is one row more than a page', () => {
		expect(showPager(21, 20)).toBe(true);
	});
});

// The sheet's rule: always keep the first page, the last page and the current
// page with one neighbour each side (three wide, kept inside the list at the
// ends). Skipped pages become a gap, but a gap of exactly one page shows the
// page instead, since "..." standing in for one number saves nothing.
describe('pageRange', () => {
	it.each([
		[1, 1, [1]],
		[1, 2, [1, 2]],
		[2, 2, [1, 2]],
		[1, 3, [1, 2, 3]],
		[1, 5, [1, 2, 3, 4, 5]],
		[1, 7, [1, 2, 3, 'gap', 7]],
		[3, 9, [1, 2, 3, 4, 'gap', 9]],
		[4, 9, [1, 2, 3, 4, 5, 'gap', 9]],
		[5, 9, [1, 'gap', 4, 5, 6, 'gap', 9]],
		[7, 7, [1, 'gap', 5, 6, 7]],
		[9, 9, [1, 'gap', 7, 8, 9]],
	])('page %i of %i is %j', (page, pages, expected) => {
		expect(pageRange(page, pages)).toEqual(expected);
	});
});

describe('summaryRange', () => {
	it('names the first and last row of the page', () => {
		expect(summaryRange(2, 20, 137)).toEqual({ from: 21, to: 40 });
	});

	it('stops at the total on a short last page', () => {
		expect(summaryRange(7, 20, 137)).toEqual({ from: 121, to: 137 });
	});
});

describe('clampPage', () => {
	it.each([
		[1, 137, 20, 1],
		[7, 137, 20, 7],
		[8, 137, 20, 7],
		[0, 137, 20, 1],
		[-3, 137, 20, 1],
		[Number.NaN, 137, 20, 1],
		[5, 0, 20, 1],
	])('page %s of %i rows at %i a page is page %i', (page, total, size, out) => {
		expect(clampPage(page, total, size)).toBe(out);
	});
});
