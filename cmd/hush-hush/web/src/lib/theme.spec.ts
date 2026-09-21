import { describe, expect, it } from 'vitest';
import { resolveTheme } from './theme';

describe('resolveTheme', () => {
	it('uses a stored light preference over the OS setting', () => {
		expect(resolveTheme('light', true)).toBe('light');
	});

	it('uses a stored dark preference over the OS setting', () => {
		expect(resolveTheme('dark', false)).toBe('dark');
	});

	it('falls back to the OS preference when nothing is stored', () => {
		expect(resolveTheme(null, true)).toBe('dark');
		expect(resolveTheme(null, false)).toBe('light');
	});

	it('falls back to the OS preference when the stored value is not a theme', () => {
		expect(resolveTheme('system', true)).toBe('dark');
	});
});
