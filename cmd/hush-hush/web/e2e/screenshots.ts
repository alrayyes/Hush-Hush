// Captures the README's light/dark screenshots of every page except the footer's against the real running
// binary - a standalone script, not a Playwright test, since it has no
// pass/fail assertion of its own and isn't part of the normal `test:e2e`
// run. e2e/server.sh is what actually builds and starts the server;
// SCREENSHOT_SERVER_URL lets the workflow that runs this reuse a server it
// already started instead of spawning a second one.
import { chromium } from '@playwright/test';
import * as age from 'age-encryption';

const base = process.env.SCREENSHOT_SERVER_URL ?? 'http://localhost:4173';
const outDir = process.env.SCREENSHOT_OUT_DIR ?? '../../../docs/screenshots';

// A real, if unremarkable, consumer - the create dialog only enables
// Create once every sample secret resolves at least one real recipient
// (client-side sealing, #395): sealing to nobody is no longer possible,
// screenshots included.
const CONSUMER = 'homelab/vps-docker';

const COLOR_SCHEMES = ['light', 'dark'] as const;

// Every page behind the login except the footer's (changelog, disclaimer,
// privacy, licence), which are plain text and not worth a README image.
const PAGES = [
	{ name: 'secrets', path: '/', heading: 'Secrets' },
	{
		name: 'secret',
		path: '/secrets/mattermost_deploy_webhook',
		heading: 'mattermost_deploy_webhook',
	},
	{ name: 'consumers', path: '/consumers', heading: 'Consumers' },
	{ name: 'audit-log', path: '/audit-log', heading: 'Audit log' },
	{ name: 'settings', path: '/settings', heading: 'Settings' },
] as const;

const SAMPLE_SECRETS = [
	{
		id: 'mattermost_deploy_webhook',
		value: 'prod deploy webhook secret, rotated on incident',
		description: 'prod deploy webhook for homelab/vps-docker',
	},
	{
		id: 'grafana_admin_password',
		value: 'grafana admin console password',
		description: 'admin console, rotated quarterly',
	},
	{
		id: 'backup_encryption_key',
		value: 'nightly offsite backup encryption key',
		description: 'nightly offsite backup, age recipient',
	},
] as const;

const browser = await chromium.launch();
const context = await browser.newContext();
const page = await context.newPage();
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

await page.setViewportSize({ width: 1280, height: 800 });

// The login page is only reachable signed out, so it is captured before the
// passkey registration below, not alongside the other pages.
await page.goto(`${base}/login`);
for (const colorScheme of COLOR_SCHEMES) {
	await page.emulateMedia({ colorScheme });
	await page.getByRole('heading', { name: 'hush-hush' }).waitFor();
	await page.screenshot({ path: `${outDir}/login-${colorScheme}.png` });
}
await page.getByRole('button', { name: 'Register passkey' }).click();

// A first-ever registration shows the one-time recovery-phrase dialog
// (client-side-encryption, #396) before the login page navigates onward -
// closeRecoveryPhrase only fires once "I've saved it" is clicked.
await page.getByRole('button', { name: "I've saved it" }).click();
await page.waitForURL(`${base}/`);

// A real age keypair, generated here rather than through the app -
// registering its public key against CONSUMER before it's picked below is
// what lets the create dialog's Create button ever enable: a secret with
// zero resolved recipients can't be created at all.
const identity = await age.generateIdentity();
const recipient = await age.identityToRecipient(identity);
const csrfToken =
	(await context.cookies()).find((c) => c.name === 'csrf_token')?.value ?? '';
await page.request.patch(`${base}/consumers/${encodeURIComponent(CONSUMER)}`, {
	headers: { 'X-CSRF-Token': csrfToken },
	data: { public_key: recipient },
});

for (const secret of SAMPLE_SECRETS) {
	// The create dialog's ConsumerCombobox mounts fresh on every open and
	// fires its own GET /consumers - has to be awaited before typing into
	// #create-used-by, or the fetch resolving mid-fill mutates the DOM and
	// steals focus back, silently dropping the keystrokes.
	const consumersLoaded = page.waitForResponse(
		(res) =>
			new URL(res.url()).pathname === '/consumers' &&
			res.request().method() === 'GET',
	);
	await page.getByRole('button', { name: 'New secret' }).click();
	await page.locator('#create-id').fill(secret.id);
	await page.locator('#create-value').fill(secret.value);
	await page.locator('#create-description').fill(secret.description);
	await consumersLoaded;
	await page.locator('#create-used-by').fill(CONSUMER);
	await page.getByRole('listbox').waitFor();
	await page.getByRole('option').first().click();
	await page.getByRole('button', { name: 'Create' }).click();
	// "New secret" is on screen the whole time, so it can't say the create
	// finished - the new row can. Without this the last secret was dropped
	// whenever the next step navigated away before its request landed.
	await page.getByRole('cell', { name: secret.id }).first().waitFor();
}

// Opening a secret writes a `read` row to the audit log, so open each one
// once up front: both colour schemes then show the same events, whichever
// page is captured first.
for (const secret of SAMPLE_SECRETS) {
	await page.goto(`${base}/secrets/${secret.id}`);
	await page.getByRole('heading', { name: secret.id, exact: true }).waitFor();
}

// One authenticated session, switched between color schemes rather than
// a second registration - only one admin account can ever exist, so a
// second bootstrap has nothing left to register against.
for (const colorScheme of COLOR_SCHEMES) {
	await page.emulateMedia({ colorScheme });
	for (const { name, path, heading } of PAGES) {
		await page.goto(`${base}${path}`);
		// The lists fill in after the heading renders; wait for them to settle.
		await page.waitForLoadState('networkidle');
		await page
			.getByRole('heading', { name: heading, exact: true })
			.first()
			.waitFor();
		await page.screenshot({ path: `${outDir}/${name}-${colorScheme}.png` });
	}
}

await browser.close();

console.log(
	`Wrote ${(PAGES.length + 1) * COLOR_SCHEMES.length} screenshots to docs/screenshots/`,
);
