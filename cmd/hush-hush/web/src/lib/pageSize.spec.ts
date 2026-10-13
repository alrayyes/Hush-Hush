import { describe, expect, it } from 'vitest';
import {
	AUDIT_SIZES,
	parsePageSize,
	SECRETS_SIZES,
	showCursorSizeSelector,
	showSizeSelector,
} from './pageSize';

// alrayyes/Hush-Hush#897: Secrets and the audit log let the reader pick how many
// rows a page holds. The size lives in the URL as ?size=, and the selector
// appears only where it can matter.

describe('the option lists', () => {
	it('are 20/50/100 for Secrets and 25/50/100 for the audit log', () => {
		expect([...SECRETS_SIZES]).toEqual([20, 50, 100]);
		expect([...AUDIT_SIZES]).toEqual([25, 50, 100]);
	});
});

describe('parsePageSize', () => {
	const params = (size?: string) =>
		new URLSearchParams(size === undefined ? '' : `size=${size}`);

	it('reads an offered size', () => {
		expect(parsePageSize(params('50'), SECRETS_SIZES)).toBe(50);
	});

	it('falls back to the first option when there is no size', () => {
		expect(parsePageSize(params(), SECRETS_SIZES)).toBe(20);
		expect(parsePageSize(params(), AUDIT_SIZES)).toBe(25);
	});

	it.each(['7', '0', '-20', '20.5', 'abc', '', '1000'])(
		'falls back for %j, which is not offered',
		(value) => {
			expect(parsePageSize(params(value), SECRETS_SIZES)).toBe(20);
		},
	);

	it('does not take a size from the other list', () => {
		expect(parsePageSize(params('25'), SECRETS_SIZES)).toBe(20);
	});
});

// A list with a total shows the selector once it has more rows than the
// smallest option. Below that, no size could change anything.
describe('showSizeSelector', () => {
	it('is off at or under the smallest option', () => {
		expect(showSizeSelector(18, SECRETS_SIZES)).toBe(false);
		expect(showSizeSelector(20, SECRETS_SIZES)).toBe(false);
	});

	it('is on above it, even when one page holds everything', () => {
		expect(showSizeSelector(21, SECRETS_SIZES)).toBe(true);
		expect(showSizeSelector(45, SECRETS_SIZES)).toBe(true);
	});
});

// A cursor list has no total. The selector shows when another page exists at
// this size, or when the size isn't the smallest option, so someone who picked
// 100 can always pick fewer again.
describe('showCursorSizeSelector', () => {
	it('is on when an older page exists', () => {
		expect(showCursorSizeSelector(25, AUDIT_SIZES, false, true)).toBe(true);
	});

	it('is on when a newer page exists', () => {
		expect(showCursorSizeSelector(25, AUDIT_SIZES, true, false)).toBe(true);
	});

	it('is off for one page at the smallest size', () => {
		expect(showCursorSizeSelector(25, AUDIT_SIZES, false, false)).toBe(false);
	});

	it('stays on for one page at a larger size, so it can be changed back', () => {
		expect(showCursorSizeSelector(100, AUDIT_SIZES, false, false)).toBe(true);
	});
});
