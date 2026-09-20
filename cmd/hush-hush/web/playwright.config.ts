import { defineConfig, devices } from '@playwright/test';

// Runs against the real built output (adapter-static's build/), embedded
// into the real Go binary by e2e/server.sh - a bare `vite preview` 500s on
// every route, since the app has no SSR and every page's own +layout.ts
// fetches GET /healthz at load, which only the Go binary answers.
// `bun run build` has to run before this, same as CI's own `web` job order.
export default defineConfig({
	testDir: 'e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	reporter: 'list',
	use: {
		baseURL: 'http://localhost:4173',
	},
	webServer: {
		command: 'bash e2e/server.sh',
		url: 'http://localhost:4173',
		reuseExistingServer: !process.env.CI,
	},
	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] },
		},
	],
});
