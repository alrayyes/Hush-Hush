import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import * as age from 'age-encryption';
import { unwrapIdentityWithRecoveryPhrase } from '../src/lib/identity';

// alrayyes/hush-hush#272: /audit-log is both a SvelteKit page route and a
// real backend API endpoint, and Go's mux used to route a hard
// navigation there straight to the JSON handler instead of the SPA's
// static fallback. A real page.goto (not a client-side nav click, which
// never hits the server for this exact path either way) is the only way
// to exercise that server-side routing decision at all.
test('a hard navigation to /audit-log renders the app, not the raw API JSON', async ({
	page,
}) => {
	const response = await page.goto('/audit-log');

	expect(response?.headers()['content-type']).toContain('text/html');

	// No session yet, so the SPA's own client-side auth check redirects
	// to /login - unaffected by this fix, and proof the SPA actually
	// booted rather than the browser just displaying a JSON array as text
	// (which would never run any client JS to redirect anywhere).
	await page.waitForURL('/login');
	await expect(page).toHaveTitle('Log in - hush-hush');
	await expect(page.locator('nav')).toHaveCount(0);
});

// alrayyes/hush-hush#295: same collision as #272 above, on /consumers -
// it shipped after that fix (#279's paginated consumer directory)
// without the same treatment.
test('a hard navigation to /consumers renders the app, not the raw API JSON', async ({
	page,
}) => {
	const response = await page.goto('/consumers');

	expect(response?.headers()['content-type']).toContain('text/html');

	await page.waitForURL('/login');
	await expect(page).toHaveTitle('Log in - hush-hush');
	await expect(page.locator('nav')).toHaveCount(0);
});

// #281: the toggle lives in the shared root layout, so it has to work for
// an anonymous visitor too, not just once logged in - /login is the one
// page every visitor reaches with no session. resolveTheme's own cascade
// (stored > OS > light) is unit-tested in theme.spec.ts; this only checks
// the interactive part a unit test can't: a real click persisting past a
// real reload.
test('the theme toggle flips data-theme and persists across a reload for an anonymous visitor', async ({
	page,
}) => {
	await page.goto('/login');

	const toggle = page.getByRole('button', {
		name: /switch to (dark|light) mode/i,
	});
	await expect(toggle).toBeVisible();

	const initialTheme = await page.evaluate(() =>
		document.documentElement.getAttribute('data-theme'),
	);
	expect(['light', 'dark']).toContain(initialTheme);

	const flipped = initialTheme === 'dark' ? 'light' : 'dark';
	await toggle.click();
	await expect(page.locator('html')).toHaveAttribute('data-theme', flipped);
	await expect(toggle).toHaveAttribute(
		'aria-pressed',
		String(flipped === 'dark'),
	);

	await page.reload();
	await expect(page.locator('html')).toHaveAttribute('data-theme', flipped);
	await expect(toggle).toHaveAttribute(
		'aria-pressed',
		String(flipped === 'dark'),
	);
});

// web-ui-design-system/tasks.md #2.1/#2.2: the shared chrome's nav follows
// a session onto every page, not just the ones under (app)/, and never
// shows for an unauthenticated visitor - plus the a11y.md-mandated
// axe-core scan, folded into this same login-and-navigate journey rather
// than a parallel suite re-driving the same pages.
test('an authenticated visitor keeps the nav across pages, an anonymous one never sees it, and the app has no a11y violations', async ({
	page,
	context,
	browserName,
}) => {
	test.skip(
		browserName !== 'chromium',
		'CDP virtual authenticator is Chromium-only',
	);

	const client = await context.newCDPSession(page);
	await client.send('WebAuthn.enable');
	await client.send('WebAuthn.addVirtualAuthenticator', {
		options: {
			protocol: 'ctap2',
			transport: 'internal',
			hasResidentKey: true,
			hasUserVerification: true,
			isUserVerified: true,
		},
	});

	// Captured here so tasks.md's 5.2 e2e coverage below can recover the
	// same escrowed identity registration just generated, entirely through
	// its own public break-glass surface (the phrase this page displays,
	// and the recovery-wrapped copy the client already sends the server as
	// part of this exact request) - never by reaching into the page's own
	// in-memory state, which the app never exposes past this request
	// either.
	let registerFinishBody: { recovery_wrapped_identity?: string } | null = null;
	page.on('request', (request) => {
		if (
			request.method() === 'POST' &&
			new URL(request.url()).pathname === '/auth/register/finish'
		) {
			registerFinishBody = request.postDataJSON();
		}
	});

	await page.goto('/login');
	await page.getByRole('button', { name: 'Register passkey' }).click();

	// tasks.md group 3.2: a first-ever registration shows the escrowed
	// identity's break-glass recovery phrase exactly once, before the
	// login page navigates onward - closeRecoveryPhrase (login/+page.svelte)
	// is what actually navigates to "/", not the registration itself.
	const recoveryPhraseField = page.getByLabel('Recovery phrase', {
		exact: true,
	});
	await expect(recoveryPhraseField).not.toHaveValue('');
	const recoveryPhrase = await recoveryPhraseField.inputValue();

	await page
		.getByRole('button', { name: "I've saved it" })
		.click({ timeout: 10_000 });
	await page.waitForURL('/');

	// A real age keypair, generated in the test itself (not through the
	// app) - registering its public key against "homelab" here, before
	// that consumer is ever picked below, is what proves the value the
	// create dialog stores is genuinely sealed to it: this identity, and
	// only this identity, can decrypt it back
	// (openspec/changes/client-side-encryption/tasks.md's 4.3).
	const homelabIdentity = await age.generateIdentity();
	const homelabRecipient = await age.identityToRecipient(homelabIdentity);
	const csrfToken =
		(await context.cookies()).find((c) => c.name === 'csrf_token')?.value ?? '';
	await page.request.patch('/consumers/homelab', {
		headers: { 'X-CSRF-Token': csrfToken },
		data: { public_key: homelabRecipient },
	});

	const nav = page.getByRole('navigation', { name: 'Primary', exact: true });
	await expect(nav.getByRole('link', { name: 'Secrets' })).toBeVisible();

	await page.goto('/changelog');
	await expect(nav.getByRole('link', { name: 'Secrets' })).toBeVisible();

	// Scanned back on the secrets overview - the one authenticated page
	// with real interactive structure (a table, dialogs), not the mostly
	// static changelog.
	await page.goto('/');
	await expect(page.locator('main')).toBeVisible();
	const results = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(results.violations).toEqual([]);

	// #301: the current page's nav link carries aria-current="page" -
	// checked here and again after navigating to Consumers below, so
	// this proves it actually moves rather than sticking to whichever
	// link loaded first.
	await expect(nav.getByRole('link', { name: 'Secrets' })).toHaveAttribute(
		'aria-current',
		'page',
	);
	await expect(
		nav.getByRole('link', { name: 'Consumers' }),
	).not.toHaveAttribute('aria-current', 'page');

	// alrayyes/hush-hush#480: below md the top nav row gives way to a fixed
	// bottom tab bar. Both are real <nav> landmarks with their own
	// aria-label, so role queries (which only match what's displayed) tell
	// them apart. Folded into this journey, not its own spec - the server
	// accepts one passkey registration per database, so a second test
	// registering in parallel races this one.
	const tabBar = page.getByRole('navigation', { name: 'Primary (mobile)' });
	await expect(tabBar).toBeHidden();

	await page.setViewportSize({ width: 390, height: 844 });
	await expect(tabBar).toBeVisible();
	await expect(
		page.getByRole('navigation', { name: 'Primary', exact: true }),
	).toBeHidden();

	for (const name of ['Secrets', 'Consumers', 'Audit log', 'Settings']) {
		const link = tabBar.getByRole('link', { name });
		await expect(link).toBeVisible();

		// 44x44px is the minimum touch target.
		const box = await link.boundingBox();
		expect(box?.width).toBeGreaterThanOrEqual(44);
		expect(box?.height).toBeGreaterThanOrEqual(44);
	}

	// Fixed to the bottom edge, and never covering the footer.
	const barBox = await tabBar.boundingBox();
	expect((barBox?.y ?? 0) + (barBox?.height ?? 0)).toBeCloseTo(844, 0);
	await page.locator('footer').scrollIntoViewIfNeeded();
	const footerBox = await page.locator('footer').boundingBox();
	expect((footerBox?.y ?? 0) + (footerBox?.height ?? 0)).toBeLessThanOrEqual(
		barBox?.y ?? 0,
	);

	// aria-current moves with navigation, same as the top nav above.
	await expect(tabBar.getByRole('link', { name: 'Secrets' })).toHaveAttribute(
		'aria-current',
		'page',
	);
	await tabBar.getByRole('link', { name: 'Consumers' }).click();
	await page.waitForURL('/consumers');
	await expect(tabBar.getByRole('link', { name: 'Consumers' })).toHaveAttribute(
		'aria-current',
		'page',
	);
	await expect(
		tabBar.getByRole('link', { name: 'Secrets' }),
	).not.toHaveAttribute('aria-current', 'page');

	const mobileResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(mobileResults.violations).toEqual([]);

	await page.setViewportSize({ width: 1280, height: 720 });
	await page.goto('/');
	await expect(tabBar).toBeHidden();

	// A real secret with real width pressure - #271's own gap: the
	// public-pages-only viewport test never caught the authenticated
	// pages' tables scrolling sideways in their own box at 320px. The
	// same dialog also exercises the consumer combobox's listbox, open
	// and populated - the hand-written ARIA APG combobox
	// (alrayyes/hush-hush#251) is the newest, most complex interactive
	// widget added since the last scan.
	// The create dialog's ConsumerCombobox mounts fresh on this first open
	// and fires its own GET /consumers?... - the same race the edit dialog
	// below already guards against (see its own comment), so this is
	// awaited before typing into #create-used-by for the same reason: a
	// mid-fill resolution mutates the DOM and steals focus back.
	const createConsumersLoaded = page.waitForResponse(
		(res) =>
			new URL(res.url()).pathname === '/consumers' &&
			res.request().method() === 'GET',
	);
	await page.getByRole('button', { name: 'New secret' }).click();
	// bits-ui's Dialog autofocuses the first field (Id) on open, racing any
	// interaction with a later field started right away - wait for that
	// autofocus to settle before touching a later field, or its own focus
	// gets stolen back mid-fill.
	await expect(page.getByLabel('Id')).toBeFocused();
	// #386/#393: the dialog's old "plain text (base64)" and "paste
	// ciphertext" create modes are gone - one plaintext value field, sealed
	// client-side, is the only way in. Scoped to this dialog, not the whole
	// page - the (currently closed, but DOM-present) view dialog still
	// legitimately describes the stored value as "Sealed ciphertext".
	const createDialog = page.getByRole('dialog', { name: 'Create a secret' });
	await expect(createDialog.getByLabel('Value', { exact: true })).toBeVisible();
	await expect(createDialog.getByText(/plain text/i)).toHaveCount(0);
	await expect(createDialog.getByText(/ciphertext/i)).toHaveCount(0);
	const plaintextValue = 'prod deploy webhook secret, sealed client-side';
	await page.locator('#create-id').fill('mattermost_deploy_webhook');
	await page.locator('#create-value').fill(plaintextValue);
	await page
		.locator('#create-description')
		.fill('prod deploy webhook for homelab/vps-docker');
	await createConsumersLoaded;
	await page.locator('#create-used-by').fill('homelab');
	await page.getByRole('listbox').waitFor();
	const comboboxResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(comboboxResults.violations).toEqual([]);
	// "homelab" already has a registered public key (set above), so the
	// combobox offers it as an existing consumer, not an "Add" option -
	// specs/consumers/spec.md's "Picking a consumer with a registered
	// public key resolves a recipient" scenario.
	await page.getByRole('option', { name: 'homelab', exact: true }).click();
	await page.getByRole('button', { name: 'Create' }).click();
	await page.getByRole('button', { name: 'New secret' }).waitFor();

	// alrayyes/hush-hush#480: below md the secrets table gives way to one
	// card per secret, plus a search box that filters both layouts. Done
	// here, right after the first create, because this is the one place a
	// real secret exists and nothing has edited or deleted it yet.
	await context.grantPermissions(['clipboard-read', 'clipboard-write']);
	await page.setViewportSize({ width: 390, height: 844 });
	await expect(page.getByRole('table')).toBeHidden();

	const secretList = page.getByRole('list', { name: 'Secrets' });
	const secretCard = secretList
		.getByRole('listitem')
		.filter({ hasText: 'mattermost_deploy_webhook' });
	await expect(secretCard).toBeVisible();
	await expect(secretCard).toContainText(
		'prod deploy webhook for homelab/vps-docker',
	);
	await expect(secretCard).toContainText('age-encrypted (X25519)');
	await expect(secretCard).toContainText(/Updated .+ by admin/);

	// Every control on a card is at least 44x44px.
	const copySlug = secretCard.getByRole('button', {
		name: 'Copy slug mattermost_deploy_webhook',
	});
	for (const control of [
		copySlug,
		secretCard.getByRole('link', { name: 'Inspect' }),
		secretCard.getByRole('button', { name: 'Edit' }),
		secretCard.getByRole('button', { name: 'Delete' }),
	]) {
		const box = await control.boundingBox();
		expect(box?.width).toBeGreaterThanOrEqual(44);
		expect(box?.height).toBeGreaterThanOrEqual(44);
	}

	await copySlug.click();
	expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
		'mattermost_deploy_webhook',
	);
	await expect(
		secretCard.getByRole('button', {
			name: 'Copied mattermost_deploy_webhook',
		}),
	).toBeVisible();

	// Delete still goes through the confirmation dialog.
	await secretCard.getByRole('button', { name: 'Delete' }).click();
	await expect(
		page.getByRole('alertdialog', {
			name: 'Delete mattermost_deploy_webhook?',
		}),
	).toBeVisible();
	await page.getByRole('button', { name: 'Cancel' }).click();

	// Search matches slug or description, and says so when nothing matches.
	const search = page.getByRole('searchbox', { name: 'Filter secrets' });
	await search.fill('no-such-secret');
	await expect(secretCard).toBeHidden();
	await expect(page.getByText('No secrets match')).toBeVisible();
	await search.fill('vps-docker');
	await expect(secretCard).toBeVisible();
	await search.fill('');

	const cardResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(cardResults.violations).toEqual([]);

	// alrayyes/hush-hush#480: Inspect opens a detail page for the secret,
	// not the dialog the desktop table's View button still uses.
	await secretCard.getByRole('link', { name: 'Inspect' }).click();
	await page.waitForURL('/secrets/mattermost_deploy_webhook');
	await expect(
		page.getByRole('heading', { name: 'mattermost_deploy_webhook' }),
	).toBeVisible();

	// The sealed ciphertext, base64, with a working copy button.
	const sealed = page.getByLabel('Sealed ciphertext (base64)');
	await expect(sealed).not.toHaveValue('');
	const sealedValue = await sealed.inputValue();
	await page.getByRole('button', { name: 'Copy sealed ciphertext' }).click();
	expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
		sealedValue,
	);

	// The CLI snippet uses the CLI's real flags and a placeholder key, never
	// a real one (the design's `-i ~/.age/key.txt` doesn't exist).
	const cliSnippet = page.getByRole('group', { name: 'Fetch with the CLI' });
	await expect(cliSnippet).toContainText(
		'hush-hush-cli get mattermost_deploy_webhook --identity "AGE-SECRET-KEY-1..."',
	);
	await expect(cliSnippet).not.toContainText('-i ~/.age');

	// The consumer recorded at creation, with its registered public key
	// truncated, not shown whole.
	const consumersSection = page.getByRole('region', {
		name: 'Authorized consumers',
	});
	await expect(consumersSection.getByRole('listitem')).toHaveCount(1);
	await expect(consumersSection).toContainText('homelab');
	await expect(consumersSection).toContainText(
		`${homelabRecipient.slice(0, 10)}…${homelabRecipient.slice(-6)}`,
	);
	await expect(consumersSection).not.toContainText(homelabRecipient);

	// At most three recent audit events, newest first. The secret has been
	// created and nothing else yet, bar this page's own read.
	const activity = page.getByRole('region', { name: 'Recent activity' });
	const activityRows = activity.getByRole('listitem');
	expect(await activityRows.count()).toBeGreaterThanOrEqual(1);
	expect(await activityRows.count()).toBeLessThanOrEqual(3);
	await expect(activityRows.last()).toContainText('create');

	const detailResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(detailResults.violations).toEqual([]);

	// A hard navigation works too (the SPA fallback), and an unknown slug
	// says so instead of rendering an empty page.
	await page.goto('/secrets/does_not_exist');
	await expect(page.getByRole('alert')).toContainText(/not found/i);
	await page.goto('/');

	await page.setViewportSize({ width: 1280, height: 720 });
	await expect(page.getByRole('table')).toBeVisible();
	await expect(secretList).toBeHidden();

	// #299: viewing a secret shows its recorded consumers, and editing one
	// can change that list - both used to be create-only. View first,
	// against the "homelab" used_by set at creation above.
	await page.getByRole('button', { name: 'View' }).click();
	const viewDialog = page.getByRole('dialog', {
		name: 'mattermost_deploy_webhook',
	});
	await expect(
		viewDialog.getByRole('listitem').getByText('homelab'),
	).toBeVisible();

	// The stored value is genuine age ciphertext, decryptable only with the
	// matching consumer private key, not a lookalike or the plaintext just
	// typed above - reads the same base64 this View dialog already fetched
	// and displays, rather than a second request of its own (which would
	// double-count as an extra admin-attributed read below). The fetch
	// behind it (openView's own getObjectValue call) is async, so this
	// waits for the textarea to actually hold it rather than the still-
	// empty value from the instant the dialog opened.
	const ciphertextField = page.getByLabel('Ciphertext (base64)');
	await expect(ciphertextField).not.toHaveValue('');
	const viewedCiphertext = await ciphertextField.inputValue();
	const sealedBytes = Uint8Array.from(atob(viewedCiphertext), (c) =>
		c.charCodeAt(0),
	);
	const decrypter = new age.Decrypter();
	decrypter.addIdentity(homelabIdentity);
	const decrypted = await decrypter.decrypt(sealedBytes, 'text');
	expect(decrypted).toBe(plaintextValue);

	const viewResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(viewResults.violations).toEqual([]);
	await viewDialog.getByRole('button', { name: 'Close' }).click();

	// Edit: the combobox comes pre-filled with the existing "homelab"
	// consumer, and adding "ci" alongside it is what proves the update
	// actually reaches the server rather than only the local form state.
	// The combobox's own GET /consumers call has to be awaited before
	// typing into it, not just its "Remove homelab" chip becoming visible
	// - bits-ui's Dialog re-asserts its initial focus target on a DOM
	// mutation inside it, and that fetch resolving while the used-by
	// field is mid-fill is exactly that mutation, stealing focus back to
	// the ciphertext textarea and silently dropping the keystrokes.
	const editConsumersLoaded = page.waitForResponse(
		(res) =>
			new URL(res.url()).pathname === '/consumers' &&
			res.request().method() === 'GET',
	);
	await page.getByRole('button', { name: 'Edit' }).click();
	const editDialog = page.getByRole('dialog', {
		name: 'Edit mattermost_deploy_webhook',
	});
	await expect(editDialog.getByLabel('Remove homelab')).toBeVisible();
	await editConsumersLoaded;
	await editDialog.locator('#edit-used-by').fill('ci');
	await editDialog.getByRole('listbox').waitFor();
	await editDialog.getByRole('option', { name: 'Add "ci"' }).click();
	// "ci" has no registered public key - specs/consumers/spec.md's
	// "Picking a consumer with no registered public key resolves no
	// recipient" scenario: the form indicates this next to its chip rather
	// than silently sealing to fewer recipients than picked ("homelab",
	// already keyed above, still resolves one, so Save stays enabled).
	await expect(editDialog.getByText('(no key)')).toBeVisible();
	await editDialog.locator('#edit-value').fill(btoa('rotated'));
	const editResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(editResults.violations).toEqual([]);
	await editDialog.getByRole('button', { name: 'Save' }).click();
	await editDialog.waitFor({ state: 'hidden' });

	await page.getByRole('button', { name: 'View' }).click();
	await expect(
		viewDialog.getByRole('listitem').getByText('homelab'),
	).toBeVisible();
	await expect(viewDialog.getByRole('listitem').getByText('ci')).toBeVisible();
	await viewDialog.getByRole('button', { name: 'Close' }).click();

	// Drop "ci" again so the consumer directory below sees only the
	// single "homelab" consumer its own assertions expect.
	await page.getByRole('button', { name: 'Edit' }).click();
	await editDialog.getByLabel('Remove ci').click();
	await editDialog.locator('#edit-value').fill(btoa('rotated-again'));
	await editDialog.getByRole('button', { name: 'Save' }).click();
	await editDialog.waitFor({ state: 'hidden' });

	// The secret just created recorded "homelab" as a consumer - the
	// directory's own filtering and select-to-filter navigation
	// (alrayyes/hush-hush#252), plus its own axe-core scan.
	await nav.getByRole('link', { name: 'Consumers' }).click();
	await expect(page.getByRole('heading', { name: 'Consumers' })).toBeVisible();
	await expect(nav.getByRole('link', { name: 'Consumers' })).toHaveAttribute(
		'aria-current',
		'page',
	);
	await expect(nav.getByRole('link', { name: 'Secrets' })).not.toHaveAttribute(
		'aria-current',
		'page',
	);
	await expect(page.getByRole('link', { name: 'homelab' })).toBeVisible();
	const consumersResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(consumersResults.violations).toEqual([]);

	await page.getByLabel('Filter by name').fill('nomatch');
	await page.getByLabel('Filter by name').blur();
	await expect(page.getByText('No consumers match "nomatch".')).toBeVisible();

	await page.getByLabel('Filter by name').fill('homelab');
	await page.getByLabel('Filter by name').blur();
	await page.getByRole('link', { name: 'homelab' }).click();
	await page.waitForURL('/?used_by=homelab');
	await expect(page.getByText('consumer: homelab')).toBeVisible();
	await expect(
		page.getByRole('cell', { name: 'mattermost_deploy_webhook' }),
	).toBeVisible();
	await page.getByRole('link', { name: 'Clear consumer filter' }).click();
	await page.waitForURL('/');

	// #282: rename and delete are bulk used_by rewrites across every
	// object recording the consumer, not CRUD on a dedicated resource -
	// this drives both through the real UI, dialog and all, rather than
	// only the handler-level tests in internal/api/consumers_test.go.
	await nav.getByRole('link', { name: 'Consumers' }).click();
	await expect(page.getByRole('heading', { name: 'Consumers' })).toBeVisible();

	await page.getByRole('button', { name: 'Rename' }).click();
	const renameDialog = page.getByRole('dialog', { name: 'Rename consumer' });
	await expect(renameDialog).toBeVisible();
	const renameResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(renameResults.violations).toEqual([]);
	await renameDialog
		.getByLabel('Name', { exact: true })
		.fill('homelab-renamed');
	await renameDialog.getByRole('button', { name: 'Save' }).click();
	await expect(
		page.getByRole('link', { name: 'homelab-renamed' }),
	).toBeVisible();
	await expect(
		page.getByRole('link', { name: 'homelab', exact: true }),
	).toHaveCount(0);

	await page.getByRole('button', { name: 'Delete' }).click();
	const deleteDialog = page.getByRole('alertdialog', {
		name: 'Delete "homelab-renamed"?',
	});
	await expect(deleteDialog).toBeVisible();
	const deleteResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(deleteResults.violations).toEqual([]);
	await deleteDialog
		.getByRole('button', { name: 'Delete', exact: true })
		.click();
	await expect(page.getByText('No consumers match yet.')).toBeVisible();

	// #324: a consumer can be added directly, with no secret referencing
	// it yet - it should show up with a secret count of 0, and adding the
	// same name again should be rejected rather than silently duplicating.
	await page.getByRole('button', { name: 'Add consumer' }).click();
	const addDialog = page.getByRole('dialog', { name: 'Add a consumer' });
	await expect(addDialog).toBeVisible();
	const addResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(addResults.violations).toEqual([]);
	await addDialog.getByLabel('Name', { exact: true }).fill('homelab-new');
	await addDialog.getByRole('button', { name: 'Add' }).click();
	await expect(addDialog).toBeHidden();
	const addedRow = page
		.getByRole('row')
		.filter({ has: page.getByRole('link', { name: 'homelab-new' }) });
	await expect(addedRow).toBeVisible();
	await expect(addedRow.getByRole('cell').nth(1)).toHaveText('0');
	// #476: a consumer with no consumer tokens shows a plain "0", not a link.
	const addedTokensCell = addedRow.getByRole('cell').nth(2);
	await expect(addedTokensCell).toHaveText('0');
	await expect(addedTokensCell.getByRole('link')).toHaveCount(0);

	await page.getByRole('button', { name: 'Add consumer' }).click();
	await addDialog.getByLabel('Name', { exact: true }).fill('homelab-new');
	await addDialog.getByRole('button', { name: 'Add' }).click();
	await expect(page.getByRole('alert')).toHaveText('consumer already exists');
	await addDialog.getByRole('button', { name: 'Cancel' }).click();

	// #323: the object-id and actor filters are select boxes populated
	// from what actually appears in the log, not free text - this
	// exercises both against the real entries the flow above already
	// produced (a create + read + update for mattermost_deploy_webhook,
	// each carrying a real actor).
	await nav.getByRole('link', { name: 'Audit log' }).click();
	await expect(page.getByRole('heading', { name: 'Audit log' })).toBeVisible();
	const auditLogResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(auditLogResults.violations).toEqual([]);

	const rows = page.locator('tbody tr');

	// alrayyes/hush-hush#483: below md the log becomes one card per event,
	// with the action as a text-labelled pill, 44px export buttons and a
	// copyable curl command. Same events as the table, same order.
	const tableRowCount = await rows.count();
	await page.setViewportSize({ width: 390, height: 844 });
	await expect(page.getByRole('table')).toBeHidden();

	const eventList = page.getByRole('list', { name: 'Audit events' });
	const eventCards = eventList.getByRole('listitem');
	await expect(eventCards).toHaveCount(tableRowCount);
	await expect(eventCards.first()).toContainText('mattermost_deploy_webhook');
	await expect(eventCards.first()).toContainText('admin');
	await expect(eventCards.first().locator('time')).toBeVisible();

	// Colour is never the only signal: every action carries its own word.
	const actionPills = eventList.getByTestId('action-pill');
	for (const action of await actionPills.allTextContents()) {
		expect(action).toMatch(/^(create|read|update|delete)$/i);
	}
	await expect(actionPills.filter({ hasText: /create/i })).not.toHaveCount(0);
	await expect(actionPills.filter({ hasText: /read/i })).not.toHaveCount(0);
	await expect(actionPills.filter({ hasText: /update/i })).not.toHaveCount(0);

	for (const control of [
		page.getByRole('button', { name: 'Export CSV' }),
		page.getByRole('button', { name: 'Export JSON' }),
		page.getByRole('button', { name: 'Copy command' }),
	]) {
		const box = await control.boundingBox();
		expect(box?.width).toBeGreaterThanOrEqual(44);
		expect(box?.height).toBeGreaterThanOrEqual(44);
	}

	const curlSnippet = page.getByRole('group', { name: 'Query with curl' });
	await expect(curlSnippet).toContainText('curl');
	await expect(curlSnippet).toContainText('/audit-log');
	await page.getByRole('button', { name: 'Copy command' }).click();
	expect(await page.evaluate(() => navigator.clipboard.readText())).toContain(
		'/audit-log',
	);

	const auditCardResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(auditCardResults.violations).toEqual([]);

	await page.setViewportSize({ width: 1280, height: 720 });
	await expect(page.getByRole('table')).toBeVisible();
	await expect(eventList).toBeHidden();
	const actorTrigger = page.getByRole('button', { name: 'Actor', exact: true });
	const objectTrigger = page.getByRole('button', {
		name: 'Object id',
		exact: true,
	});

	await actorTrigger.click();
	// #438/#446: GET /objects/{slug} now requires a credential, so a
	// logged-in "View" authenticates as the session's own admin account
	// instead of recording no verified actor - the dropdown no longer
	// offers a "none" option at all (this used to be the "unverified
	// actor" case this test covered, alrayyes/hush-hush#451).
	await expect(
		page.getByRole('option', { name: 'none', exact: true }),
	).toHaveCount(0);
	await page.getByRole('option', { name: 'admin', exact: true }).click();
	await expect(page.getByText('actor: admin')).toBeVisible();
	// Every row so far (the create, both "View" reads, the detail page's
	// own read of the ciphertext (#480), both edits) is
	// now attributed to admin - waiting on the row count itself (not
	// just the chip, which updates synchronously before the refetch
	// resolves) avoids asserting against the table's still-unfiltered
	// content.
	await expect(rows).toHaveCount(6);
	for (const row of await rows.all()) {
		await expect(row.getByRole('cell').nth(2)).toHaveText('admin');
	}

	await actorTrigger.click();
	await page.getByRole('option', { name: 'Any actor' }).click();
	await expect(page.getByText('actor: admin')).toHaveCount(0);
	await expect(rows).toHaveCount(6);

	await objectTrigger.click();
	await page.getByRole('option', { name: 'mattermost_deploy_webhook' }).click();
	await expect(
		page.getByText('object: mattermost_deploy_webhook'),
	).toBeVisible();
	await expect(rows).toHaveCount(6);
	for (const row of await rows.all()) {
		await expect(row.getByRole('cell').first()).toHaveText(
			'mattermost_deploy_webhook',
		);
	}

	await page
		.locator('li')
		.filter({ hasText: 'object: mattermost_deploy_webhook' })
		.getByRole('button')
		.click();
	await expect(page.getByText('object: mattermost_deploy_webhook')).toHaveCount(
		0,
	);

	// tasks.md's 5.2: the owner-recipient opt-in checkbox. Recovering the
	// escrowed identity generated at registration above, through its own
	// public break-glass surface (the recovery phrase and the
	// recovery-wrapped copy captured then) - never through anything the
	// app exposes to a decrypt UI, since it has none yet.
	if (!registerFinishBody?.recovery_wrapped_identity) {
		throw new Error(
			'registration never sent a recovery-wrapped identity to unwrap',
		);
	}
	const ownerIdentity = await unwrapIdentityWithRecoveryPhrase(
		registerFinishBody.recovery_wrapped_identity,
		recoveryPhrase,
	);

	await nav.getByRole('link', { name: 'Secrets' }).click();
	await expect(page.getByRole('button', { name: 'New secret' })).toBeVisible();

	await page.getByRole('button', { name: 'New secret' }).click();
	await expect(page.getByLabel('Id')).toBeFocused();
	const ownerOptInValue = 'grafana admin console password';
	await page.locator('#create-id').fill('grafana_admin_password');
	await page.locator('#create-value').fill(ownerOptInValue);
	// No consumer picked - the checkbox alone is what makes this
	// decryptable at all (specs/secret-objects/spec.md's "Opt-in
	// owner-recipient inclusion at create time" requirement, tested here
	// in isolation from any consumer recipient).
	await page.getByLabel('Keep a readable copy for yourself').click();
	await page.getByRole('button', { name: 'Create' }).click();
	await page.getByRole('button', { name: 'New secret' }).waitFor();

	const grafanaRow = page.getByRole('row', { name: /grafana_admin_password/ });
	await grafanaRow.getByRole('button', { name: 'View' }).click();
	const grafanaViewDialog = page.getByRole('dialog', {
		name: 'grafana_admin_password',
	});
	const grafanaCiphertextField = grafanaViewDialog.getByLabel(
		'Ciphertext (base64)',
	);
	await expect(grafanaCiphertextField).not.toHaveValue('');
	const grafanaCiphertext = await grafanaCiphertextField.inputValue();
	const grafanaSealedBytes = Uint8Array.from(atob(grafanaCiphertext), (c) =>
		c.charCodeAt(0),
	);

	const ownerDecrypter = new age.Decrypter();
	ownerDecrypter.addIdentity(ownerIdentity);
	const grafanaDecrypted = await ownerDecrypter.decrypt(
		grafanaSealedBytes,
		'text',
	);
	expect(grafanaDecrypted).toBe(ownerOptInValue);
	await grafanaViewDialog.getByRole('button', { name: 'Close' }).click();

	// The owner never opted into mattermost_deploy_webhook - the same
	// escrowed identity must not be able to decrypt it
	// (specs/secret-objects/spec.md's "Owner recipient is not the
	// default" scenario), fetched fresh rather than reusing the value
	// captured before its own edits above.
	const mattermostRow = page.getByRole('row', {
		name: /mattermost_deploy_webhook/,
	});
	await mattermostRow.getByRole('button', { name: 'View' }).click();
	await expect(ciphertextField).not.toHaveValue('');
	const currentMattermostCiphertext = await ciphertextField.inputValue();
	const currentMattermostBytes = Uint8Array.from(
		atob(currentMattermostCiphertext),
		(c) => c.charCodeAt(0),
	);
	const nonOptInDecrypter = new age.Decrypter();
	nonOptInDecrypter.addIdentity(ownerIdentity);
	await expect(
		nonOptInDecrypter.decrypt(currentMattermostBytes, 'text'),
	).rejects.toThrow();
	await viewDialog.getByRole('button', { name: 'Close' }).click();

	// Settings: passkey and bearer-token management, and the only
	// authenticated page area rules/a11y.md's "every page a journey test
	// covers gets its own scan" didn't already reach above - the viewport
	// loop below visits it too, but only for the horizontal-scroll check,
	// never for axe.
	await nav.getByRole('link', { name: 'Settings' }).click();
	await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
	const settingsResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(settingsResults.violations).toEqual([]);

	// alrayyes/hush-hush#484: the offline recovery notice (ADR 21) shows at
	// every width, and below md each passkey is a card with Rename and
	// Delete as 44px targets.
	const recoveryNotice = page.getByRole('complementary', {
		name: 'Recovery phrase',
	});
	await expect(recoveryNotice).toContainText('shown once');
	await expect(recoveryNotice).toContainText('keeps no copy');

	await page.setViewportSize({ width: 390, height: 844 });
	await expect(recoveryNotice).toBeVisible();
	const passkeyCard = page
		.getByRole('list', { name: 'Passkey list' })
		.getByRole('listitem')
		.first();
	await expect(passkeyCard).toBeVisible();
	await expect(passkeyCard).toContainText('Added');
	await expect(passkeyCard).toContainText('Last used');
	for (const control of [
		passkeyCard.getByRole('button', { name: 'Rename' }),
		passkeyCard.getByRole('button', { name: 'Delete' }),
	]) {
		const box = await control.boundingBox();
		expect(box?.width).toBeGreaterThanOrEqual(44);
		expect(box?.height).toBeGreaterThanOrEqual(44);
	}
	const passkeyCardResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(passkeyCardResults.violations).toEqual([]);
	await page.setViewportSize({ width: 1280, height: 720 });

	// Consumer tokens: create/rotate/revoke, and the single-select
	// consumer picker (max=1, showKeyStatus=false -
	// openspec/changes/consumer-read-tokens-web-ui/design.md). "ci-runner"
	// is deliberately a consumer with no registered public key - proving
	// showKeyStatus=false actually suppresses the "(no key)" hint
	// ConsumerCombobox's used_by callers still show.
	await page.getByRole('button', { name: 'New consumer token' }).click();
	const createConsumerTokenDialog = page.getByRole('dialog', {
		name: 'Create a consumer token',
	});
	await page.locator('#consumer-token-consumer').fill('ci-runner');
	await page.getByRole('listbox').waitFor();
	await expect(createConsumerTokenDialog.getByText('(no key)')).toHaveCount(0);
	await page.getByRole('option', { name: 'Add "ci-runner"' }).click();
	await expect(createConsumerTokenDialog.getByLabel('Consumer')).toHaveCount(0);
	await page
		.locator('#consumer-token-description')
		.fill('deploy read token for ci-runner');
	await page.locator('#consumer-token-ttl').fill('30');
	// The Create button's disabled state clears (consumer is picked) but
	// Tailwind's disabled:opacity-50 briefly outlives the DOM's own
	// `disabled` property by a tick after the field fills above - a real
	// user's own typing pace never lands inside that window, but a
	// synchronous axe snapshot right after `.fill()` can, and reports a
	// false color-contrast violation against the fading-out low-opacity
	// state. Confirmed by direct getComputedStyle() polling: opacity
	// reads 0.5 immediately after fill, 1 within 1s, with `disabled`
	// already `false` at both points.
	await expect(
		createConsumerTokenDialog.getByRole('button', { name: 'Create' }),
	).toHaveCSS('opacity', '1');
	const createConsumerTokenResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(createConsumerTokenResults.violations).toEqual([]);
	await createConsumerTokenDialog
		.getByRole('button', { name: 'Create' })
		.click();
	const consumerTokenValueField = page.getByLabel('Token value');
	await expect(consumerTokenValueField).not.toHaveValue('');
	const firstConsumerTokenValue = await consumerTokenValueField.inputValue();
	await page
		.getByRole('dialog', { name: 'Token created' })
		.getByRole('button', { name: 'Done' })
		.click();

	const consumerTokenRow = page.getByRole('row', { name: /ci-runner/ });
	await expect(consumerTokenRow.getByRole('cell').nth(5)).toHaveText('Active');

	// alrayyes/hush-hush#484: below md each token is a card with a
	// time-left badge and Rotate / Revoke as separate 44px targets. The
	// consumer token above was created with a 30-day TTL.
	await page.setViewportSize({ width: 390, height: 844 });
	await expect(consumerTokenRow).toHaveCount(0);
	const consumerTokenCard = page
		.getByRole('list', { name: 'Consumer token list' })
		.getByRole('listitem')
		.filter({ hasText: 'ci-runner' });
	await expect(consumerTokenCard).toBeVisible();
	await expect(consumerTokenCard).toContainText('Active');
	await expect(consumerTokenCard.getByTestId('ttl-badge')).toHaveText(
		/^(29|30)d left$/,
	);
	const rotateBox = await consumerTokenCard
		.getByRole('button', { name: 'Rotate' })
		.boundingBox();
	const revokeBox = await consumerTokenCard
		.getByRole('button', { name: 'Revoke' })
		.boundingBox();
	for (const box of [rotateBox, revokeBox]) {
		expect(box?.width).toBeGreaterThanOrEqual(44);
		expect(box?.height).toBeGreaterThanOrEqual(44);
	}
	// Isolated: a clear gap between the safe and the destructive action, so
	// a thumb aimed at one can't land on the other.
	const horizontalGap =
		(revokeBox?.x ?? 0) - ((rotateBox?.x ?? 0) + (rotateBox?.width ?? 0));
	const verticalGap =
		(revokeBox?.y ?? 0) - ((rotateBox?.y ?? 0) + (rotateBox?.height ?? 0));
	expect(Math.max(horizontalGap, verticalGap)).toBeGreaterThanOrEqual(8);
	const settingsCardResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(settingsCardResults.violations).toEqual([]);
	await page.setViewportSize({ width: 1280, height: 720 });

	await consumerTokenRow.getByRole('button', { name: 'Rotate' }).click();
	const rotateConsumerTokenDialog = page.getByRole('dialog', {
		name: 'Rotate this token?',
	});
	await rotateConsumerTokenDialog
		.getByRole('button', { name: 'Rotate', exact: true })
		.click();
	await expect(consumerTokenValueField).not.toHaveValue('');
	const rotatedConsumerTokenValue = await consumerTokenValueField.inputValue();
	expect(rotatedConsumerTokenValue).not.toBe(firstConsumerTokenValue);
	await page
		.getByRole('dialog', { name: 'Token rotated' })
		.getByRole('button', { name: 'Done' })
		.click();

	await consumerTokenRow.getByRole('button', { name: 'Revoke' }).click();
	await page
		.getByRole('alertdialog', { name: 'Revoke this token?' })
		.getByRole('button', { name: 'Revoke', exact: true })
		.click();
	await expect(consumerTokenRow.getByRole('cell').nth(5)).toHaveText('Revoked');
	await expect(
		consumerTokenRow.getByRole('button', { name: 'Rotate' }),
	).toHaveCount(0);
	await expect(
		consumerTokenRow.getByRole('button', { name: 'Revoke' }),
	).toHaveCount(0);

	// Consumers directory: Tokens count column, linking to a filtered
	// Settings view (alrayyes/hush-hush#476). "homelab-new" (added above at
	// #324, still in the directory with 0 secrets) is used here rather
	// than "ci-runner" - #476's Tokens column only has a count to show for
	// a consumer that already has a directory row, and issuing a token
	// doesn't create one on its own (design.md's Risks/Trade-offs note).
	// "ci-runner"'s own now-revoked token stays in place through this
	// block, so the filtered view below has a real second row to exclude.
	await page.getByRole('button', { name: 'New consumer token' }).click();
	const homelabTokenDialog = page.getByRole('dialog', {
		name: 'Create a consumer token',
	});
	await page.locator('#consumer-token-consumer').fill('homelab-new');
	await page.getByRole('listbox').waitFor();
	await page.getByRole('option', { name: 'homelab-new', exact: true }).click();
	await homelabTokenDialog
		.locator('#consumer-token-description')
		.fill('directory link check');
	await homelabTokenDialog.locator('#consumer-token-ttl').fill('30');
	await homelabTokenDialog.getByRole('button', { name: 'Create' }).click();
	await page
		.getByRole('dialog', { name: 'Token created' })
		.getByRole('button', { name: 'Done' })
		.click();

	await nav.getByRole('link', { name: 'Consumers' }).click();
	await expect(page.getByRole('heading', { name: 'Consumers' })).toBeVisible();
	const homelabNewRow = page
		.getByRole('row')
		.filter({ has: page.getByRole('link', { name: 'homelab-new' }) });
	const homelabNewTokensCell = homelabNewRow.getByRole('cell').nth(2);
	await expect(homelabNewTokensCell.getByRole('link')).toHaveText('1');
	const consumersTokensColumnResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(consumersTokensColumnResults.violations).toEqual([]);

	await homelabNewTokensCell.getByRole('link').click();
	await page.waitForURL('/settings?consumer=homelab-new');
	await expect(page.getByText('consumer: homelab-new')).toBeVisible();
	await expect(page.getByRole('row', { name: /homelab-new/ })).toBeVisible();
	await expect(page.getByRole('row', { name: /ci-runner/ })).toHaveCount(0);
	const filteredSettingsResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(filteredSettingsResults.violations).toEqual([]);

	await page.getByRole('button', { name: 'Remove consumer filter' }).click();
	await page.waitForURL('/settings');
	await expect(page.getByText('consumer: homelab-new')).toHaveCount(0);
	await expect(page.getByRole('row', { name: /ci-runner/ })).toBeVisible();

	// Purge (alrayyes/hush-hush#441): a dead token (revoked or expired)
	// offers "Delete permanently" instead of Rotate/Revoke, and confirming
	// removes it immediately. consumerTokenRow ("ci-runner") is already
	// revoked from the block above - reused here rather than creating a
	// fourth token just to purge it.
	await expect(
		consumerTokenRow.getByRole('button', { name: 'Delete permanently' }),
	).toBeVisible();
	await consumerTokenRow
		.getByRole('button', { name: 'Delete permanently' })
		.click();
	const purgeConsumerTokenDialog = page.getByRole('alertdialog', {
		name: 'Delete this token permanently?',
	});
	await expect(purgeConsumerTokenDialog).toContainText(
		'Audit-log entries referencing it will show as unresolvable',
	);
	const purgeResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(purgeResults.violations).toEqual([]);
	await purgeConsumerTokenDialog
		.getByRole('button', { name: 'Delete permanently' })
		.click();
	await expect(consumerTokenRow).toHaveCount(0);

	// The bearer-token side of the same gating, exercising the "Expired"
	// path specifically (unrevoked, past its own expiry) - the case the
	// old two-state `revoked ? 'Revoked' : 'Active'` Status column got
	// wrong (design.md's own "latent bug" note). The create dialog's own
	// TTL field is day-granularity (tokenTTLDays), so a 1-second-TTL
	// token is seeded directly through the API instead, the same way
	// this test already seeds a consumer's public key above.
	await page.request.post('/tokens', {
		headers: { 'X-CSRF-Token': csrfToken },
		data: { description: 'short-lived token', ttl_seconds: 1 },
	});
	await page.waitForTimeout(1_500);
	await page.reload();
	await page.getByRole('heading', { name: 'Settings' }).waitFor();

	const shortLivedRow = page.getByRole('row', { name: /short-lived token/ });
	await expect(shortLivedRow.getByRole('cell').nth(5)).toHaveText('Expired');

	// #484: an expired token's card says so, and offers only the permanent
	// delete, not Rotate or Revoke.
	await page.setViewportSize({ width: 390, height: 844 });
	const shortLivedCard = page
		.getByRole('list', { name: 'Bearer token list' })
		.getByRole('listitem')
		.filter({ hasText: 'short-lived token' });
	await expect(shortLivedCard.getByTestId('ttl-badge')).toHaveText('expired');
	await expect(shortLivedCard).toContainText('Expired');
	await expect(
		shortLivedCard.getByRole('button', { name: 'Rotate' }),
	).toHaveCount(0);
	await expect(
		shortLivedCard.getByRole('button', { name: 'Revoke' }),
	).toHaveCount(0);
	const purgeBox = await shortLivedCard
		.getByRole('button', { name: 'Delete permanently' })
		.boundingBox();
	expect(purgeBox?.width).toBeGreaterThanOrEqual(44);
	expect(purgeBox?.height).toBeGreaterThanOrEqual(44);
	await page.setViewportSize({ width: 1280, height: 720 });
	await expect(
		shortLivedRow.getByRole('button', { name: 'Rotate' }),
	).toHaveCount(0);
	await expect(
		shortLivedRow.getByRole('button', { name: 'Revoke' }),
	).toHaveCount(0);
	await shortLivedRow
		.getByRole('button', { name: 'Delete permanently' })
		.click();
	await page
		.getByRole('alertdialog', { name: 'Delete this token permanently?' })
		.getByRole('button', { name: 'Delete permanently' })
		.click();
	await expect(shortLivedRow).toHaveCount(0);

	await page.setViewportSize({ width: 320, height: 720 });

	// #294: the topbar's nav links, theme toggle, and Log out button used
	// to each wrap onto their own line at phone width via `flex-wrap`
	// fighting `margin-left: auto` - four-plus ragged rows before any page
	// content was visible. Clustering every topbar control's own top
	// offset (within a tolerance wider than the few px a link and a
	// padded button can differ by even centered on the same line) catches
	// that without pinning an exact pixel height to font metrics. The nav
	// links themselves moved to the bottom tab bar below md (#480), so what's
	// left in the topbar to cluster is the two account controls.
	const topbarControls = [
		page.getByRole('button', { name: 'Log out' }),
		page.getByRole('button', { name: /switch to (dark|light) mode/i }),
	];
	const boxes = await Promise.all(
		topbarControls.map((control) => control.boundingBox()),
	);
	const tops: number[] = [];
	for (const box of boxes) {
		expect(
			box,
			'every topbar control should be visible at 320px',
		).not.toBeNull();
		if (box !== null) {
			tops.push(box.y);
		}
	}
	tops.sort((a, b) => a - b);
	const rowTolerancePx = 16;
	let topbarRows = tops.length > 0 ? 1 : 0;
	for (let i = 1; i < tops.length; i++) {
		if (tops[i] - tops[i - 1] > rowTolerancePx) {
			topbarRows++;
		}
	}
	expect(
		topbarRows,
		'topbar controls should read as at most two rows at 320px',
	).toBeLessThanOrEqual(2);

	// Client-side nav clicks, not page.goto() - a hard navigation to
	// /audit-log is its own dedicated test below (#272), and this loop is
	// about the mobile layout, not routing. Via the bottom tab bar: at 320px
	// it's the only nav displayed.
	for (const linkName of ['Secrets', 'Consumers', 'Audit log', 'Settings']) {
		await tabBar.getByRole('link', { name: linkName }).click();
		await expect(page.locator('main')).toBeVisible();

		// Checks every scrollable element on the page, not just the
		// document - a table with overflow-x: auto never overflows the
		// document (it scrolls sideways within its own box instead),
		// which is exactly what let this regression ship unnoticed the
		// first time. Scoped to overflow-x: scroll/auto specifically -
		// overflow: hidden (the visually-hidden <thead> pattern) also
		// reports a scrollWidth/clientWidth mismatch by design, with no
		// visible scrollbar to go with it.
		const widest = await page.evaluate(() =>
			Math.max(
				0,
				...Array.from(document.querySelectorAll('*'))
					.filter((el) =>
						['scroll', 'auto'].includes(getComputedStyle(el).overflowX),
					)
					.map((el) => el.scrollWidth - el.clientWidth),
			),
		);
		expect(
			widest,
			`${linkName} has a horizontally scrollable element at 320px`,
		).toBe(0);
	}
	await page.setViewportSize({ width: 1280, height: 800 });

	// Log out lives in .topbar-actions, not <nav> - it's an account
	// action, not a navigation link (#294).
	await page.getByRole('button', { name: 'Log out' }).click();
	await page.waitForURL('/login');

	await page.goto('/changelog');
	await expect(page.locator('nav')).toHaveCount(0);
});
