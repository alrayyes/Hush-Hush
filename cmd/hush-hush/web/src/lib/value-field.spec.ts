import { describe, expect, it } from 'vitest';
import { normalizeValue } from './value-field';

describe('normalizeValue', () => {
	it('trims spaces, tabs and newlines at both ends', () => {
		expect(normalizeValue('  \n\ttoken\r\n ')).toBe('token');
	});

	it('keeps whitespace inside the value', () => {
		expect(normalizeValue(' a  b\nc ')).toBe('a  b\nc');
	});

	it('turns a whitespace-only value into the empty string', () => {
		expect(normalizeValue(' \n\t ')).toBe('');
	});
});
