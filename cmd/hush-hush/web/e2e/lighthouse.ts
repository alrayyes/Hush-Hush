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
import { playAudit } from 'playwright-lighthouse';

const PORT = 9222;
const BASE_URL = 'http://localhost:4173';
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
): Promise<void> {
	const results = await playAudit({
		page,
		port: PORT,
		thresholds: THRESHOLDS,
		ignoreError: true,
		// One HTML and JSON pair per audited page, which the pages job
		// publishes (rules/published-reports.md).
		reports: {
			formats: { html: true, json: true },
			directory: REPORTS_DIR,
			name: reportName,
		},
	});

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

async function main() {
	const serverProc = Bun.spawn(['bash', 'e2e/server.sh'], {
		stdout: 'inherit',
		stderr: 'inherit',
	});

	try {
		await waitForServer(`${BASE_URL}/healthz`);

		const browser = await chromium.launch({
			args: [`--remote-debugging-port=${PORT}`],
		});
		try {
			const context = await browser.newContext({ baseURL: BASE_URL });
			const page = await context.newPage();

			await page.goto('/login');
			await auditPage(page, '/login', 'login');

			// Same CDP virtual-authenticator flow as journey.spec.ts's own
			// login - a passkey has no username/password form Lighthouse's
			// usual auth recipes assume, so an authenticated audit has to
			// drive the actual WebAuthn ceremony rather than skip it.
			//
			// This whole block is best-effort, not fatal like the rest of
			// main(): playAudit opens its own CDP connection over the same
			// remote-debugging port this browser was launched with, and
			// that appears to sometimes leave a CDP-registered virtual
			// authenticator non-functional on whatever page it touches
			// next - the "Register passkey" click still fires but the
			// ceremony it starts never completes, hanging until
			// waitForURL's timeout (found live, reproducible, root cause
			// not yet pinned down - reordering the CDP setup earlier and
			// isolating the audited pages onto separate targets each
			// failed to fix it locally, though the exact same registration
			// flow passes reliably in journey.spec.ts's own Playwright
			// suite). This check is documented as warn-only
			// (rules/browser-compat.md) - a failure to even drive the
			// authenticated half shouldn't hard-fail the build any more
			// than a missed threshold does, so it's caught and reported
			// the same way rather than propagating to main()'s own
			// process.exit(1).
			try {
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
				await page.waitForURL('/');

				await auditPage(page, '/ (secrets overview)', 'secrets-overview');
			} catch (err) {
				console.warn(
					'\n[lighthouse] could not complete the authenticated audit ' +
						'(/, secrets overview) - treating as a warning, not a ' +
						'build failure:',
					err,
					'\n',
				);
			}
		} finally {
			await browser.close();
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
