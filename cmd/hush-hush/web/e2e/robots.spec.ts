import { expect, test } from '@playwright/test';

// alrayyes/Hush-Hush#834: hush-hush is an internal tool, so every page tells
// search engines not to index it or follow its links. The X-Robots-Tag header
// is the server's half; this is the HTML shell's. /robots.txt must stay
// permissive, or a crawler never reads the noindex.
for (const path of ['/login', '/changelog']) {
	test(`${path} carries a noindex, nofollow robots meta tag`, async ({
		page,
	}) => {
		await page.goto(path);
		await expect(page.locator('head meta[name="robots"]')).toHaveAttribute(
			'content',
			'noindex, nofollow',
		);
	});
}

test('robots.txt does not disallow crawling, so the noindex is readable', async ({
	request,
}) => {
	const response = await request.get('/robots.txt');
	expect(response.ok()).toBe(true);
	expect(await response.text()).not.toMatch(/^\s*Disallow:\s*\/\s*$/m);
});
