import DOMPurify from 'dompurify';
import { marked } from 'marked';

// CHANGELOG.md is trusted repo content, but it still crosses a network
// fetch at runtime (+page.ts) rather than being compiled at build time -
// design.md's "Changelog rendering" decision (#249) - so the parsed
// output is sanitized before it ever reaches {@html}.
//
// The file's own leading `# Changelog` is dropped before parsing - the
// changelog route already renders its own page <h1>, and rendering this
// one too duplicated it on the page (alrayyes/hush-hush#315).
export function renderChangelog(markdown: string): string {
	const withoutLeadingTitle = markdown.replace(/^#[ \t]+[^\n]*\n+/, '');
	return DOMPurify.sanitize(
		marked.parse(withoutLeadingTitle, { async: false }),
	);
}
