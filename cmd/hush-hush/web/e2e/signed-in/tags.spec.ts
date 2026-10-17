import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import * as age from 'age-encryption';

// alrayyes/hush-hush#522: the secrets list shows each secret's tags and filters
// by one. The tags on a card, the pills and their counts, the pressed state and
// a tag with the search box are overview.svelte.spec.ts's. What is left here is
// that the filter is wired to the real list at phone width, with 44px pills and
// an axe scan. The tags are this spec's own, so the other specs' secrets don't
// change the counts, and the three secrets are removed again.
test('a tag pill filters the real list, and the pills are 44px targets', async ({
	page,
	context,
}) => {
	const csrfToken =
		(await context.cookies()).find((c) => c.name === 'csrf_token')?.value ?? '';
	const headers = { 'X-CSRF-Token': csrfToken };
	const recipient = await age.identityToRecipient(await age.generateIdentity());
	const slugs = ['tagged_one', 'tagged_two', 'tagged_three'];

	try {
		for (const [slug, tags] of [
			['tagged_one', ['tagspec-prod', 'tagspec-homelab']],
			['tagged_two', ['tagspec-prod']],
			['tagged_three', ['tagspec-backup']],
		] as const) {
			const encrypter = new age.Encrypter();
			encrypter.addRecipient(recipient);
			const sealed = await encrypter.encrypt(`value of ${slug}`);
			const created = await page.request.post('/objects', {
				headers,
				data: { slug, value: btoa(String.fromCharCode(...sealed)), tags },
			});
			expect(created.ok()).toBe(true);
		}

		const listed = (await (
			await page.request.get('/objects?limit=500')
		).json()) as { id: string }[] | { objects?: { id: string }[] };
		const allSecrets = (Array.isArray(listed) ? listed : (listed.objects ?? []))
			.length;

		// The phone cards page at 20, like the table.
		const onOnePage = Math.min(allSecrets, 20);
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/');
		const cardList = page.getByRole('list', { name: 'Secrets' });
		await expect(cardList.getByRole('listitem')).toHaveCount(onOnePage);

		const tagFilter = page.getByRole('group', { name: 'Filter by tag' });
		for (const pill of await tagFilter.getByRole('button').all()) {
			const box = await pill.boundingBox();
			expect(box?.height).toBeGreaterThanOrEqual(44);
		}

		// One real click, to prove the filter is wired to the real list.
		await tagFilter.getByRole('button', { name: 'tagspec-prod (2)' }).click();
		await expect(cardList.getByRole('listitem')).toHaveCount(2);

		// The pills animate their colours when pressed, and axe samples a
		// mid-transition colour as a false contrast failure: let every CSS
		// transition finish before scanning.
		await page.waitForFunction(() => document.getAnimations().length === 0);
		const tagResults = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(tagResults.violations).toEqual([]);

		await tagFilter
			.getByRole('button', { name: `All (${allSecrets})` })
			.click();
		await expect(cardList.getByRole('listitem')).toHaveCount(onOnePage);
	} finally {
		const removed = await Promise.all(
			slugs.map((slug) => page.request.delete(`/objects/${slug}`, { headers })),
		);
		for (const response of removed) {
			// Already gone is fine: a seed that failed leaves nothing to remove.
			expect([200, 204, 404]).toContain(response.status());
		}
	}
});
