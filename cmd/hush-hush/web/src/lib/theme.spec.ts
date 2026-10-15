import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { runInNewContext } from 'node:vm';
import { describe, expect, it } from 'vitest';
import { applyTheme, resolveTheme, THEME_COLORS, themeColor } from './theme';

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

		const colors = Object.values(THEME_COLORS);

		expect(tokens.length).toBeGreaterThan(0);
		// Every token is one of the two colors, and both colors are used.
		expect(tokens.filter((hex) => !colors.includes(hex))).toEqual([]);
		expect(new Set(tokens)).toEqual(new Set(colors));
	});

	it("are the ones app.html's inline script sets before first paint", () => {
		const html = read('src/app.html');

		expect(html).toContain(`light: '${THEME_COLORS.light}'`);
		expect(html).toContain(`dark: '${THEME_COLORS.dark}'`);
	});
});

// What the page does with a theme, against a stand-in document: the meta tag
// is the only thing the browser reads, so recording its content is enough.
function fakeDocument() {
	const meta = { content: '' };
	const root = { theme: '' };
	return {
		meta,
		root,
		document: {
			documentElement: {
				setAttribute: (_name: string, value: string) => {
					root.theme = value;
				},
			},
			querySelector: () => ({
				setAttribute: (_name: string, value: string) => {
					meta.content = value;
				},
			}),
		},
	};
}

describe('applyTheme', () => {
	it.each(['light', 'dark'] as const)(
		'sets data-theme and the theme-color meta to %s',
		(theme) => {
			const page = fakeDocument();
			const original = globalThis.document;
			globalThis.document = page.document as unknown as Document;
			try {
				applyTheme(theme);
			} finally {
				globalThis.document = original;
			}

			expect(page.root.theme).toBe(theme);
			expect(page.meta.content).toBe(THEME_COLORS[theme]);
		},
	);
});

describe("app.html's inline theme script", () => {
	// The script runs before first paint, so it can't import theme.ts; run
	// the real text from the file against a stand-in browser instead.
	const script = /<script>([\s\S]*?)<\/script>/
		.exec(read('src/app.html'))?.[1]
		.replace(/%sveltekit\.head%/g, '');

	function load(stored: string | null, prefersDark: boolean) {
		const page = fakeDocument();
		runInNewContext(script ?? '', {
			document: page.document,
			localStorage: { getItem: () => stored },
			window: { matchMedia: () => ({ matches: prefersDark }) },
		});
		return page;
	}

	it('follows the OS scheme when nothing is stored', () => {
		const page = load(null, true);

		expect(page.root.theme).toBe('dark');
		expect(page.meta.content).toBe(THEME_COLORS.dark);
	});

	it('lets a stored choice win over the OS scheme', () => {
		const page = load('dark', false);

		expect(page.root.theme).toBe('dark');
		expect(page.meta.content).toBe(THEME_COLORS.dark);
	});

	it('shows light when nothing is stored and the OS does not ask for dark', () => {
		const page = load(null, false);

		expect(page.root.theme).toBe('light');
		expect(page.meta.content).toBe(THEME_COLORS.light);
	});
});
