import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import * as age from 'age-encryption';

// alrayyes/hush-hush#670: one name can hold a value per consumer, each under its
// own id. What each dialog says and which id each button uses is
// overview.svelte.spec.ts's, the conflict message is the Go API tests', and the
// choice page is secret-detail.svelte.spec.ts's. This walks the real flow: edit
// one variant, view another, create a third through the dialog (and the server
// refusing a consumer that already has one), inspect one by its own page, delete
// them one at a time. The variants are removed again in a finally.
test('one name holds a value per consumer, and each is edited, created and deleted on its own', async ({
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

	try {
		// alrayyes/hush-hush#670: one name can hold a value per consumer, each
		// under its own id. Two are made through the API (the create dialog is
		// exercised for the third below); sealing here only needs a well-formed
		// age file, which any recipient gives.
		for (const consumer of ['consumer_a', 'consumer_d']) {
			const created = await page.request.post('/objects', {
				headers,
				data: {
					slug: 'release_token',
					value: await sealFor(`token for ${consumer}`),
					used_by: [consumer],
				},
			});
			expect(created.ok()).toBe(true);
		}

		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/');
		const variantRows = page
			.getByRole('row')
			.filter({ has: page.getByRole('cell', { name: 'release_token' }) });
		await expect(variantRows).toHaveCount(2);
		const variantA = variantRows.filter({ hasText: 'consumer_a' });
		const variantD = variantRows.filter({ hasText: 'consumer_d' });
		await expect(variantA).toHaveCount(1);
		await expect(variantD).toHaveCount(1);

		// Edit, View and Delete say which variant they act on, and use its id:
		// overview.svelte.spec.ts's. Here the real server takes the edit.
		await variantD.getByRole('button', { name: 'Edit' }).click();
		const variantEdit = page.getByRole('dialog', {
			name: 'Edit release_token',
		});
		await variantEdit.locator('#edit-value').fill('rotated for consumer_d');
		await variantEdit.getByLabel('Keep a readable copy for yourself').check();
		await variantEdit.getByRole('button', { name: 'Save' }).click();
		await variantEdit.waitFor({ state: 'hidden' });
		await expect(variantRows).toHaveCount(2);
		await expect(variantA).toContainText('consumer_a');
		await expect(variantD).toContainText('consumer_d');

		await variantA.getByRole('button', { name: 'View' }).click();
		const variantView = page.getByRole('dialog', { name: 'release_token' });
		await expect(variantView.getByLabel('Ciphertext (base64)')).not.toHaveValue(
			'',
		);
		await variantView.getByRole('button', { name: 'Close' }).click();

		// Creating a name that already exists adds a variant when no consumer of
		// it already has one, and says why when one does.
		const variantConsumersLoaded = page.waitForResponse(
			(res) =>
				new URL(res.url()).pathname === '/consumers' &&
				res.request().method() === 'GET',
		);
		await page.getByRole('button', { name: 'New secret' }).click();
		const variantCreate = page.getByRole('dialog', { name: 'Create a secret' });
		await expect(page.getByLabel('Id')).toBeFocused();
		await variantCreate.locator('#create-id').fill('release_token');
		await variantCreate
			.locator('#create-value')
			.fill('token for consumer_d too');
		await variantCreate.getByLabel('Keep a readable copy for yourself').check();
		await variantConsumersLoaded;
		await variantCreate.locator('#create-used-by').fill('consumer_d');
		await variantCreate
			.getByRole('option', { name: 'consumer_d', exact: true })
			.click();
		await variantCreate.getByRole('button', { name: 'Create' }).click();
		await expect(variantCreate.getByRole('alert')).toContainText(
			/consumer "consumer_d" is already in the secret named "release_token"/,
		);
		await variantCreate.getByLabel('Remove consumer_d').click();
		await variantCreate.locator('#create-used-by').fill('consumer_e');
		await variantCreate
			.getByRole('option', { name: 'Add "consumer_e"' })
			.click();
		await variantCreate.getByRole('button', { name: 'Create' }).click();
		await variantCreate.waitFor({ state: 'hidden' });
		await expect(variantRows).toHaveCount(3);

		// Inspect opens one variant's own page, with its own consumers.
		await variantD.getByRole('link', { name: 'Inspect' }).click();
		await page.waitForURL(/\/secrets\/release_token\?id=[0-9a-f-]{36}$/);
		const variantConsumers = page.getByRole('region', {
			name: 'Authorized consumers',
		});
		await expect(variantConsumers.getByRole('listitem')).toHaveCount(1);
		await expect(variantConsumers).toContainText('consumer_d');
		const variantDetailResults = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(variantDetailResults.violations).toEqual([]);

		// Without an id the name is ambiguous, so the page offers the choice.
		await page.goto('/secrets/release_token');
		await expect(
			page.getByRole('region', { name: 'Variants' }).getByRole('listitem'),
		).toHaveCount(3);
		await page.goto('/');

		// Phone width: each variant is its own card, told apart by consumer.
		await page.setViewportSize({ width: 390, height: 844 });
		const variantCards = page
			.getByRole('list', { name: 'Secrets' })
			.getByRole('listitem')
			.filter({ hasText: 'release_token' });
		await expect(variantCards).toHaveCount(3);
		await expect(variantCards.filter({ hasText: 'consumer_a' })).toHaveCount(1);
		await expect(variantCards.filter({ hasText: 'consumer_d' })).toHaveCount(1);
		await page.waitForFunction(() => document.getAnimations().length === 0);
		const variantCardResults = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(variantCardResults.violations).toEqual([]);

		// Delete removes the one variant, and leaves its siblings.
		await variantCards
			.filter({ hasText: 'consumer_d' })
			.getByRole('button', { name: 'Delete' })
			.click();
		const variantDelete = page.getByRole('alertdialog', {
			name: 'Delete release_token?',
		});
		await variantDelete
			.getByRole('button', { name: 'Delete', exact: true })
			.click();
		await expect(variantCards).toHaveCount(2);
		await expect(variantCards.filter({ hasText: 'consumer_d' })).toHaveCount(0);
		await expect(variantCards.filter({ hasText: 'consumer_a' })).toHaveCount(1);

		// Clear the rest so nothing after this sees them.
		for (const consumer of ['consumer_a', 'consumer_e']) {
			await variantCards
				.filter({ hasText: consumer })
				.getByRole('button', { name: 'Delete' })
				.click();
			await page
				.getByRole('alertdialog', { name: 'Delete release_token?' })
				.getByRole('button', { name: 'Delete', exact: true })
				.click();
		}
		await expect(variantCards).toHaveCount(0);
	} finally {
		// Whatever variants are left (a failed run stops partway through): every
		// secret named release_token, by id.
		const listed = (await (
			await page.request.get('/objects?limit=500')
		).json()) as
			| { objects?: { id: string; slug: string }[] }
			| { id: string; slug: string }[];
		const rows = Array.isArray(listed) ? listed : (listed.objects ?? []);
		for (const row of rows.filter((r) => r.slug === 'release_token')) {
			await page.request.delete(`/objects/release_token?id=${row.id}`, {
				headers,
			});
		}
	}
});
