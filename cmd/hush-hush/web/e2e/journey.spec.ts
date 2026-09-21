import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

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

	await page.goto('/login');
	await page.getByRole('button', { name: 'Register passkey' }).click();
	await page.waitForURL('/');

	const nav = page.locator('nav');
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

	// A real secret with real width pressure - #271's own gap: the
	// public-pages-only viewport test never caught the authenticated
	// pages' tables scrolling sideways in their own box at 320px. The
	// same dialog also exercises the consumer combobox's listbox, open
	// and populated - the hand-written ARIA APG combobox
	// (alrayyes/hush-hush#251) is the newest, most complex interactive
	// widget added since the last scan.
	await page.getByRole('button', { name: 'New secret' }).click();
	// bits-ui's Dialog autofocuses the first field (Id) on open, racing any
	// interaction with a later field started right away - wait for that
	// autofocus to settle before touching a later field, or its own focus
	// gets stolen back mid-fill.
	await expect(page.getByLabel('Id')).toBeFocused();
	await page.locator('#create-id').fill('mattermost_deploy_webhook');
	await page.locator('#create-value').fill(btoa('placeholder'));
	await page
		.locator('#create-description')
		.fill('prod deploy webhook for homelab/vps-docker');
	await page.locator('#create-used-by').fill('homelab');
	await page.getByRole('listbox').waitFor();
	const comboboxResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(comboboxResults.violations).toEqual([]);
	await page.getByRole('option', { name: 'Add "homelab"' }).click();
	await page.getByRole('button', { name: 'Create' }).click();
	await page.getByRole('button', { name: 'New secret' }).waitFor();

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
	const viewResults = await new AxeBuilder({ page })
		.withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
		.analyze();
	expect(viewResults.violations).toEqual([]);
	await viewDialog.getByRole('button', { name: 'Close' }).click();

	// Edit: the combobox comes pre-filled with the existing "homelab"
	// consumer, and adding "ci" alongside it is what proves the update
	// actually reaches the server rather than only the local form state.
	await page.getByRole('button', { name: 'Edit' }).click();
	const editDialog = page.getByRole('dialog', {
		name: 'Edit mattermost_deploy_webhook',
	});
	await expect(editDialog.getByLabel('Remove homelab')).toBeVisible();
	await editDialog.locator('#edit-used-by').fill('ci');
	await editDialog.getByRole('option', { name: 'Add "ci"' }).click();
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
	await expect(page.getByText('Filtered to consumer homelab')).toBeVisible();
	await expect(page.getByText('mattermost_deploy_webhook')).toBeVisible();
	await page.getByRole('link', { name: 'Clear filter' }).click();
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

	await page.setViewportSize({ width: 320, height: 720 });

	// #294: the topbar's nav links, theme toggle, and Log out button used
	// to each wrap onto their own line at phone width via `flex-wrap`
	// fighting `margin-left: auto` - four-plus ragged rows before any page
	// content was visible. Clustering every topbar control's own top
	// offset (within a tolerance wider than the few px a link and a
	// padded button can differ by even centered on the same line) catches
	// that without pinning an exact pixel height to font metrics.
	const topbarControls = [
		...(await nav.getByRole('link').all()),
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
	// about the mobile layout, not routing.
	for (const linkName of ['Secrets', 'Consumers', 'Audit log', 'Settings']) {
		await nav.getByRole('link', { name: linkName }).click();
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
