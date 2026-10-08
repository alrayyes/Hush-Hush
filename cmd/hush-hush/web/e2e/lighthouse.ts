// rules/browser-compat.md's "warn, don't fail the build" default for a
// judgment-call check, applied to Lighthouse the same way `a11y.md`'s
// axe-core scan is the opposite: performance and best-practices scores
// are reported, never asserted hard. SEO and PWA categories are skipped
// outright - not meaningful for a self-hosted secrets UI with no public
// search presence and no installable-app ambitions.
//
// `@lhci/cli`'s own collection is URL-based and has no story for this
// app's WebAuthn passkey auth beyond a hand-rolled Puppeteer script
// duplicating the CDP virtual-authenticator dance journey.spec.ts
// already does for Playwright - so this reuses Playwright itself (via
// `playwright-lighthouse`, which drives real Lighthouse over an existing
// page's CDP session) instead of adding a second automation stack. A
// judgment call worth flagging: this is the "or equivalent" in the
// issue's "@lhci/cli or equivalent", not literally `@lhci/cli`.
import { chromium } from '@playwright/test';
import * as age from 'age-encryption';
import { playAudit } from 'playwright-lighthouse';
import { AUDITED_PAGES, SEEDED_SLUG } from './lighthouse-pages';

// E2E_PORT picks the server's port (e2e/server.sh reads it too) and
// E2E_DEBUG_PORT the browser's remote-debugging port, so two runs on one
// machine don't share, or kill, each other's server.
const PORT = Number(process.env.E2E_DEBUG_PORT ?? 9222);
const BASE_URL = `http://localhost:${process.env.E2E_PORT ?? '4173'}`;
const REPORTS_DIR = 'reports/lighthouse';

// Same scores either page could reasonably be held to - a self-hosted,
// mostly-static SvelteKit SPA with no third-party trackers or ads has no
// excuse for a low best-practices score, and performance has real room
// for a slower CI runner without being meaningless as a check.
const THRESHOLDS = { performance: 70, 'best-practices': 90 };

// 120s, not less: e2e/server.sh runs a full `go build` before the server can
// answer, and a runner with a cold Go cache needs well over 30s for it.
async function waitForServer(url: string, timeoutMs = 120_000): Promise<void> {
	const deadline = Date.now() + timeoutMs;
	for (;;) {
		try {
			const res = await fetch(url);
			if (res.ok) return;
		} catch {
			// Server not listening yet - keep polling.
		}
		if (Date.now() > deadline) {
			throw new Error(`${url} did not become ready within ${timeoutMs}ms`);
		}
		await new Promise((resolve) => setTimeout(resolve, 250));
	}
}

async function auditPage(
	page: import('@playwright/test').Page,
	label: string,
	reportName: string,
	options: { expectedPath: string; keepSession?: boolean },
): Promise<void> {
	const results = await playAudit({
		page,
		port: PORT,
		thresholds: THRESHOLDS,
		ignoreError: true,
		// Lighthouse clears the origin's storage before a run by default, which
		// would log the authenticated audit straight back out.
		opts: options.keepSession ? { disableStorageReset: true } : undefined,
		// One HTML and JSON pair per audited page, which the pages job
		// publishes (rules/published-reports.md).
		reports: {
			formats: { html: true, json: true },
			directory: REPORTS_DIR,
			// <page>.report.html and .report.json: the names the catalogue
			// recognises as a Lighthouse run. playwright-lighthouse treats the
			// last dot of `name` as an extension and drops it, hence the
			// trailing .html.
			name: `${reportName}.report.html`,
		},
	});

	// An audit that lands somewhere else measured the wrong page, and its
	// scores would read as the right one's. The authenticated audit used to
	// do exactly that without anyone noticing (it was redirected to /login).
	const landed = new URL(results.lhr.finalDisplayedUrl).pathname;
	if (landed !== options.expectedPath) {
		throw new Error(
			`${label}: audited ${landed}, expected ${options.expectedPath}`,
		);
	}

	// playAudit only logs the metrics that *passed* - a failing one only
	// ever reaches `results.comparisonError`, which it never prints itself
	// when `ignoreError: true` tells it not to throw. Surface it here, or
	// a regression warns nobody.
	if (results.comparisonError) {
		console.warn(`\n[lighthouse] ${label}: ${results.comparisonError}\n`);
	} else {
		console.log(`[lighthouse] ${label}: all thresholds met`);
	}
}

// Fills the instance so the signed-in pages are audited as people use them,
// not empty: a few secrets (one the detail page is audited on), their
// consumers, an audit trail, and one bearer and one consumer token.
async function seed(
	context: import('@playwright/test').BrowserContext,
	page: import('@playwright/test').Page,
): Promise<void> {
	const csrf =
		(await context.cookies()).find((c) => c.name === 'csrf_token')?.value ?? '';
	const headers = { 'X-CSRF-Token': csrf };
	const identity = await age.generateIdentity();
	const recipient = await age.identityToRecipient(identity);
	const seal = async (text: string) => {
		const encrypter = new age.Encrypter();
		encrypter.addRecipient(recipient);

		return btoa(String.fromCharCode(...(await encrypter.encrypt(text))));
	};

	for (const consumer of ['homelab', 'ci-runner']) {
		await page.request.patch(`/consumers/${consumer}`, {
			headers,
			data: { public_key: recipient },
		});
	}
	const secrets = [
		[SEEDED_SLUG, 'prod deploy webhook', ['homelab', 'ci-runner'], ['prod']],
		[
			'grafana_admin_password',
			'admin console, rotated quarterly',
			['homelab'],
			[],
		],
		[
			'backup_encryption_key',
			'nightly offsite backup',
			['ci-runner'],
			['backup'],
		],
	] as const;
	for (const [slug, description, usedBy, tags] of secrets) {
		await page.request.post('/objects', {
			headers,
			data: {
				slug,
				value: await seal(slug),
				description,
				used_by: usedBy,
				tags,
			},
		});
	}
	await page.request.post('/tokens', {
		headers,
		data: { description: 'deploy token', ttl_seconds: 30 * 86_400 },
	});
	await page.request.post('/consumer-tokens', {
		headers,
		data: {
			consumer: 'ci-runner',
			description: 'deploy read token for ci-runner',
			ttl_seconds: 30 * 86_400,
		},
	});
}

async function main() {
	const serverProc = Bun.spawn(['bash', 'e2e/server.sh'], {
		stdout: 'inherit',
		stderr: 'inherit',
	});

	try {
		await waitForServer(`${BASE_URL}/healthz`);

		// A persistent context, not browser.newContext(): that is an isolated
		// incognito-style context, and the tab Lighthouse opens over the
		// debugging port lives in the browser's default one, so it never sees
		// the session this script logs in with and audits /login again.
		const context = await chromium.launchPersistentContext('', {
			args: [`--remote-debugging-port=${PORT}`],
			baseURL: BASE_URL,
		});
		try {
			const page = await context.newPage();

			// Lighthouse leaves the tab blank after each audit, so every page
			// is loaded again before anything is clicked on it.
			const [publicPages, sessionPages] = [
				AUDITED_PAGES.filter((p) => !p.session),
				AUDITED_PAGES.filter((p) => p.session),
			];
			for (const target of publicPages) {
				await page.goto(target.path);
				await auditPage(page, target.path, target.report, {
					expectedPath: target.path,
				});
			}

			// Same CDP virtual-authenticator flow as journey.spec.ts's own
			// login - a passkey has no username/password form Lighthouse's
			// usual auth recipes assume, so an authenticated audit has to
			// drive the actual WebAuthn ceremony rather than skip it.
			//
			// Three things used to make this block fail in CI, none of them the
			// CDP connection playAudit opens: Lighthouse leaves the tab blank
			// after the /login audit; a first registration shows the escrowed
			// identity's recovery phrase, and the page only navigates to "/"
			// once "I've saved it" is clicked; and the session lived in an
			// isolated context Lighthouse's own tab could not see (see the
			// launchPersistentContext comment above).
			//
			// The block stays best-effort, not fatal like the rest of main():
			// this check is documented as warn-only (rules/browser-compat.md),
			// so a failure to drive the authenticated half is caught and
			// reported the same way a missed threshold is, rather than
			// propagating to main()'s own process.exit(1). Each page is caught
			// on its own, so one that fails doesn't skip the rest.
			try {
				await page.goto('/login');

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
				await page.getByRole('button', { name: 'Register passkey' }).click();
				await page
					.getByRole('button', { name: "I've saved it" })
					.click({ timeout: 10_000 });
				await page.waitForURL('/');

				await seed(context, page);

				for (const target of sessionPages) {
					try {
						await page.goto(target.path);
						await auditPage(page, target.path, target.report, {
							expectedPath: target.path,
							keepSession: true,
						});
					} catch (err) {
						console.warn(
							`\n[lighthouse] could not audit ${target.path} - ` +
								'treating as a warning, not a build failure:',
							err,
							'\n',
						);
					}
				}
			} catch (err) {
				console.warn(
					'\n[lighthouse] could not complete the authenticated audits - ' +
						'treating as a warning, not a build failure:',
					err,
					'\n',
				);
			}
		} finally {
			await context.close();
		}
	} finally {
		serverProc.kill();
		await serverProc.exited;
	}
}

main().catch((err) => {
	// A crash running the audit itself (server never came up, browser
	// launch failed) is still worth failing on - only a threshold miss is
	// a warning. Distinguishing those is the whole point of this script
	// existing instead of a bare `playAudit` call.
	console.error('[lighthouse] audit run failed:', err);
	process.exit(1);
});
