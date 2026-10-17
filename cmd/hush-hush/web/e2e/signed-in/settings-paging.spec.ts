import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

// alrayyes/Hush-Hush#896: the Settings lists page on their own, 10 at a time,
// each with its page in the address. Twelve more bearer tokens and consumer
// tokens push both past a page; the passkeys stay on one. The rows and pager
// text of each list, a list's page taken from its own parameter, the links
// keeping the other list's position, a page past the end, the consumer filter
// and the phone cards are settings.svelte.spec.ts's. What these real clicks, the
// scroll and the reload prove is that the address really carries both positions.
test('the Settings lists page on their own and the address carries both positions', async ({
	page,
	context,
}) => {
	const csrfToken =
		(await context.cookies()).find((c) => c.name === 'csrf_token')?.value ?? '';
	const headers = { 'X-CSRF-Token': csrfToken };

	const seeded = await Promise.all(
		Array.from({ length: 12 }, (_, i) => {
			const n = String(i).padStart(2, '0');

			return [
				page.request.post('/tokens', {
					headers,
					data: { description: `pager bearer ${n}`, ttl_seconds: 86_400 },
				}),
				page.request.post('/consumer-tokens', {
					headers,
					data: {
						consumer: `pager-consumer-${n}`,
						description: `pager consumer ${n}`,
						ttl_seconds: 86_400,
					},
				}),
			];
		}).flat(),
	);

	try {
		for (const created of seeded) {
			expect(created.ok(), await created.text()).toBe(true);
		}

		await page.setViewportSize({ width: 1280, height: 900 });
		await page.goto('/settings');
		const bearerPager = page.getByRole('navigation', {
			name: 'Bearer tokens pages',
		});
		const consumerPager = page.getByRole('navigation', {
			name: 'Consumer tokens pages',
		});
		await bearerPager.getByRole('link', { name: 'Page 2' }).click();
		await expect(page).toHaveURL(/[?&]token_page=2(&|$)/);
		await consumerPager.getByRole('link', { name: 'Next' }).click();
		await expect(page).toHaveURL(/token_page=2/);
		await expect(page).toHaveURL(/consumer_token_page=2/);

		// The consumer pager is below the fold, so clicking it scrolled the page.
		// Paging it doesn't jump back to the top.
		expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(0);

		// Both positions survive a reload.
		await page.reload();
		await expect(bearerPager).toContainText('Showing 11');
		await expect(consumerPager).toContainText('Showing 11');

		// The paged state, in both themes.
		for (const theme of ['light', 'dark']) {
			await page.evaluate(
				(t) => document.documentElement.setAttribute('data-theme', t),
				theme,
			);
			await page.waitForFunction(() => document.getAnimations().length === 0);
			const results = await new AxeBuilder({ page })
				.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
				.analyze();
			expect(results.violations, theme).toEqual([]);
		}
	} finally {
		// Leave the lists as they were, for whatever runs next.
		for (const [path, list] of [
			['/tokens', 'tokens'],
			['/consumer-tokens', 'consumer-tokens'],
		] as const) {
			const ids = (
				(await (await page.request.get(`${path}?limit=500`)).json()) as {
					id: string;
					description: string;
				}[]
			)
				.filter((t) => /^pager (bearer|consumer) \d\d$/.test(t.description))
				.map((t) => t.id);
			for (const id of ids) {
				await page.request.delete(`/${list}/${id}`, { headers });
				await page.request.delete(`/${list}/${id}/purge`, { headers });
			}
		}
	}
});
