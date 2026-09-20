// Captures the README's light/dark screenshots against the real running
// binary - a standalone script, not a Playwright test, since it has no
// pass/fail assertion of its own and isn't part of the normal `test:e2e`
// run. e2e/server.sh is what actually builds and starts the server;
// SCREENSHOT_SERVER_URL lets the workflow that runs this reuse a server it
// already started instead of spawning a second one.
import { chromium } from '@playwright/test';

const base = process.env.SCREENSHOT_SERVER_URL ?? 'http://localhost:4173';
const outDir = process.env.SCREENSHOT_OUT_DIR ?? '../../../docs/screenshots';

const SAMPLE_SECRETS = [
	{
		id: 'mattermost_deploy_webhook',
		description: 'prod deploy webhook for homelab/vps-docker',
	},
	{
		id: 'grafana_admin_password',
		description: 'admin console, rotated quarterly',
	},
	{
		id: 'backup_encryption_key',
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

await page.goto(`${base}/login`);
await page.getByRole('button', { name: 'Register passkey' }).click();
await page.waitForURL(`${base}/`);

for (const secret of SAMPLE_SECRETS) {
	await page.getByRole('button', { name: 'New secret' }).click();
	await page.locator('#create-id').fill(secret.id);
	await page
		.locator('#create-value')
		.fill(btoa(`sealed-placeholder-${secret.id}`));
	await page.locator('#create-description').fill(secret.description);
	await page.getByRole('button', { name: 'Create' }).click();
	await page.getByRole('button', { name: 'New secret' }).waitFor();
}

await page.setViewportSize({ width: 1280, height: 800 });

// One authenticated session, switched between color schemes rather than
// a second registration - only one admin account can ever exist, so a
// second bootstrap has nothing left to register against.
for (const colorScheme of ['light', 'dark'] as const) {
	await page.emulateMedia({ colorScheme });
	await page.reload();
	await page.getByRole('heading', { name: 'Secrets' }).waitFor();
	await page.screenshot({ path: `${outDir}/secrets-${colorScheme}.png` });
}

await browser.close();

console.log('Wrote docs/screenshots/secrets-light.png and secrets-dark.png');
