import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import * as age from 'age-encryption';

// A variant can have a hundred consumers, and a secret can have the longest slug,
// ten of the longest tags and a long description (alrayyes/hush-hush#752, #894,
// #905). These are layout checks, so they need a browser: the row is as tall as
// one with three consumers, the table fits the page at both widths, the full list
// opens in a dialog that scrolls on its own, a hovered Delete stays readable in
// both themes, and the consumers directory pages at 20. What the list says and
// filters is ConsumerList.svelte.spec.ts's, overview.svelte.spec.ts's and
// secret-detail.svelte.spec.ts's. The four secrets it makes are removed again.
test('a hundred consumers and the longest values fit the Secrets and Consumers pages', async ({
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
	const widestSlug = `wide_${'x'.repeat(123)}`;
	const slugs = [
		'many_consumers',
		'few_consumers',
		'some_consumers',
		widestSlug,
	];

	try {
		// A variant can have a hundred consumers. Its row is as tall as one with
		// three, the table fits the page at both widths, and the full list opens
		// in a dialog that filters and scrolls on its own.
		const manyConsumers = Array.from(
			{ length: 100 },
			(_, i) =>
				`team/${i % 34 === 0 ? 'dotfiles-' : 'svcfiles-'}${String(i).padStart(3, '0')}`,
		);
		for (const [slug, usedBy] of [
			['many_consumers', manyConsumers],
			['few_consumers', manyConsumers.slice(0, 3)],
			['some_consumers', manyConsumers.slice(0, 12)],
		] as const) {
			const created = await page.request.post('/objects', {
				headers,
				data: {
					slug,
					value: await sealFor(slug),
					used_by: usedBy,
				},
			});
			expect(created.ok()).toBe(true);
		}
		const rowFor = (slug: string) =>
			page.getByRole('row').filter({
				has: page.getByRole('cell', { name: new RegExp(`^${slug}`) }),
			});
		// 1280px is where the xl: truncation widths start and every wider width is
		// the same layout with more room, so a row that fits here fits there: no
		// 1920px pass.
		await page.setViewportSize({ width: 1280, height: 900 });
		await page.goto('/');
		await expect(rowFor('many_consumers')).toHaveCount(1);
		const manyBox = await rowFor('many_consumers').boundingBox();
		const fewBox = await rowFor('few_consumers').boundingBox();
		expect(manyBox?.height).toBeLessThanOrEqual((fewBox?.height ?? 0) + 4);
		expect(
			await page.evaluate(
				() =>
					document.documentElement.scrollWidth <=
					document.documentElement.clientWidth,
			),
		).toBe(true);
		await expect(
			rowFor('many_consumers').getByRole('button', { name: 'Delete' }),
		).toBeInViewport();

		// alrayyes/Hush-Hush#894: the consumers directory pages at 20 with the
		// shared pager. The 100 consumers seeded above are five pages and more.
		await page.setViewportSize({ width: 1280, height: 900 });
		await page.goto('/consumers');
		const consumerPages = page.getByRole('navigation', {
			name: 'Consumers pages',
		});
		// The directory holds whatever the other specs left, plus these hundred.
		const directoryTotal = (
			(await (
				await page.request.get('/consumers?page=1&page_size=1')
			).json()) as {
				total: number;
			}
		).total;
		expect(directoryTotal).toBeGreaterThanOrEqual(100);
		await expect(
			consumerPages.getByText(`Showing 1–20 of ${directoryTotal}`),
		).toBeVisible();
		// One header row and one row per consumer on the page.
		await expect(page.getByRole('row')).toHaveCount(21);
		const consumerPagesAxe = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(consumerPagesAxe.violations).toEqual([]);

		await consumerPages.getByRole('link', { name: 'Page 2' }).click();
		await expect(page).toHaveURL(/\/consumers\?page=2$/);

		// That the page links carry the search, a page past the end lands on the
		// last page (the load function's 307) and Enter in the filter goes to page
		// 1 are consumers.svelte.spec.ts's and load.spec.ts's; the q filter and
		// the totals are the Go consumers tests'.

		// alrayyes/hush-hush#752: a secret with the longest slug, ten of the
		// longest tags and a long description still fits the page at desktop
		// widths. Long values truncate rather than push the table wider.
		const widest = await page.request.post('/objects', {
			headers,
			data: {
				slug: widestSlug,
				value: await sealFor(widestSlug),
				used_by: manyConsumers.slice(0, 3),
				description: 'a description that keeps going '.repeat(8).trim(),
				tags: Array.from(
					{ length: 10 },
					(_, i) => `${String(i)}${'t'.repeat(31)}`,
				),
			},
		});
		expect(widest.ok()).toBe(true);
		// The tightest width of each regime. The table shows from 768px (md:), the
		// truncation widths grow at 1280px (xl:), and within a regime a wider
		// viewport only adds room, so a fit at 768 and at 1280 is a fit at 900,
		// 1024 and 1440. No width in between has a breakpoint of its own; a new
		// one (an md-to-xl utility, a media query) needs its width added back here.
		for (const width of [768, 1280]) {
			await page.setViewportSize({ width, height: 900 });
			await page.goto('/');
			await expect(rowFor(widestSlug)).toHaveCount(1);
			expect(
				await page.evaluate(
					() =>
						document.documentElement.scrollWidth <=
						document.documentElement.clientWidth,
				),
				`no horizontal scroll at ${String(width)}px`,
			).toBe(true);
			const widestDelete = rowFor(widestSlug).getByRole('button', {
				name: 'Delete',
			});
			await widestDelete.scrollIntoViewIfNeeded();
			await expect(widestDelete).toBeInViewport({ ratio: 1 });
		}

		// alrayyes/Hush-Hush#905: a hovered Delete button stays readable in both
		// themes. The scans above never hover, which is how the dark hover colour
		// slipped under 4.5:1.
		for (const theme of ['dark', 'light']) {
			await page.evaluate(
				(t) => document.documentElement.setAttribute('data-theme', t),
				theme,
			);
			await rowFor(widestSlug).getByRole('button', { name: 'Delete' }).hover();
			// The button's colours transition (transition-all): axe would sample
			// a mid-fade colour and report a contrast failure that isn't real.
			await page.waitForFunction(() => document.getAnimations().length === 0);
			const hoverResults = await new AxeBuilder({ page })
				.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
				.analyze();
			expect(hoverResults.violations, `hovered Delete in ${theme}`).toEqual([]);
		}

		// The confirm dialog's Delete sits on the popover surface, a step lighter
		// than the page, where the same tint has less contrast.
		await page
			.getByRole('row')
			.nth(1)
			.getByRole('button', { name: 'Delete' })
			.click();
		const confirmDelete = page
			.getByRole('alertdialog')
			.getByRole('button', { name: 'Delete' });
		for (const theme of ['dark', 'light']) {
			await page.evaluate(
				(t) => document.documentElement.setAttribute('data-theme', t),
				theme,
			);
			await confirmDelete.hover();
			await page.waitForFunction(() => document.getAnimations().length === 0);
			const dialogHoverResults = await new AxeBuilder({ page })
				.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
				.analyze();
			expect(
				dialogHoverResults.violations,
				`hovered dialog Delete in ${theme}`,
			).toEqual([]);
		}
		await page.getByRole('button', { name: 'Cancel' }).click();
		await page.mouse.move(0, 0);

		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/');
		await rowFor('many_consumers')
			.getByRole('button', { name: 'Show all 100 consumers of many_consumers' })
			.click();
		const consumersDialog = page.getByRole('dialog', {
			name: 'Consumers of many_consumers',
		});
		// The dialog's count, filter, empty message and links are
		// ConsumerList.svelte.spec.ts's and overview.svelte.spec.ts's.
		const dialogList = consumersDialog.getByRole('list');
		expect(
			await dialogList.evaluate((el) => el.scrollHeight > el.clientHeight),
		).toBe(true);
		const manyDialogResults = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(manyDialogResults.violations).toEqual([]);
		await consumersDialog
			.getByRole('button', { name: 'Close' })
			.first()
			.click();
		await expect(consumersDialog).toBeHidden();

		// The detail page filters and scrolls too.
		await page.goto('/secrets/many_consumers');
		const detailConsumers = page.getByRole('region', {
			name: 'Authorized consumers',
		});
		await expect(detailConsumers.getByRole('status')).toHaveText(
			'100 of 100 shown',
		);
		// Its filter is secret-detail.svelte.spec.ts's.

		// Phone width: the card names two consumers and offers the same button
		// as a 44px target, and the card doesn't grow with the list.
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/');
		const manyCard = page
			.getByRole('list', { name: 'Secrets' })
			.getByRole('listitem')
			.filter({ hasText: 'many_consumers' });
		const someCard = page
			.getByRole('list', { name: 'Secrets' })
			.getByRole('listitem')
			.filter({ hasText: 'some_consumers' });
		const manyCardBox = await manyCard.boundingBox();
		const someCardBox = await someCard.boundingBox();
		expect(manyCardBox?.height).toBeLessThanOrEqual(
			(someCardBox?.height ?? 0) + 8,
		);
		const moreButton = manyCard.getByRole('button', {
			name: 'Show all 100 consumers of many_consumers',
		});
		const moreBox = await moreButton.boundingBox();
		expect(moreBox?.height).toBeGreaterThanOrEqual(44);
		await page.waitForFunction(() => document.getAnimations().length === 0);
		const manyCardResults = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(manyCardResults.violations).toEqual([]);
	} finally {
		// Leave the list as it was, for whatever runs next.
		const removed = await Promise.all(
			slugs.map((slug) => page.request.delete(`/objects/${slug}`, { headers })),
		);
		for (const response of removed) {
			// Already gone is fine: a seed that failed leaves nothing to remove.
			expect([200, 204, 404]).toContain(response.status());
		}
	}
});
