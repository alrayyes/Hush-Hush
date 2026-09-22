// @vitest-environment jsdom
//
// DOMPurify (used by renderChangelog) needs a real `window`/`document` to
// sanitize against; the project's default `server`/node test environment
// has neither, but the app itself only ever calls this client-side (SPA,
// `ssr = false`), so this override is test-only.
import { describe, expect, it } from 'vitest';
import { renderChangelog } from './markdown';

describe('renderChangelog', () => {
	it('drops the leading top-level heading, since the changelog page renders its own <h1>', () => {
		const html = renderChangelog(
			'# Changelog\n\n## [1.0.0](https://example.com) (2026-01-01)\n\n### Features\n\n- one\n',
		);

		expect(html).not.toContain('<h1>');
		expect(html).toContain('<li>one</li>');
	});

	it('renders a non-leading heading and a list item as HTML', () => {
		const html = renderChangelog('# Changelog\n\n## Title\n\n- one\n- two\n');

		expect(html).toContain('<h2>Title</h2>');
		expect(html).toContain('<li>one</li>');
		expect(html).toContain('<li>two</li>');
	});

	it('strips a script tag from the input', () => {
		const html = renderChangelog('# Title\n\n<script>alert(1)</script>\n');

		expect(html).not.toContain('<script>');
	});
});
