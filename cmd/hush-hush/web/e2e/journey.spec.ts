import AxeBuilder from '@axe-core/playwright';
import { expect, type Locator, test } from '@playwright/test';
import * as age from 'age-encryption';
import { unwrapIdentityWithRecoveryPhrase } from '../src/lib/identity';
import { expectListParity, expectNavParity } from './layout-parity';

// A timestamp as formatTimestamp writes it, for the parity checks.
const TIMESTAMP = /[A-Z][a-z]{2} \d{1,2}, \d{4}/;

// alrayyes/hush-hush#661: a secret's plaintext is typed or pasted into these
// fields before it is sealed, so they opt out of everything a browser might
// send elsewhere - Chrome and Edge's enhanced spell check uploads field text.
async function expectValueFieldIsPrivate(field: Locator) {
	await expect(field).toHaveAttribute('spellcheck', 'false');
	await expect(field).toHaveAttribute('autocomplete', 'off');
	await expect(field).toHaveAttribute('autocapitalize', 'off');
	await expect(field).toHaveAttribute('autocorrect', 'off');
}

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
	// One journey registers a passkey and walks every page, and each web-ui
	// change adds to it, so it has outgrown Playwright's 30s default (#524).
	// A timeout there fails at whichever step the clock runs out on, which
	// looks like a click that can't land, not like a timeout. Set here, not
	// suite-wide, so a hung short test still fails fast.
	test.setTimeout(120_000);

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
			// The escrowed key is wrapped with the passkey's PRF output, which
			// the edit dialog's re-seal-without-the-value path reads back.
			hasPrf: true,
		},
	});

	// Captured here so tasks.md's 5.2 e2e coverage below can recover the
	// same escrowed identity registration just generated, entirely through
	// its own public break-glass surface (the phrase this page displays,
	// and the recovery-wrapped copy the client already sends the server as
	// part of this exact request) - never by reaching into the page's own
	// in-memory state, which the app never exposes past this request
	// either.
	// Assigned in the request handler below, so declare it un-narrowed: a bare
	// `= null` makes TypeScript treat every later read as `null`.
	let registerFinishBody = null as {
		recovery_wrapped_identity?: string;
	} | null;
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
	// #817: the phrase, shown once, has a Copy button that copies it exactly.
	await context.grantPermissions(['clipboard-read', 'clipboard-write']);
	await page.getByRole('button', { name: 'Copy recovery phrase' }).click();
	await expect(
		page.getByRole('button', { name: 'Copied recovery phrase' }),
	).toBeVisible();
	expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
		recoveryPhrase,
	);

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

	// Which nav link is the current page (#301), in the top nav and the phone
	// tab bar, and what the top bar says (#481), are layout.svelte.spec.ts's.

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

	// The footer's repository link is a 44px target on a phone, and the footer
	// doesn't scroll the page sideways.
	await page.locator('footer').scrollIntoViewIfNeeded();
	const phoneSource = await page
		.getByRole('contentinfo')
		.getByRole('link', { name: /^GitHub/ })
		.boundingBox();
	expect(phoneSource?.height).toBeGreaterThanOrEqual(44);
	expect(
		await page.evaluate(
			() =>
				document.documentElement.scrollWidth <=
				document.documentElement.clientWidth,
		),
	).toBe(true);

	// Fixed to the bottom edge, and never covering the footer.
	const barBox = await tabBar.boundingBox();
	expect((barBox?.y ?? 0) + (barBox?.height ?? 0)).toBeCloseTo(844, 0);
	await page.locator('footer').scrollIntoViewIfNeeded();
	const footerBox = await page.locator('footer').boundingBox();
	expect((footerBox?.y ?? 0) + (footerBox?.height ?? 0)).toBeLessThanOrEqual(
		barBox?.y ?? 0,
	);

	await tabBar.getByRole('link', { name: 'Consumers' }).click();
	await page.waitForURL('/consumers');

	// alrayyes/hush-hush#481: the phone top bar carries the brand, the
	// instance label from /healthz, an ACTIVE pill, and an account menu that
	// holds Log out and the theme toggle (the design drops both).
	const topBar = page.getByRole('banner');
	await expect(topBar.getByRole('button', { name: 'Log out' })).toHaveCount(0);

	const accountMenu = topBar.getByRole('button', { name: 'Account menu' });
	const accountBox = await accountMenu.boundingBox();
	expect(accountBox?.width).toBeGreaterThanOrEqual(44);
	expect(accountBox?.height).toBeGreaterThanOrEqual(44);
	await accountMenu.click();
	const accountDialog = page.getByRole('dialog', { name: 'Account' });
	await expect(accountDialog).toBeVisible();
	const accountResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(accountResults.violations).toEqual([]);
	await page.keyboard.press('Escape');
	await expect(accountDialog).toBeHidden();

	const mobileResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(mobileResults.violations).toEqual([]);

	await page.setViewportSize({ width: 1280, height: 720 });
	await page.goto('/');
	await expect(tabBar).toBeHidden();
	// At desktop width Log out sits in the bar itself, and the account menu
	// button isn't offered.
	await expect(topBar.getByText('e2e / local')).toBeVisible();
	await expect(topBar.getByRole('button', { name: 'Log out' })).toBeVisible();
	await expect(
		topBar.getByRole('button', { name: 'Account menu' }),
	).toBeHidden();

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
	// #865: a dialog taller than the viewport scrolls, and its close button
	// stays put instead of scrolling away with the content.
	const createViewport = page.viewportSize();
	await page.setViewportSize({ width: 1280, height: 400 });
	const createClose = createDialog.getByRole('button', { name: 'Close' });
	await expect(createClose).toBeInViewport({ ratio: 1 });
	const createBox = await createDialog.boundingBox();
	await page.mouse.move(
		(createBox?.x ?? 0) + (createBox?.width ?? 0) / 2,
		(createBox?.y ?? 0) + (createBox?.height ?? 0) / 2,
	);
	await page.mouse.wheel(0, 2000);
	await expect(
		createDialog.getByRole('button', { name: 'Create' }),
	).toBeInViewport();
	await expect(createClose).toBeInViewport({ ratio: 1 });
	await createDialog.evaluate((dialog) => {
		dialog.scrollTop = 0;
	});
	if (createViewport) await page.setViewportSize(createViewport);
	await expect(createDialog.getByText(/plain text/i)).toHaveCount(0);
	await expect(createDialog.getByText(/ciphertext/i)).toHaveCount(0);
	const plaintextValue = 'prod deploy webhook secret, sealed client-side';
	await expectValueFieldIsPrivate(page.locator('#create-value'));
	await page.locator('#create-id').fill('mattermost_deploy_webhook');
	// #804: stray whitespace at either end is trimmed before sealing, the
	// whitespace inside the value stays.
	await page.locator('#create-value').fill(`  \n${plaintextValue}\t \n`);
	await page
		.locator('#create-description')
		.fill('prod deploy webhook for homelab/vps-docker');
	// alrayyes/hush-hush#523: uppercase and a repeat are accepted and
	// normalised, so what comes back is the lowercase set once.
	await page.locator('#create-tags').fill('Prod, homelab, prod');
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
	// That a tag the API would refuse and a value of only whitespace stop the
	// submit is overview.svelte.spec.ts's.
	await page.locator('#create-value').fill(`  \n${plaintextValue}\t \n`);
	await page.getByRole('button', { name: 'Create' }).click();
	await page.getByRole('button', { name: 'New secret' }).waitFor();

	// The row has to exist before a second page reads it: the Create click
	// above returns before the request does.
	await expect(
		page.getByRole('cell', { name: /^mattermost_deploy_webhook/ }),
	).toBeVisible();

	// alrayyes/hush-hush#577: the card list and the table offer the same
	// actions and show the same facts.
	await expectListParity(page, {
		path: '/',
		list: 'Secrets',
		ids: ['mattermost_deploy_webhook'],
		facts: {
			description: { pattern: /prod deploy webhook for homelab\/vps-docker/ },
			'created by and updated by': { pattern: /admin/, times: 2 },
			'created and updated': { pattern: TIMESTAMP, times: 2 },
			tags: { pattern: /homelab/ },
		},
	});
	await expectNavParity(page);

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
	// The card's description and encryption badge, and the search box's
	// matching by id or description, are overview.svelte.spec.ts's.
	// The API returns tags sorted, whatever order they were entered in.
	await expect(secretCard.getByTestId('tag')).toHaveText(['homelab', 'prod']);
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

	// Inspect is a link (to the detail page) styled as a button: it must not
	// pick up the global anchor underline, or it reads differently from Edit
	// and Delete beside it (#519).
	await expect(secretCard.getByRole('link', { name: 'Inspect' })).toHaveCSS(
		'text-decoration-line',
		'none',
	);
	// ...nor the link colour: an outline button's text colour has to match
	// whether it's an <a> or a <button>.
	const textColor = (locator: import('@playwright/test').Locator) =>
		locator.evaluate((el) => getComputedStyle(el).color);
	expect(
		await textColor(secretCard.getByRole('link', { name: 'Inspect' })),
	).toBe(await textColor(secretCard.getByRole('button', { name: 'Edit' })));

	await copySlug.click();
	expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
		'mattermost_deploy_webhook',
	);
	await expect(
		secretCard.getByRole('button', {
			name: 'Copied mattermost_deploy_webhook',
		}),
	).toBeVisible();

	const cardResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(cardResults.violations).toEqual([]);

	// alrayyes/hush-hush#480: Inspect opens a detail page for the secret,
	// not the dialog the desktop table's View button still uses.
	await secretCard.getByRole('link', { name: 'Inspect' }).click();
	await page.waitForURL(
		/\/secrets\/mattermost_deploy_webhook\?id=[0-9a-f-]{36}$/,
	);
	await expect(
		page.getByRole('heading', { name: 'mattermost_deploy_webhook' }),
	).toBeVisible();

	// The sealed ciphertext, base64, fetched for real.
	const sealed = page.getByLabel('Sealed ciphertext (base64)');
	await expect(sealed).not.toHaveValue('');
	// The copy button, the CLI snippet and that a key is never shown whole
	// are secret-detail.svelte.spec.ts's.
	// The consumer recorded at creation, joined with its registered public
	// key from the directory, truncated.
	const consumersSection = page.getByRole('region', {
		name: 'Authorized consumers',
	});
	await expect(consumersSection.getByRole('listitem')).toHaveCount(1);
	await expect(consumersSection).toContainText('homelab');
	await expect(consumersSection).toContainText(
		`${homelabRecipient.slice(0, 10)}…${homelabRecipient.slice(-6)}`,
	);

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
	// That "ci", with no registered key, is marked beside its chip, and that
	// the dialog opens with the secret's tags and consumers, are
	// overview.svelte.spec.ts's.
	// #847: a dialog taller than the viewport stays inside it and scrolls, so
	// Save is never pushed off-screen (the re-seal error adds a paragraph).
	const originalViewport = page.viewportSize();
	await page.setViewportSize({ width: 1280, height: 400 });
	await expect
		.poll(async () => {
			const box = await editDialog.boundingBox();
			return box ? [box.y >= 0, box.y + box.height <= 400] : null;
		})
		.toEqual([true, true]);
	// Header and footer stay pinned, only the body scrolls: Save and the title
	// are on screen without scrolling anything.
	await expect(editDialog.getByRole('button', { name: 'Save' })).toBeInViewport(
		{ ratio: 1 },
	);
	await expect(
		editDialog.getByRole('heading', { name: 'Edit mattermost_deploy_webhook' }),
	).toBeInViewport({ ratio: 1 });
	// The body is what scrolls: a field below the fold is off screen until the
	// body is scrolled, then reachable.
	const usedBy = editDialog.locator('#edit-used-by');
	const editBody = editDialog.getByTestId('edit-dialog-body');
	await expect(usedBy).not.toBeInViewport();
	// A wheel over the body, as a user would, not a scripted scrollTop: that
	// still works under overflow-hidden and would hide a lost scrollbar.
	const bodyBox = await editBody.boundingBox();
	await page.mouse.move(
		(bodyBox?.x ?? 0) + (bodyBox?.width ?? 0) / 2,
		(bodyBox?.y ?? 0) + (bodyBox?.height ?? 0) / 2,
	);
	await page.mouse.wheel(0, 2000);
	await expect(usedBy).toBeInViewport();
	await editBody.evaluate((body) => {
		body.scrollTop = 0;
	});
	if (originalViewport) await page.setViewportSize(originalViewport);
	await expectValueFieldIsPrivate(editDialog.locator('#edit-value'));
	await editDialog.locator('#edit-value').fill(btoa('rotated'));
	await editDialog.locator('#edit-tags').fill('prod');
	const editResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(editResults.violations).toEqual([]);
	await editDialog.getByRole('button', { name: 'Save' }).click();
	await editDialog.waitFor({ state: 'hidden' });
	await expect(page.locator('[data-testid="tag"]:visible')).toHaveText([
		'prod',
	]);

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
	// An edit that leaves the Tags field alone leaves the tags alone.
	await expect(page.locator('[data-testid="tag"]:visible')).toHaveText([
		'prod',
	]);

	// Emptying the field clears them - and leaves this secret untagged for
	// the tag-filter journey below, which counts tags across the list. The
	// dialog's own GET /consumers is awaited first, as for the first edit
	// above: it resolving mid-fill makes the dialog re-grab focus and drop
	// keystrokes.
	const clearConsumersLoaded = page.waitForResponse(
		(res) =>
			new URL(res.url()).pathname === '/consumers' &&
			res.request().method() === 'GET',
	);
	await page.getByRole('button', { name: 'Edit' }).click();
	await clearConsumersLoaded;
	await editDialog.locator('#edit-value').fill(btoa('rotated-final'));
	await editDialog.locator('#edit-tags').fill('');
	await editDialog.getByRole('button', { name: 'Save' }).click();
	await editDialog.waitFor({ state: 'hidden' });
	await expect(page.locator('[data-testid="tag"]:visible')).toHaveCount(0);

	// #784, #843: a description or tags change with no value typed is a PATCH of
	// just that, with no re-seal and no PUT: overview.svelte.spec.ts's. Here the
	// real server takes each one.
	await page.getByRole('button', { name: 'Edit' }).click();
	await expect(editDialog.getByLabel('Description')).toBeVisible();
	await editDialog
		.getByLabel('Description')
		.fill('rotated by the on-call, see runbook');
	await editDialog.getByRole('button', { name: 'Save' }).click();
	await editDialog.waitFor({ state: 'hidden' });
	await expect(
		page
			.getByText('rotated by the on-call, see runbook')
			.locator('visible=true'),
	).toBeVisible();

	// #843: tags change with no value typed, as a PATCH of just the tags,
	// so no passkey touch and no PUT either.
	await page.getByRole('button', { name: 'Edit' }).click();
	await editDialog.locator('#edit-tags').fill('ops, rotated');
	await editDialog.getByRole('button', { name: 'Save' }).click();
	await editDialog.waitFor({ state: 'hidden' });
	await expect(page.locator('[data-testid="tag"]:visible')).toHaveText([
		'ops',
		'rotated',
	]);

	// A rename the server refuses shows its message and changes nothing.
	await page.getByRole('button', { name: 'Edit' }).click();
	await editDialog.getByLabel('Id', { exact: true }).fill('Not A Slug');
	await editDialog.getByRole('button', { name: 'Save' }).click();
	await expect(editDialog.getByRole('alert')).toBeVisible();
	const editRenameResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(editRenameResults.violations).toEqual([]);

	// A valid rename moves the row; rename back so later steps find it.
	await editDialog
		.getByLabel('Id', { exact: true })
		.fill('mattermost_deploy_hook');
	await editDialog.getByRole('button', { name: 'Save' }).click();
	await editDialog.waitFor({ state: 'hidden' });
	await expect(
		page.getByRole('cell', { name: /^mattermost_deploy_hook/ }),
	).toBeVisible();
	await expect(
		page.getByRole('cell', { name: /^mattermost_deploy_webhook/ }),
	).toHaveCount(0);
	await page.getByRole('button', { name: 'Edit' }).click();
	const renamedDialog = page.getByRole('dialog', {
		name: 'Edit mattermost_deploy_hook',
	});
	await renamedDialog
		.getByLabel('Id', { exact: true })
		.fill('mattermost_deploy_webhook');
	await renamedDialog.locator('#edit-tags').fill('');
	await renamedDialog.getByRole('button', { name: 'Save' }).click();
	await renamedDialog.waitFor({ state: 'hidden' });
	await expect(page.locator('[data-testid="tag"]:visible')).toHaveCount(0);

	// #790: a path no route matches shows the 404 page inside the signed-in
	// shell, and a load that fails shows the 500 page with no internals.
	await page.goto('/no/such/page');
	// What the page says besides its heading, and where it links, is
	// error.svelte.spec.ts's; here it has to be the page an unmatched path
	// reaches, inside the signed-in shell.
	await expect(
		page.getByRole('heading', { level: 1, name: 'Page not found' }),
	).toBeVisible();
	await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible();
	// The signed-in variant follows the root layout's session, which the
	// component test stubs.
	await expect(page.getByRole('link', { name: 'Open secrets' })).toBeVisible();
	const notFoundResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(notFoundResults.violations).toEqual([]);

	await page.route('**/objects?*', (route) => route.abort());
	await page.goto('/');
	await expect(
		page.getByRole('heading', { level: 1, name: 'Something went wrong' }),
	).toBeVisible();
	await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible();
	expect(await page.content()).not.toMatch(
		/\bat \S+\.(js|ts|svelte)|node_modules/,
	);
	const failedResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(failedResults.violations).toEqual([]);
	await page.unroute('**/objects?*');
	await page.goto('/');
	await expect(page.getByRole('button', { name: 'New secret' })).toBeVisible();

	// The secret just created recorded "homelab" as a consumer - the
	// directory's own filtering and select-to-filter navigation
	// (alrayyes/hush-hush#252), plus its own axe-core scan.
	await nav.getByRole('link', { name: 'Consumers' }).click();
	await expect(page.getByRole('heading', { name: 'Consumers' })).toBeVisible();
	await expect(page.getByRole('link', { name: 'homelab' })).toBeVisible();
	// That one consumer fits on a page with no pager is consumers.svelte.spec.ts's.
	const consumersResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(consumersResults.violations).toEqual([]);
	await expectListParity(page, {
		path: '/consumers',
		list: 'Consumers',
		ids: ['homelab'],
		facts: {
			'secret count': { pattern: /\b1\b/ },
			'public key': { pattern: /age1[a-z0-9]{4}/ },
		},
	});

	// alrayyes/hush-hush#482: below md the directory table becomes one
	// card per consumer - name, secrets and tokens counts, the registered
	// public key truncated with a copy button, and the real actions.
	await page.setViewportSize({ width: 390, height: 844 });
	await expect(page.getByRole('table')).toBeHidden();

	const consumerList = page.getByRole('list', { name: 'Consumers' });
	const consumerCard = consumerList
		.getByRole('listitem')
		.filter({ hasText: 'homelab' });
	await expect(consumerCard).toBeVisible();
	// The card's link, count and truncated key, and Copy writing the whole
	// key, are consumers.svelte.spec.ts's.
	const copyKey = consumerCard.getByRole('button', {
		name: 'Copy public key homelab',
	});
	for (const control of [
		copyKey,
		consumerCard.getByRole('button', { name: 'Rename' }),
		consumerCard.getByRole('button', { name: 'Delete' }),
	]) {
		const box = await control.boundingBox();
		expect(box?.width).toBeGreaterThanOrEqual(44);
		expect(box?.height).toBeGreaterThanOrEqual(44);
	}

	const consumerCardResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(consumerCardResults.violations).toEqual([]);

	await page.setViewportSize({ width: 1280, height: 720 });
	await expect(page.getByRole('table')).toBeVisible();
	await expect(consumerList).toBeHidden();

	await page.getByLabel('Filter by name').fill('homelab');
	await page.getByLabel('Filter by name').blur();
	await page.getByRole('link', { name: 'homelab' }).click();
	await page.waitForURL('/?used_by=homelab');
	await expect(page.getByText('Filters (active)')).toBeVisible();
	await expect(
		page.getByRole('cell', { name: /^mattermost_deploy_webhook/ }),
	).toBeVisible();
	await page.getByRole('button', { name: 'Clear filters' }).click();
	await page.waitForURL('/');

	// #816: several consumers at once (any of them), who created a secret,
	// and created/updated since a day. All live in the URL.
	await page.goto('/?used_by=homelab&used_by=no-such-consumer');
	await expect(
		page.getByRole('cell', { name: /^mattermost_deploy_webhook/ }),
	).toBeVisible();
	await page.goto('/?created_by=admin&created_since=2000-01-01');
	await expect(
		page.getByRole('cell', { name: /^mattermost_deploy_webhook/ }),
	).toBeVisible();
	await page.goto('/?created_by=token:nobody');
	await expect(page.getByText('No secrets match these filters.')).toBeVisible();
	await page.getByRole('button', { name: 'Clear filters' }).first().click();
	await page.waitForURL('/');
	await expect(
		page.getByRole('cell', { name: /^mattermost_deploy_webhook/ }),
	).toBeVisible();

	// The form writes the same parameters, and survives a reload.
	await page.getByText('Filters', { exact: true }).click();
	await page.getByLabel('Updated since').fill('2999-01-01');
	await page.getByRole('button', { name: 'Apply filters' }).click();
	await page.waitForURL('/?updated_since=2999-01-01');
	await expect(page.getByText('No secrets match these filters.')).toBeVisible();
	const filtersResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(filtersResults.violations).toEqual([]);
	await page.reload();
	await expect(page.getByLabel('Updated since')).toHaveValue('2999-01-01');
	await page.getByRole('button', { name: 'Clear filters' }).first().click();
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

	// #324: a consumer can be added directly, with no secret referencing it
	// yet, and shows up in the directory.
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
	// The 0 count and plain 0 for a consumer with no tokens (#476), the phone
	// card's 'not registered' with no Copy (#482), and the server's 'consumer
	// already exists' for a duplicate (#324) are consumers.svelte.spec.ts's,
	// with the count and message pinned in consumers_test.go.

	// #780: a consumer's public key is registered from the UI. The server
	// validates it, so a bad key shows the server's own message and
	// changes nothing; a valid one shows up truncated, with Copy.
	const keyed = await age.identityToRecipient(await age.generateIdentity());
	const keyedTruncated = `${keyed.slice(0, 10)}…${keyed.slice(-6)}`;
	const keyedRow = page
		.getByRole('row')
		.filter({ has: page.getByRole('link', { name: 'keyed-consumer' }) });
	await page.getByRole('button', { name: 'Add consumer' }).click();
	await addDialog.getByLabel('Name', { exact: true }).fill('keyed-consumer');
	await addDialog.getByLabel('Public key (optional)').fill(keyed);
	await addDialog.getByRole('button', { name: 'Add' }).click();
	await expect(addDialog).toBeHidden();
	await expect(keyedRow).toContainText(keyedTruncated);

	const noKeyRow = page
		.getByRole('row')
		.filter({ has: page.getByRole('link', { name: 'homelab-new' }) });
	await noKeyRow
		.getByRole('button', { name: 'Set key for homelab-new' })
		.click();
	const keyDialog = page.getByRole('dialog', { name: 'Set public key' });
	await expect(keyDialog).toBeVisible();
	// A key the server refuses shows the server's message and is never echoed
	// (#780), in consumers.svelte.spec.ts and consumers_test.go. The open
	// dialog is scanned once here.
	const keyDialogResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(keyDialogResults.violations).toEqual([]);
	await keyDialog.getByLabel('Public key', { exact: true }).fill(keyed);
	await keyDialog.getByRole('button', { name: 'Save' }).click();
	await expect(keyDialog).toBeHidden();
	await expect(noKeyRow).toContainText(keyedTruncated);

	// Put the directory back the way later steps expect it.
	await keyedRow.getByRole('button', { name: 'Delete' }).click();
	await page
		.getByRole('alertdialog')
		.getByRole('button', { name: 'Delete', exact: true })
		.click();
	await expect(keyedRow).toHaveCount(0);

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
	await expectListParity(page, {
		path: '/audit-log',
		list: 'Audit events',
		ids: [],
		facts: {
			object: { pattern: /mattermost_deploy_webhook/ },
			actor: { pattern: /admin/ },
			'caller address': { pattern: /127\.0\.0\.1/ },
			time: { pattern: TIMESTAMP },
		},
	});

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
	// What a card says, the action as a word and the curl command are
	// audit-log.svelte.spec.ts's. The real log has to give the same count.

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
	const auditCardResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(auditCardResults.violations).toEqual([]);

	await page.setViewportSize({ width: 1280, height: 720 });
	await expect(page.getByRole('table')).toBeVisible();
	await expect(eventList).toBeHidden();

	// alrayyes/hush-hush#753: the log opens newest first, the Timestamp header
	// flips it, and paging onward uses the cursor that matches the order.
	const timestampHeader = page.getByRole('columnheader', { name: 'Timestamp' });
	const orderButton = timestampHeader.getByRole('button', {
		name: /Timestamp/,
	});
	const rowTexts = async () =>
		(await page.locator('tbody tr').allTextContents()).map((text) =>
			text.replace(/\s+/g, ' ').trim(),
		);
	await expect(timestampHeader).toHaveAttribute('aria-sort', 'descending');
	await expect(curlSnippet).toContainText('order=desc');
	const newestFirst = await rowTexts();
	expect(newestFirst.length).toBeGreaterThan(2);

	// That a log which fits a page has no pager, and which cursor Newer and
	// Older send in either order, are audit-log.svelte.spec.ts's.
	await orderButton.click();
	await expect(timestampHeader).toHaveAttribute('aria-sort', 'ascending');
	await expect(curlSnippet).toContainText('order=asc');
	await expect.poll(rowTexts).toEqual([...newestFirst].reverse());
	await orderButton.click();
	await expect(timestampHeader).toHaveAttribute('aria-sort', 'descending');
	await expect.poll(rowTexts).toEqual(newestFirst);
	const orderResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(orderResults.violations).toEqual([]);

	// That the newer of two order changes wins when the older fetch arrives
	// last (#922), and that a failed change puts the controls back and keeps
	// the table (#921), are audit-log.svelte.spec.ts's.
	const actorTrigger = page.getByRole('combobox', {
		name: 'Actor',
		exact: true,
	});

	// The menus' options (including no `none` while every entry is attributed,
	// #438/#446, which is a store test), the chips, and what is fetched for an
	// actor or an object are audit-log.svelte.spec.ts's, and the server's
	// filtering is the Go audit tests'.
	await actorTrigger.click();
	// alrayyes/Hush-Hush#917: the open menu is its own state, and no scan had
	// it open. The trigger is a combobox that controls the list, and the list
	// is named by the label.
	await expect(page.getByRole('listbox', { name: 'Actor' })).toBeVisible();
	await page.waitForFunction(() => document.getAnimations().length === 0);
	const openSelectResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(openSelectResults.violations).toEqual([]);
	// One real pick, to prove the menu is wired to the real page.
	await page.getByRole('option', { name: 'admin', exact: true }).click();
	await expect(page.getByText('actor: admin')).toBeVisible();

	// alrayyes/Hush-Hush#895: the audit log pages 25 at a time with Newer and
	// Older. Twenty-six more reads of one secret push the log past a page.
	for (let i = 0; i < 26; i++) {
		const read = await page.request.get('/objects/mattermost_deploy_webhook');
		expect(read.ok()).toBe(true);
	}
	await page.goto('/audit-log');
	const auditPages = page.getByRole('navigation', { name: 'Audit log pages' });
	const newerButton = auditPages.getByRole('button', { name: 'Newer' });
	const olderButton = auditPages.getByRole('button', { name: 'Older' });
	await expect(page.locator('tbody tr')).toHaveCount(25);
	await expect(auditPages.getByText('Showing 25 entries')).toBeVisible();
	await expect(curlSnippet).toContainText('limit=25');
	await expect(newerButton).toBeDisabled();
	await expect(olderButton).toBeEnabled();
	const auditPagerAxe = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(auditPagerAxe.violations).toEqual([]);

	// The paged state in the dark theme too. Finishing the colour transitions
	// first means axe reads the final colours, with no timer.
	await page.evaluate(() =>
		document.documentElement.setAttribute('data-theme', 'dark'),
	);
	await page.evaluate(() =>
		Promise.all(document.getAnimations().map((a) => a.finished)),
	);
	const auditPagerAxeDark = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(auditPagerAxeDark.violations).toEqual([]);
	await page.evaluate(() =>
		document.documentElement.setAttribute('data-theme', 'light'),
	);

	const firstAuditPage = await rowTexts();
	await olderButton.click();
	await expect(newerButton).toBeEnabled();
	await expect.poll(rowTexts).not.toEqual(firstAuditPage);
	await newerButton.click();
	await expect.poll(rowTexts).toEqual(firstAuditPage);

	// alrayyes/Hush-Hush#897: rows per page on the audit log.
	await page.goto('/audit-log');
	const auditSize = auditPages.getByRole('combobox', { name: 'Rows per page' });
	await expect(auditSize).toHaveText('25');
	// The options, the extra row asked for and the curl command are
	// audit-log.svelte.spec.ts's. What this proves is that picking one really
	// changes the address, and that the address is what a reload and a link
	// read back.
	await auditSize.click();
	await page.getByRole('option', { name: '50', exact: true }).click();
	await expect(page).toHaveURL(/[?&]size=50(&|$)/);
	await expect(auditSize).toHaveText('50');
	await page.reload();
	await expect(auditSize).toHaveText('50');
	// The size is the address's: following the nav link to /audit-log, which
	// has no ?size=, goes back to the default and shows 25 rows again.
	await nav.getByRole('link', { name: /audit log/i }).click();
	await expect(page).not.toHaveURL(/size=/);
	await expect(auditSize).toHaveText('25');
	await expect(page.locator('tbody tr')).toHaveCount(25);
	await expect(olderButton).toBeEnabled();

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

	// #805: a consumer is added to a secret with a readable copy without
	// typing its value. The browser opens the stored value with the owner's
	// escrowed key and seals it again, so the new consumer can read it, the
	// owner still can, and the plaintext is never shown.
	const newConsumerIdentity = await age.generateIdentity();
	const reseal = await page.request.patch('/consumers/reseal-target', {
		headers: { 'X-CSRF-Token': csrfToken },
		data: { public_key: await age.identityToRecipient(newConsumerIdentity) },
	});
	expect(reseal.ok()).toBe(true);
	const grafanaConsumersLoaded = page.waitForResponse(
		(res) =>
			new URL(res.url()).pathname === '/consumers' &&
			res.request().method() === 'GET',
	);
	await grafanaRow.getByRole('button', { name: 'Edit' }).click();
	const grafanaEditDialog = page.getByRole('dialog', {
		name: 'Edit grafana_admin_password',
	});
	await grafanaConsumersLoaded;
	await expect(grafanaEditDialog.locator('#edit-value')).toHaveValue('');
	await grafanaEditDialog.locator('#edit-used-by').fill('reseal-target');
	await grafanaEditDialog.getByRole('listbox').waitFor();
	await grafanaEditDialog
		.getByRole('option', { name: /reseal-target/ })
		.first()
		.click();
	await grafanaEditDialog.getByRole('button', { name: 'Save' }).click();
	await grafanaEditDialog.waitFor({ state: 'hidden' });

	await grafanaRow.getByRole('button', { name: 'View' }).click();
	await expect(grafanaCiphertextField).not.toHaveValue('');
	const resealedBytes = Uint8Array.from(
		atob(await grafanaCiphertextField.inputValue()),
		(c) => c.charCodeAt(0),
	);
	for (const identity of [ownerIdentity, newConsumerIdentity]) {
		const reader = new age.Decrypter();
		reader.addIdentity(identity);
		expect(await reader.decrypt(resealedBytes, 'text')).toBe(ownerOptInValue);
	}
	await expect(page.getByText(ownerOptInValue)).toHaveCount(0);
	await grafanaViewDialog.getByRole('button', { name: 'Close' }).click();

	// Removing it the same way takes its read access away again.
	await grafanaRow.getByRole('button', { name: 'Edit' }).click();
	await expect(
		grafanaEditDialog.getByLabel('Remove reseal-target'),
	).toBeVisible();
	await grafanaEditDialog.getByLabel('Remove reseal-target').click();
	await grafanaEditDialog.getByRole('button', { name: 'Save' }).click();
	await grafanaEditDialog.waitFor({ state: 'hidden' });
	await grafanaRow.getByRole('button', { name: 'View' }).click();
	await expect(grafanaCiphertextField).not.toHaveValue('');
	const droppedBytes = Uint8Array.from(
		atob(await grafanaCiphertextField.inputValue()),
		(c) => c.charCodeAt(0),
	);
	const droppedReader = new age.Decrypter();
	droppedReader.addIdentity(newConsumerIdentity);
	await expect(droppedReader.decrypt(droppedBytes, 'text')).rejects.toThrow();
	const ownerReader = new age.Decrypter();
	ownerReader.addIdentity(ownerIdentity);
	expect(await ownerReader.decrypt(droppedBytes, 'text')).toBe(ownerOptInValue);
	await grafanaViewDialog.getByRole('button', { name: 'Close' }).click();

	// A secret with no readable copy can't be re-sealed without its value:
	// the dialog says so, and the stored secret is unchanged.
	const noCopyBefore = await page.request.get(
		'/objects/mattermost_deploy_webhook',
	);
	const noCopyConsumersLoaded = page.waitForResponse(
		(res) =>
			new URL(res.url()).pathname === '/consumers' &&
			res.request().method() === 'GET',
	);
	await page
		.getByRole('row', { name: /mattermost_deploy_webhook/ })
		.getByRole('button', { name: 'Edit' })
		.click();
	const noCopyDialog = page.getByRole('dialog', {
		name: 'Edit mattermost_deploy_webhook',
	});
	await noCopyConsumersLoaded;
	await noCopyDialog.locator('#edit-used-by').fill('reseal-target');
	await noCopyDialog.getByRole('listbox').waitFor();
	await noCopyDialog
		.getByRole('option', { name: /reseal-target/ })
		.first()
		.click();
	await noCopyDialog.getByRole('button', { name: 'Save' }).click();
	await expect(noCopyDialog.getByRole('alert')).toContainText(
		'no readable copy',
	);
	const noCopyResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(noCopyResults.violations).toEqual([]);
	await noCopyDialog.getByRole('button', { name: 'Cancel' }).click();
	const noCopyAfter = await page.request.get(
		'/objects/mattermost_deploy_webhook',
	);
	expect(await noCopyAfter.body()).toEqual(await noCopyBefore.body());

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
	// What the notice and a passkey card say are settings.svelte.spec.ts's.

	await page.setViewportSize({ width: 390, height: 844 });
	await expect(recoveryNotice).toBeVisible();
	const passkeyCard = page
		.getByRole('list', { name: 'Passkey list' })
		.getByRole('listitem')
		.first();
	await expect(passkeyCard).toBeVisible();
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
	await expectListParity(page, {
		path: '/settings',
		list: 'Passkey list',
		ids: [],
		facts: {
			added: { pattern: TIMESTAMP },
			'last used': { pattern: /never/ },
		},
	});

	// Consumer tokens: create/rotate/revoke, and the single-select
	// consumer picker (max=1, showKeyStatus=false -
	// openspec/changes/consumer-read-tokens-web-ui/design.md). "ci-runner"
	// is deliberately a consumer with no registered public key; that the
	// picker then shows no "(no key)" hint is settings.svelte.spec.ts's.
	await page.getByRole('button', { name: 'New consumer token' }).click();
	const createConsumerTokenDialog = page.getByRole('dialog', {
		name: 'Create a consumer token',
	});
	await page.locator('#consumer-token-consumer').fill('ci-runner');
	await page.getByRole('listbox').waitFor();
	await page.getByRole('option', { name: 'Add "ci-runner"' }).click();
	await page
		.locator('#consumer-token-description')
		.fill('deploy read token for ci-runner');
	// The lifetime's default and limits (#537), the picker leaving no key hint
	// and the repeat warning's text are settings.svelte.spec.ts's.
	const consumerTokenTtl = page.locator('#consumer-token-ttl');
	await consumerTokenTtl.fill('30');
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

	// alrayyes/hush-hush#710: two tokens for one consumer and description
	// differ only by id, so the table shows it. A second one is minted
	// straight through the API, checked, then removed so the rows below see
	// only the one token they expect.
	const twin = await page.request.post('/consumer-tokens', {
		headers: { 'X-CSRF-Token': csrfToken },
		data: {
			consumer: 'ci-runner',
			description: 'deploy read token for ci-runner',
			ttl_seconds: 30 * 86_400,
		},
	});
	expect(twin.ok()).toBe(true);
	const twinId = (await twin.json()).id as string;
	await page.reload();
	const sameConsumerRows = page.getByRole('row', { name: /ci-runner/ });
	await expect(sameConsumerRows).toHaveCount(2);
	const shownIds = (
		await sameConsumerRows.locator('[data-testid="token-id"]').allTextContents()
	).map((id) => id.trim());
	expect(shownIds).toHaveLength(2);
	expect(shownIds[0]).toMatch(/^[0-9a-f]{16}$/);
	expect(new Set(shownIds).size).toBe(2);
	expect(shownIds).toContain(twinId);
	await expect(sameConsumerRows.first()).toContainText(/ID [0-9a-f]{16}/);
	expect(
		(
			await page.request.delete(`/consumer-tokens/${twinId}`, {
				headers: { 'X-CSRF-Token': csrfToken },
			})
		).ok(),
	).toBe(true);
	expect(
		(
			await page.request.delete(`/consumer-tokens/${twinId}/purge`, {
				headers: { 'X-CSRF-Token': csrfToken },
			})
		).ok(),
	).toBe(true);
	await page.reload();
	await expect(sameConsumerRows).toHaveCount(1);

	// alrayyes/hush-hush#711: repeating an active token's consumer and
	// description warns and lets the user go ahead. What the warning says is
	// settings.svelte.spec.ts's; here the server's token id feeds it.
	const existingTokenId = (
		await sameConsumerRows.locator('[data-testid="token-id"]').textContent()
	)?.trim();
	await page.getByRole('button', { name: 'New consumer token' }).click();
	const repeatDialog = page.getByRole('dialog', {
		name: 'Create a consumer token',
	});
	await page.locator('#consumer-token-consumer').fill('ci-runner');
	await page.getByRole('listbox').waitFor();
	await page.getByRole('option', { name: 'Add "ci-runner"' }).click();
	await expect(repeatDialog.getByRole('alert')).toHaveCount(0);
	await repeatDialog
		.locator('#consumer-token-description')
		.fill('deploy read token for ci-runner');
	await expect(
		repeatDialog.getByRole('button', { name: 'Create anyway' }),
	).toBeVisible();
	// The submit button fades in once the form is valid; scan after it has.
	await expect(
		repeatDialog.getByRole('button', { name: 'Create anyway' }),
	).toHaveCSS('opacity', '1');
	const repeatWarningResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(repeatWarningResults.violations).toEqual([]);
	await repeatDialog
		.locator('#consumer-token-description')
		.fill('another description');
	await expect(repeatDialog.getByRole('alert')).toHaveCount(0);
	await expect(
		repeatDialog.getByRole('button', { name: 'Create', exact: true }),
	).toBeVisible();
	await repeatDialog
		.locator('#consumer-token-description')
		.fill('deploy read token for ci-runner');
	await repeatDialog.getByRole('button', { name: 'Create anyway' }).click();
	await page
		.getByRole('dialog', { name: 'Token created' })
		.getByRole('button', { name: 'Done' })
		.click();
	await page.reload();
	await expect(sameConsumerRows).toHaveCount(2);
	// Remove the one just made, so the rows below see the one they expect.
	const repeated = (
		(await (await page.request.get('/consumer-tokens')).json()) as {
			id: string;
			consumer: string;
		}[]
	).filter((t) => t.consumer === 'ci-runner' && t.id !== existingTokenId);
	expect(repeated).toHaveLength(1);
	for (const suffix of ['', '/purge']) {
		expect(
			(
				await page.request.delete(
					`/consumer-tokens/${repeated[0].id}${suffix}`,
					{ headers: { 'X-CSRF-Token': csrfToken } },
				)
			).ok(),
		).toBe(true);
	}
	await page.reload();
	await expect(sameConsumerRows).toHaveCount(1);
	// Before the clock-skew page below: page.clock is the context's clock, so a
	// page opened after it would see the skewed time.
	await expectListParity(page, {
		path: '/settings',
		list: 'Consumer token list',
		ids: ['ci-runner'],
		facts: {
			description: { pattern: /deploy read token for ci-runner/ },
			status: { pattern: /Active/ },
			'created and expires': { pattern: TIMESTAMP, times: 2 },
			'last used': { pattern: /never/ },
			'time left': { pattern: /\d+d left/ },
			'token id': { pattern: /ID [0-9a-f]{16}/ },
		},
	});

	// That a token's status and buttons are the server's answer, not the browser
	// clock's (#536), is settings.svelte.spec.ts's, for both token tables.
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
	const rotatedCopyResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(rotatedCopyResults.violations).toEqual([]);
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
	await expect(purgeConsumerTokenDialog).toBeVisible();
	const purgeResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(purgeResults.violations).toEqual([]);
	await purgeConsumerTokenDialog
		.getByRole('button', { name: 'Delete permanently' })
		.click();
	await expect(consumerTokenRow).toHaveCount(0);

	// The bearer-token side of the same gating, with a token that is dead. A
	// revoked one is made and revoked through the API, so there is nothing to
	// wait for; how a dead token reads and which buttons it gets, expired or
	// revoked, is settings.svelte.spec.ts's. What is left here is that the two
	// layouts agree, that the card's delete is a 44px target, and that the
	// delete really removes the row.
	const created = await page.request.post('/tokens', {
		headers: { 'X-CSRF-Token': csrfToken },
		data: { description: 'revoked token', ttl_seconds: 3600 },
	});
	const { id: revokedTokenId } = await created.json();
	await page.request.delete(`/tokens/${revokedTokenId}`, {
		headers: { 'X-CSRF-Token': csrfToken },
	});
	await page.reload();
	await page.getByRole('heading', { name: 'Settings' }).waitFor();

	const deadRow = page.getByRole('row', { name: /revoked token/ });
	await expectListParity(page, {
		path: '/settings',
		list: 'Bearer token list',
		ids: [],
		facts: {
			description: { pattern: /revoked token/ },
			status: { pattern: /Revoked/ },
			'created and expires': { pattern: TIMESTAMP, times: 2 },
			'last used': { pattern: /never/ },
		},
	});

	// #484: below md the dead token's delete is a 44px target.
	await page.setViewportSize({ width: 390, height: 844 });
	const deadCard = page
		.getByRole('list', { name: 'Bearer token list' })
		.getByRole('listitem')
		.filter({ hasText: 'revoked token' });
	const purgeBox = await deadCard
		.getByRole('button', { name: 'Delete permanently' })
		.boundingBox();
	expect(purgeBox?.width).toBeGreaterThanOrEqual(44);
	expect(purgeBox?.height).toBeGreaterThanOrEqual(44);
	await page.setViewportSize({ width: 1280, height: 720 });
	await deadRow.getByRole('button', { name: 'Delete permanently' }).click();
	await page
		.getByRole('alertdialog', { name: 'Delete this token permanently?' })
		.getByRole('button', { name: 'Delete permanently' })
		.click();
	await expect(deadRow).toHaveCount(0);

	await page.setViewportSize({ width: 320, height: 720 });

	// #294: the topbar's nav links, theme toggle, and Log out button used
	// to each wrap onto their own line at phone width via `flex-wrap`
	// fighting `margin-left: auto` - four-plus ragged rows before any page
	// content was visible. Clustering every topbar control's own top
	// offset (within a tolerance wider than the few px a link and a
	// padded button can differ by even centered on the same line) catches
	// that without pinning an exact pixel height to font metrics. The nav
	// links themselves moved to the bottom tab bar below md (#480), so what's
	// left in the topbar to cluster is the brand, the instance label, the
	// status pill and the account menu button (#481).
	const topbarControls = [
		page.getByRole('banner').getByText('HUSH-HUSH'),
		page.getByRole('banner').getByText('e2e / local'),
		page.getByRole('banner').getByText('ACTIVE', { exact: true }),
		page.getByRole('banner').getByRole('button', { name: 'Account menu' }),
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

	// alrayyes/hush-hush#522: the secrets list shows each secret's tags and
	// filters by one. Done last because it adds secrets, and every earlier
	// assertion counts audit rows and secrets. The values are sealed here,
	// not through the dialog (which can't set tags yet, #523). The tags on a
	// card, the pills and their counts, the pressed state, a tag with the
	// search box and who a card says created and updated a secret are
	// overview.svelte.spec.ts's; attribution past the first 50 audit events
	// (#535) is store/attribution_test.go's.
	for (const [slug, tags] of [
		['tagged_one', ['prod', 'homelab']],
		['tagged_two', ['prod']],
		['tagged_three', ['backup']],
	] as const) {
		const encrypter = new age.Encrypter();
		encrypter.addRecipient(homelabRecipient);
		const sealed = await encrypter.encrypt(`value of ${slug}`);
		const created = await page.request.post('/objects', {
			headers: { 'X-CSRF-Token': csrfToken },
			data: { slug, value: btoa(String.fromCharCode(...sealed)), tags },
		});
		expect(created.ok()).toBe(true);
	}

	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto('/');
	const cardList = page.getByRole('list', { name: 'Secrets' });
	await expect(cardList.getByRole('listitem')).toHaveCount(5);

	const tagFilter = page.getByRole('group', { name: 'Filter by tag' });
	for (const pill of await tagFilter.getByRole('button').all()) {
		const box = await pill.boundingBox();
		expect(box?.height).toBeGreaterThanOrEqual(44);
	}

	// One real click, to prove the filter is wired to the real list.
	await tagFilter.getByRole('button', { name: 'prod (2)' }).click();
	await expect(cardList.getByRole('listitem')).toHaveCount(2);

	// The pills animate their colours when pressed, and axe samples a
	// mid-transition colour as a false contrast failure: let every CSS
	// transition finish before scanning.
	await page.waitForFunction(() => document.getAnimations().length === 0);
	const tagResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(tagResults.violations).toEqual([]);

	await tagFilter.getByRole('button', { name: 'All (5)' }).click();
	await expect(cardList.getByRole('listitem')).toHaveCount(5);

	// alrayyes/hush-hush#670: one name can hold a value per consumer, each
	// under its own id. Two are made through the API (the create dialog is
	// exercised for the third below); sealing here only needs a well-formed
	// age file, which any recipient gives.
	const sealForVariant = async (text: string) => {
		const encrypter = new age.Encrypter();
		encrypter.addRecipient(homelabRecipient);
		const sealed = await encrypter.encrypt(text);

		return btoa(String.fromCharCode(...sealed));
	};
	for (const consumer of ['consumer_a', 'consumer_d']) {
		const created = await page.request.post('/objects', {
			headers: { 'X-CSRF-Token': csrfToken },
			data: {
				slug: 'release_token',
				value: await sealForVariant(`token for ${consumer}`),
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
	const variantEdit = page.getByRole('dialog', { name: 'Edit release_token' });
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
	await variantCreate.locator('#create-value').fill('token for consumer_d too');
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
	await variantCreate.getByRole('option', { name: 'Add "consumer_e"' }).click();
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
			headers: { 'X-CSRF-Token': csrfToken },
			data: {
				slug,
				value: await sealForVariant(slug),
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
	for (const width of [1280]) {
		await page.setViewportSize({ width, height: 900 });
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
	}

	// alrayyes/Hush-Hush#894: the consumers directory pages at 20 with the
	// shared pager. The 100 consumers seeded above are five pages and more.
	await page.setViewportSize({ width: 1280, height: 900 });
	await page.goto('/consumers');
	const consumerPages = page.getByRole('navigation', {
		name: 'Consumers pages',
	});
	await expect(consumerPages.getByText('Showing 1–20 of 102')).toBeVisible();
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
	const widestSlug = `wide_${'x'.repeat(123)}`;
	const widest = await page.request.post('/objects', {
		headers: { 'X-CSRF-Token': csrfToken },
		data: {
			slug: widestSlug,
			value: await sealForVariant(widestSlug),
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
	await consumersDialog.getByRole('button', { name: 'Close' }).first().click();
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

	for (const slug of ['many_consumers', 'few_consumers', 'some_consumers']) {
		expect(
			(
				await page.request.delete(`/objects/${slug}`, {
					headers: { 'X-CSRF-Token': csrfToken },
				})
			).ok(),
		).toBe(true);
	}

	// alrayyes/Hush-Hush#893: the Secrets table pages 20 at a time, the page is
	// in the URL, and a short list shows no pager. Seeded on top of whatever
	// the journey left behind, so the totals are worked out, not assumed.
	const before = await (await page.request.get('/objects')).json();
	const existing = (before.objects ?? before).length as number;
	const pagingSlugs = Array.from(
		{ length: 21 },
		(_, i) => `paging_${String(i).padStart(2, '0')}`,
	);
	for (const slug of pagingSlugs) {
		const created = await page.request.post('/objects', {
			headers: { 'X-CSRF-Token': csrfToken },
			data: {
				slug,
				value: await sealForVariant(slug),
				used_by: [],
				tags: ['paging'],
			},
		});
		expect(created.ok()).toBe(true);
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
	await page.getByRole('searchbox', { name: 'Filter secrets' }).fill('paging_');
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

	for (const slug of pagingSlugs) {
		expect(
			(
				await page.request.delete(`/objects/${slug}`, {
					headers: { 'X-CSRF-Token': csrfToken },
				})
			).ok(),
		).toBe(true);
	}

	// alrayyes/Hush-Hush#896: the Settings lists page on their own, 10 at a time,
	// each with its page in the address. Twelve more bearer tokens and consumer
	// tokens push both past a page; the passkeys stay on one.
	for (let i = 0; i < 12; i++) {
		const n = String(i).padStart(2, '0');
		expect(
			(
				await page.request.post('/tokens', {
					headers: { 'X-CSRF-Token': csrfToken },
					data: { description: `pager bearer ${n}`, ttl_seconds: 86_400 },
				})
			).ok(),
		).toBe(true);
		expect(
			(
				await page.request.post('/consumer-tokens', {
					headers: { 'X-CSRF-Token': csrfToken },
					data: {
						consumer: `pager-consumer-${n}`,
						description: `pager consumer ${n}`,
						ttl_seconds: 86_400,
					},
				})
			).ok(),
		).toBe(true);
	}
	await page.setViewportSize({ width: 1280, height: 900 });
	await page.goto('/settings');
	const bearerPager = page.getByRole('navigation', {
		name: 'Bearer tokens pages',
	});
	const consumerPager = page.getByRole('navigation', {
		name: 'Consumer tokens pages',
	});
	// The rows and pager text of each list, a list's page taken from its own
	// parameter, the links keeping the other list's position, a page past the
	// end, the consumer filter and the phone cards are
	// settings.svelte.spec.ts's. What these two real clicks, the scroll and the
	// reload prove is that the address really carries both positions.
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
		const settingsPagerResults = await new AxeBuilder({ page })
			.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
			.analyze();
		expect(settingsPagerResults.violations, theme).toEqual([]);
	}
	await page.evaluate(() =>
		document.documentElement.setAttribute('data-theme', 'light'),
	);

	await page.setViewportSize({ width: 1280, height: 800 });

	// Log out lives in .topbar-actions, not <nav> - it's an account
	// action, not a navigation link (#294).
	await page.getByRole('button', { name: 'Log out' }).click();
	await page.waitForURL('/login');

	await page.goto('/changelog');
	await expect(page.locator('nav')).toHaveCount(0);
});
