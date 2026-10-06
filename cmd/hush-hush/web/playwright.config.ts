import { defineConfig, devices } from '@playwright/test';

// Runs against the real built output (adapter-static's build/), embedded
// into the real Go binary by e2e/server.sh - a bare `vite preview` 500s on
// every route, since the app has no SSR and every page's own +layout.ts
// fetches GET /healthz at load, which only the Go binary answers.
// `bun run build` has to run before this, same as CI's own `web` job order.
// E2E_PORT picks the server's port (e2e/server.sh reads the same variable),
// so two runs on one machine don't share, or kill, each other's server.
const baseURL = `http://localhost:${process.env.E2E_PORT ?? '4173'}`;

export default defineConfig({
	testDir: 'e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	// CI also writes JUnit, which the pages job publishes next to the other
	// test results (rules/published-reports.md).
	reporter: process.env.CI
		? [['list'], ['junit', { outputFile: 'reports/e2e.xml' }]]
		: 'list',
	use: {
		baseURL,
	},
	webServer: {
		command: 'bash e2e/server.sh',
		url: baseURL,
		reuseExistingServer: !process.env.CI,
	},
	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] },
		},
	],
});
