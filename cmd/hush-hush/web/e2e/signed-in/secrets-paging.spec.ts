import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import * as age from 'age-encryption';

// alrayyes/Hush-Hush#893 and #897: the Secrets table pages 20 at a time, the
// page is in the URL, a short list shows no pager, and the reader picks 20, 50
// or 100 rows. Seeded on top of whatever the other specs left behind, so the
// totals are worked out, not assumed; what each page shows and what the size
// menu offers are overview.svelte.spec.ts's.
test('the Secrets table pages and sizes through the address', async ({
	page,
	context,
}) => {
	const csrfToken =
		(await context.cookies()).find((c) => c.name === 'csrf_token')?.value ?? '';
	const headers = { 'X-CSRF-Token': csrfToken };

	// Sealing here only needs a well-formed age file, which any recipient gives.
	const recipient = await age.identityToRecipient(await age.generateIdentity());
	const sealFor = async (text: string) => {
		const encrypter = new age.Encrypter();
		encrypter.addRecipient(recipient);
		const sealed = await encrypter.encrypt(text);

		return btoa(String.fromCharCode(...sealed));
	};

	const before = await (await page.request.get('/objects')).json();
	const existing = (before.objects ?? before).length as number;
	const pagingSlugs = Array.from(
		{ length: 21 },
		(_, i) => `paging_${String(i).padStart(2, '0')}`,
	);
	const pagingCreated = await Promise.all(
		pagingSlugs.map(async (slug) =>
			page.request.post('/objects', {
				headers,
				data: {
					slug,
					value: await sealFor(slug),
					used_by: [],
					tags: ['paging'],
				},
			}),
		),
	);
	try {
		for (const created of pagingCreated) {
			expect(created.ok(), await created.text()).toBe(true);
		}
		const total = existing + pagingSlugs.length;
		await page.setViewportSize({ width: 1280, height: 900 });
		await page.goto('/');
		const secretsPager = page.getByRole('navigation', {
			name: 'Secrets pages',
		});
		await expect(secretsPager).toContainText(`Showing 1–20 of ${total}`);
		await expect(page.getByRole('row')).toHaveCount(20 + 1);
		await page.waitForFunction(() => document.getAnimations().length === 0);
		for (const theme of ['light', 'dark']) {
			await page.evaluate(
				(t) => document.documentElement.setAttribute('data-theme', t),
				theme,
			);
			await page.waitForFunction(() => document.getAnimations().length === 0);
			const pagerResults = await new AxeBuilder({ page })
				.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
				.analyze();
			expect(pagerResults.violations).toEqual([]);
		}

		await secretsPager.getByRole('link', { name: 'Next' }).click();
		await expect(page).toHaveURL(/[?&]page=2(&|$)/);
		await expect(secretsPager).toContainText(
			`Showing 21–${Math.min(40, total)} of ${total}`,
		);

		// Page 2 at desktop, and the collapsed "Page 2 of n" pager at 390px, are
		// scanned too: they are different markup from page 1.
		await page.evaluate(() =>
			document.documentElement.setAttribute('data-theme', 'light'),
		);
		await page.mouse.move(0, 0);
		for (const width of [1280, 390]) {
			await page.setViewportSize({ width, height: 900 });
			await page.waitForFunction(() => document.getAnimations().length === 0);
			const pageTwoResults = await new AxeBuilder({ page })
				.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
				.analyze();
			expect(pageTwoResults.violations).toEqual([]);
		}
		await page.setViewportSize({ width: 1280, height: 900 });

		// What a page past the end shows, and that a reload keeps the page, is
		// overview.svelte.spec.ts's.
		// Narrowing the list sends the reader back to page 1.
		await page.goto('/?page=2');
		// That loading ?page=2 keeps page 2, and that each narrowing asks to go back
		// to page 1, is overview.svelte.spec.ts's. What this proves is that the
		// address really changes.
		await expect(secretsPager).toContainText('Showing 21');
		await page
			.getByRole('searchbox', { name: 'Filter secrets' })
			.fill('paging_');
		await expect(page).not.toHaveURL(/page=/);
		await expect(page.getByRole('row')).toHaveCount(20 + 1);
		await expect(secretsPager).toContainText('Showing 1–20 of 21');
		await page.getByRole('searchbox', { name: 'Filter secrets' }).fill('');

		// So does picking a tag.
		await page.goto('/?page=2');
		await page
			.getByRole('group', { name: 'Filter by tag' })
			.getByRole('button', { name: 'paging (21)' })
			.click();
		await expect(page).not.toHaveURL(/page=/);
		await expect(secretsPager).toContainText('Showing 1–20 of 21');

		// alrayyes/Hush-Hush#897: the reader picks 20, 50 or 100 rows. The size is
		// the URL's, and a list that fits at the new size keeps the selector so the
		// choice can be undone.
		await page.goto('/');
		const secretsSize = secretsPager.getByRole('combobox', {
			name: 'Rows per page',
		});
		await secretsSize.click();
		const sizeMenuResults = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(sizeMenuResults.violations).toEqual([]);
		// And the open menu in the dark theme, once the colour transitions have
		// finished so axe reads the final colours.
		await page.evaluate(() =>
			document.documentElement.setAttribute('data-theme', 'dark'),
		);
		await page.waitForFunction(() => document.getAnimations().length === 0);
		const sizeMenuDarkResults = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(sizeMenuDarkResults.violations).toEqual([]);
		await page.evaluate(() =>
			document.documentElement.setAttribute('data-theme', 'light'),
		);
		await page.getByRole('option', { name: '50', exact: true }).click();
		await expect(page).toHaveURL(/[?&]size=50(&|$)/);
		await expect(page).not.toHaveURL(/page=/);
		// What size 50 shows, the selector staying when one page holds everything,
		// and picking 20 again, are overview.svelte.spec.ts's.
	} finally {
		// Leave the list as it was, for whatever runs next.
		const removed = await Promise.all(
			pagingSlugs.map((slug) =>
				page.request.delete(`/objects/${slug}`, { headers }),
			),
		);
		for (const response of removed) {
			// Already gone is fine: a seed that failed leaves nothing to remove.
			expect([200, 204, 404]).toContain(response.status());
		}
	}
});
