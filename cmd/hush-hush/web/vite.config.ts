import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true,
			},
			adapter: adapter({ fallback: 'index.html' }),
			// The stylesheet is 46 KB, so it ships inside the HTML and the first
			// paint waits on no CSS request (alrayyes/hush-hush#762). The cap
			// is finite on purpose: a stylesheet that outgrows it goes back to
			// a cached file, and Lighthouse's render-blocking insight says so.
			inlineStyleThreshold: 65_536,
		}),
	],
	test: {
		expect: { requireAssertions: true },
		// Only measured when asked for (`--coverage`), as CI's `web` job does:
		// cobertura is the format `rules/published-reports.md` wants, html is
		// the one a person opens.
		coverage: {
			provider: 'v8',
			include: ['src/**/*.{ts,svelte}'],
			reporter: ['text-summary', 'html', 'cobertura'],
		},
		projects: [
			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: ['src/**/*.svelte.{test,spec}.{js,ts}'],
				},
			},
		],
	},
});
