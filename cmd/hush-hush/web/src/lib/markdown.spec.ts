// @vitest-environment jsdom
//
// DOMPurify (used by renderChangelog) needs a real `window`/`document` to
// sanitize against; the project's default `server`/node test environment
// has neither, but the app itself only ever calls this client-side (SPA,
// `ssr = false`), so this override is test-only.
import { describe, expect, it } from 'vitest';
import { renderChangelog } from './markdown';

describe('renderChangelog', () => {
	it('renders a heading and a list item as HTML', () => {
		const html = renderChangelog('# Title\n\n- one\n- two\n');

		expect(html).toContain('<h1>Title</h1>');
		expect(html).toContain('<li>one</li>');
		expect(html).toContain('<li>two</li>');
	});

	it('strips a script tag from the input', () => {
		const html = renderChangelog('# Title\n\n<script>alert(1)</script>\n');

		expect(html).not.toContain('<script>');
	});
});
