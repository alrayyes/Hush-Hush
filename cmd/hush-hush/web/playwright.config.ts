import { defineConfig, devices } from '@playwright/test';
import { SESSION_FILE } from './e2e/session';

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
	// The journey registers the one passkey the server accepts and signs in, so it
	// goes first and saves the session. The signed-in specs start from that
	// session and run once it passes, and the sign-out check runs last, since
	// signing out ends the session for everything before it. Specs that need no
	// session run beside all of them. A signed-in spec shares the journey's
	// database, so it works out its own counts and removes what it adds.
	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] },
			testIgnore: ['journey.spec.ts', 'signed-in/**', 'signed-out/**'],
		},
		{
			name: 'journey',
			use: { ...devices['Desktop Chrome'] },
			testMatch: 'journey.spec.ts',
		},
		{
			name: 'signed-in',
			use: { ...devices['Desktop Chrome'], storageState: SESSION_FILE },
			testMatch: 'signed-in/**/*.spec.ts',
			dependencies: ['journey'],
		},
		{
			name: 'signed-out',
			use: { ...devices['Desktop Chrome'], storageState: SESSION_FILE },
			testMatch: 'signed-out/**/*.spec.ts',
			dependencies: ['signed-in'],
		},
	],
});
