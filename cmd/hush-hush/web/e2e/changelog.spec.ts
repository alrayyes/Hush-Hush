import { readFileSync } from 'node:fs';
import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

// alrayyes/hush-hush#485: the changelog is one card per release, not one
// long document. /changelog is a public page, so unlike the authenticated
// journey this needs no passkey registration and can run on its own.
const changelog = readFileSync(
	new URL('../../../../CHANGELOG.md', import.meta.url),
	'utf8',
);
const releaseHeadings = [...changelog.matchAll(/^## \[?(\d+\.\d+\.\d+)/gm)].map(
	(m) => m[1],
);
const newest = releaseHeadings[0];

test.use({ permissions: ['clipboard-read', 'clipboard-write'] });

test('the changelog shows a card per release, newest first', async ({
	page,
}) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto('/changelog');

	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Changelog');
	const cards = page.getByRole('article');
	await expect(cards).toHaveCount(releaseHeadings.length);

	const first = cards.first();
	await expect(first.getByRole('heading', { level: 2 })).toHaveText(
		`v${newest}`,
	);
	await expect(first.locator('time')).toBeVisible();

	// Every entry carries its category as a word, never colour alone.
	const badges = first.getByTestId('entry-badge');
	expect(await badges.count()).toBeGreaterThan(0);
	for (const word of await badges.allTextContents()) {
		expect(word).toMatch(/^[A-Z ]+$/);
	}
});

test('a release card copies its image tag and links its release page', async ({
	page,
}) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto('/changelog');

	const first = page.getByRole('article').first();
	const copy = first.getByRole('button', {
		name: `Copy image tag for ${newest}`,
	});
	await expect(first).toContainText(`ghcr.io/alrayyes/hush-hush:${newest}`);

	const release = first.getByRole('link', {
		name: `Release page for ${newest}`,
	});
	await expect(release).toHaveAttribute(
		'href',
		`https://github.com/alrayyes/Hush-Hush/releases/tag/v${newest}`,
	);

	for (const control of [copy, release]) {
		const box = await control.boundingBox();
		expect(box?.width).toBeGreaterThanOrEqual(44);
		expect(box?.height).toBeGreaterThanOrEqual(44);
	}

	// The tag, with no v prefix, is what lands on the clipboard.
	await copy.click();
	expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
		`ghcr.io/alrayyes/hush-hush:${newest}`,
	);
	await expect(
		first.getByRole('button', { name: `Copied image tag for ${newest}` }),
	).toBeVisible();
});

test('the changelog has no horizontal scroll at 320px and no a11y violations', async ({
	page,
}) => {
	await page.setViewportSize({ width: 320, height: 720 });
	await page.goto('/changelog');
	await expect(page.getByRole('article').first()).toBeVisible();

	const { scrollWidth, clientWidth } = await page.evaluate(() => ({
		scrollWidth: document.documentElement.scrollWidth,
		clientWidth: document.documentElement.clientWidth,
	}));
	expect(scrollWidth).toBeLessThanOrEqual(clientWidth);

	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(results.violations).toEqual([]);
});
