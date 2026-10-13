import { expect, test } from '@playwright/test';

// alrayyes/Hush-Hush#881: the web UI can be installed. The manifest, the icons
// and the head tags are checked without a browser in src/lib/installable.spec.ts,
// and the service worker's caching rule in src/lib/shell-cache.spec.ts. What
// needs a real browser is left here: that Chromium finds the page installable,
// and that the worker takes control and holds only the app shell.

test('Chromium reports no installability error', async ({ page }) => {
	await page.goto('/login');
	await page.evaluate(() => navigator.serviceWorker.ready);

	const cdp = await page.context().newCDPSession(page);
	const { installabilityErrors } = await cdp.send(
		'Page.getInstallabilityErrors',
	);

	expect(installabilityErrors).toEqual([]);
});

test('a service worker takes control and caches only the app shell', async ({
	page,
}) => {
	await page.goto('/login');
	await page.evaluate(() => navigator.serviceWorker.ready);
	await page.reload();

	const worker = await page.evaluate(
		() => navigator.serviceWorker.controller?.scriptURL,
	);
	expect(worker).toMatch(/\/service-worker\.js$/);

	// The wiring: the worker answers a shell file, and leaves an API call to
	// the network. Playwright knows which responses came from the worker.
	const shellResponse = page.waitForResponse('**/manifest.webmanifest');
	const apiResponse = page.waitForResponse('**/healthz');
	await page.evaluate(() => {
		void fetch('/manifest.webmanifest');
		void fetch('/healthz');
	});
	expect((await shellResponse).fromServiceWorker()).toBe(true);
	expect((await apiResponse).fromServiceWorker()).toBe(false);

	// The worker fills its cache when it installs, so this needs no wait.
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
	for (const path of cached) {
		expect(path, path).toMatch(
			/^\/(_app\/|icons\/|(manifest\.webmanifest|robots\.txt|CHANGELOG\.md|LICENSE\.txt)$)/,
		);
	}
});
