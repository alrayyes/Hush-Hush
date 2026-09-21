import DOMPurify from 'dompurify';
import { marked } from 'marked';

// CHANGELOG.md is trusted repo content, but it still crosses a network
// fetch at runtime (+page.ts) rather than being compiled at build time -
// design.md's "Changelog rendering" decision (#249) - so the parsed
// output is sanitized before it ever reaches {@html}.
export function renderChangelog(markdown: string): string {
	return DOMPurify.sanitize(marked.parse(markdown, { async: false }));
}
