import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

// alrayyes/Hush-Hush#881: the files a browser reads to offer the install. The
// browser-side proof is e2e/installable.spec.ts; what is cheap to check
// without a browser is checked here.

const root = resolve(import.meta.dirname, '../..');
const read = (path: string) => readFileSync(resolve(root, path));

interface Icon {
	src: string;
	sizes: string;
	type: string;
	purpose?: string;
}

const manifest = JSON.parse(read('static/manifest.webmanifest').toString());

// A PNG starts with an 8-byte signature, then the IHDR chunk, whose first two
// fields are the width and height.
function pngSize(file: Buffer) {
	const signature = file.subarray(0, 8).toString('hex');
	return {
		signature,
		width: file.readUInt32BE(16),
		height: file.readUInt32BE(20),
	};
}

describe('manifest.webmanifest', () => {
	it('names the app and opens it standalone from the root', () => {
		expect(manifest).toMatchObject({
			name: 'hush-hush',
			short_name: 'hush-hush',
			start_url: '/',
			scope: '/',
			display: 'standalone',
		});
	});

	it('sets the theme and background colors as hex values', () => {
		expect(manifest.theme_color).toMatch(/^#[0-9a-f]{6}$/i);
		expect(manifest.background_color).toMatch(/^#[0-9a-f]{6}$/i);
	});

	it('carries a plain and a maskable icon at the sizes browsers ask for', () => {
		const wanted = (manifest.icons as Icon[]).map(
			(icon) => `${icon.sizes} ${icon.purpose ?? 'any'}`,
		);
		expect(wanted).toEqual(
			expect.arrayContaining([
				'192x192 any',
				'512x512 any',
				'512x512 maskable',
			]),
		);
	});

	it.each(manifest.icons as Icon[])(
		'has a real PNG of the declared size behind $src',
		(icon) => {
			const { signature, width, height } = pngSize(read(`static${icon.src}`));
			const [w, h] = icon.sizes.split('x').map(Number);

			expect({ signature, width, height }).toEqual({
				signature: '89504e470d0a1a0a',
				width: w,
				height: h,
			});
		},
	);
});

describe('app.html', () => {
	const html = read('src/app.html').toString();

	it('links the manifest', () => {
		expect(html).toContain(
			'<link rel="manifest" href="/manifest.webmanifest" />',
		);
	});

	it('sets a theme color for each color scheme', () => {
		expect(html).toMatch(/name="theme-color"[^>]*prefers-color-scheme: light/);
		expect(html).toMatch(/name="theme-color"[^>]*prefers-color-scheme: dark/);
	});

	it('points iOS at a PNG home-screen icon that exists', () => {
		const href = html.match(/rel="apple-touch-icon" href="([^"]+)"/)?.[1];
		expect(href).toMatch(/\.png$/);
		expect(pngSize(read(`static${href}`)).width).toBe(180);
	});
});
