import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { resolveTheme, THEME_COLORS, themeColor } from './theme';

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

// alrayyes/Hush-Hush#885: the browser's window chrome (and an installed app's
// title bar) takes its color from <meta name="theme-color">. It follows the
// theme the page is showing, which the toggle can set against the OS scheme.

const root = resolve(import.meta.dirname, '../..');
const read = (path: string) => readFileSync(resolve(root, path), 'utf8');

describe('themeColor', () => {
	it('is the page background of the light theme', () => {
		expect(themeColor('light')).toBe('#ffffff');
	});

	it('is the page background of the dark theme', () => {
		expect(themeColor('dark')).toBe('#121212');
	});
});

// The two colors are written out in three places: here, app.css's tokens, and
// app.html's inline script, which runs before this module loads. These keep
// them from drifting apart.
describe('THEME_COLORS', () => {
	it('match the --color-bg token of each theme in app.css', () => {
		const tokens = [
			...read('src/app.css').matchAll(/--color-bg:\s*(#[0-9a-f]+)/gi),
		]
			.map((m) => m[1].toLowerCase())
			.map((hex) =>
				hex.length === 4
					? `#${[...hex.slice(1)].map((c) => c + c).join('')}`
					: hex,
			);

		expect(tokens).toEqual(
			expect.arrayContaining([THEME_COLORS.light, THEME_COLORS.dark]),
		);
	});

	it("are the ones app.html's inline script sets before first paint", () => {
		const html = read('src/app.html');

		expect(html).toContain(`light: '${THEME_COLORS.light}'`);
		expect(html).toContain(`dark: '${THEME_COLORS.dark}'`);
	});
});
