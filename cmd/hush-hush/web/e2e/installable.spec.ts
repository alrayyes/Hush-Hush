import { expect, test } from '@playwright/test';

// alrayyes/Hush-Hush#881: the web UI can be installed from the browser and
// opens as its own window. That takes a web app manifest with icons, a service
// worker, and a theme colour for the window chrome. The service worker keeps
// the app shell only: an API response can carry a secret's ciphertext or a
// token's metadata, so none of it may land in a cache.

test('the page links a manifest that describes an installable app', async ({
	page,
	request,
}) => {
	await page.goto('/login');

	const href = await page
		.locator('head link[rel="manifest"]')
		.getAttribute('href');
	expect(href).toBeTruthy();

	const response = await request.get(href as string);
	expect(response.ok()).toBe(true);
	expect(response.headers()['content-type']).toContain(
		'application/manifest+json',
	);

	const manifest = await response.json();
	expect(manifest).toMatchObject({
		name: 'hush-hush',
		short_name: 'hush-hush',
		start_url: '/',
		scope: '/',
		display: 'standalone',
	});
	expect(manifest.theme_color).toMatch(/^#[0-9a-f]{6}$/i);
	expect(manifest.background_color).toMatch(/^#[0-9a-f]{6}$/i);
});

test('the manifest icons exist at the sizes a browser asks for', async ({
	page,
	request,
}) => {
	await page.goto('/login');
	const href = await page
		.locator('head link[rel="manifest"]')
		.getAttribute('href');
	const manifest = await (await request.get(href as string)).json();

	const sizes = manifest.icons.map(
		(icon: { sizes: string; purpose?: string }) =>
			`${icon.sizes} ${icon.purpose ?? 'any'}`,
	);
	expect(sizes).toEqual(
		expect.arrayContaining(['192x192 any', '512x512 any', '512x512 maskable']),
	);

	for (const icon of manifest.icons as { src: string }[]) {
		const response = await request.get(icon.src);
		expect(response.ok(), icon.src).toBe(true);
		expect(response.headers()['content-type'], icon.src).toBe('image/png');
	}
});

test('the page sets a theme colour and an iOS home-screen icon', async ({
	page,
}) => {
	await page.goto('/login');

	// One per colour scheme, so the window chrome follows the OS theme.
	await expect(page.locator('head meta[name="theme-color"]')).toHaveCount(2);
	await expect(
		page.locator('head link[rel="apple-touch-icon"]'),
	).toHaveAttribute('href', /\.png$/);
});

test('Chromium reports no installability error', async ({ page }) => {
	await page.goto('/login');
	await page.evaluate(() => navigator.serviceWorker.ready);

	const cdp = await page.context().newCDPSession(page);
	const { installabilityErrors } = await cdp.send(
		'Page.getInstallabilityErrors',
	);

	expect(installabilityErrors).toEqual([]);
});

test('a service worker takes control of the app', async ({ page }) => {
	await page.goto('/login');
	await page.evaluate(() => navigator.serviceWorker.ready);
	await page.reload();

	const scope = await page.evaluate(
		() => navigator.serviceWorker.controller?.scriptURL,
	);
	expect(scope).toMatch(/\/service-worker\.js$/);
});

test('the service worker caches the app shell and never an API response', async ({
	page,
}) => {
	await page.goto('/login');
	await page.evaluate(() => navigator.serviceWorker.ready);
	await page.reload();
	// A same-origin API call made under the worker's control.
	await page.evaluate(() => fetch('/healthz'));

	const cached = await page.evaluate(async () => {
		const urls: string[] = [];
		for (const name of await caches.keys()) {
			const cache = await caches.open(name);
			for (const request of await cache.keys()) {
				urls.push(new URL(request.url).pathname);
			}
		}
		return urls;
	});

	expect(cached).toContain('/manifest.webmanifest');
	expect(cached).not.toContain('/healthz');
	for (const path of cached) {
		expect(path, path).toMatch(
			/^\/(_app\/|icons\/|(manifest\.webmanifest|robots\.txt|CHANGELOG\.md|LICENSE\.txt)$)/,
		);
	}
});
