import { describe, expect, it } from 'vitest';
import { cursorParams, flipOrder } from './audit-paging';

describe('cursorParams', () => {
	it('pages an oldest-first log forward with after', () => {
		expect(cursorParams('asc', 42)).toEqual({ after: 42 });
	});

	it('pages a newest-first log onward with before, since after would return newer entries', () => {
		expect(cursorParams('desc', 42)).toEqual({ before: 42 });
	});

	it('sends no cursor for the first page in either order', () => {
		expect(cursorParams('asc', undefined)).toEqual({});
		expect(cursorParams('desc', undefined)).toEqual({});
	});
});

describe('flipOrder', () => {
	it('swaps newest first and oldest first', () => {
		expect(flipOrder('desc')).toBe('asc');
		expect(flipOrder('asc')).toBe('desc');
	});
});
