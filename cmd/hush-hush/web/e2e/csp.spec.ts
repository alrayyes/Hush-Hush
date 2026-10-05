import { expect, test } from '@playwright/test';

// alrayyes/hush-hush#660: the server sends a Content-Security-Policy that lets
// only this origin's scripts, and the build's own inline scripts, run. The
// whole e2e suite already runs under it, so a blocked script would break the
// app and fail those tests; this one names the policy and fails on any
// violation the browser reports, including ones that don't break anything yet.
for (const path of ['/login', '/changelog']) {
	test(`${path} is served under a strict CSP and the browser reports no violation`, async ({
		page,
	}) => {
		await page.addInitScript(() => {
			const w = window as unknown as { __cspViolations: string[] };
			w.__cspViolations = [];
			document.addEventListener('securitypolicyviolation', (event) => {
				w.__cspViolations.push(
					`${event.violatedDirective} blocked ${event.blockedURI || 'inline'}`,
				);
			});
		});

		const response = await page.goto(path);
		const policy = response?.headers()['content-security-policy'] ?? '';

		expect(policy).toContain("default-src 'self'");
		expect(policy).toContain("frame-ancestors 'none'");
		expect(policy).not.toContain('unsafe-eval');
		expect(policy.match(/script-src[^;]*/)?.[0]).not.toContain('unsafe-inline');

		await page.waitForLoadState('networkidle');

		const violations = await page.evaluate(
			() =>
				(window as unknown as { __cspViolations: string[] }).__cspViolations,
		);
		expect(violations).toEqual([]);
	});
}
