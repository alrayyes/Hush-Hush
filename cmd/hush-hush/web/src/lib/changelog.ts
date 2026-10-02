// CHANGELOG.md is release-please output: a `## [version](compare-url) (date)`
// heading per release, `### Features` / `### Bug Fixes` sections under it,
// and one `* **scope:** summary ([#N](issue-url)) ([sha](commit-url))` line
// per change, sometimes with a trailing `, closes [#N](issue-url)`. The
// changelog page shows a card per release instead of one long document, so
// this turns the file into data. Entries are plain text, never HTML, so
// nothing here needs sanitizing.

export interface ChangelogRef {
	label: string;
	url: string;
}

export interface ChangelogEntry {
	badge: string;
	scope?: string;
	text: string;
	refs: ChangelogRef[];
}

export interface ChangelogRelease {
	version: string;
	date: string;
	entries: ChangelogEntry[];
}

// release-please's own section names. An unknown one still gets a badge
// (its heading, uppercased) rather than being dropped.
const SECTION_BADGES: Record<string, string> = {
	'⚠ BREAKING CHANGES': 'BREAKING',
	Features: 'FEATURE',
	'Bug Fixes': 'FIX',
	'Performance Improvements': 'PERF',
	'Code Refactoring': 'REFACTOR',
	Reverts: 'REVERT',
};

const RELEASE_HEADING =
	/^##\s+\[?(\d+\.\d+\.\d+[^\]\s)]*)\]?(?:\([^)]*\))?\s*\((\d{4}-\d{2}-\d{2})\)/;
const SCOPE = /^\* \*\*([^*]+):\*\*\s*/;
const MARKDOWN_LINK = /\[([^\]]+)\]\(([^)]+)\)/g;

function badgeFor(heading: string): string {
	return SECTION_BADGES[heading] ?? heading.toUpperCase();
}

// The issue/PR links an entry carries, in order of appearance. Commit links
// are noise on a phone and are dropped.
function issueRefs(line: string): ChangelogRef[] {
	const refs: ChangelogRef[] = [];

	for (const [, label, url] of line.matchAll(MARKDOWN_LINK)) {
		if (label.startsWith('#') && !refs.some((r) => r.label === label)) {
			refs.push({ label, url });
		}
	}

	return refs;
}

function parseEntry(line: string, badge: string): ChangelogEntry {
	const scope = line.match(SCOPE)?.[1];
	const body = line.replace(SCOPE, '').replace(/^\* /, '');

	// Drop every trailing link group - "([#N](url))", "([sha](url))" and
	// ", closes [#N](url)" - then flatten any link left in the sentence to
	// its label.
	const text = body
		.replace(/\s*\(\[[^\]]+\]\([^)]+\)\)/g, '')
		.replace(/,\s*closes\s+\[[^\]]+\]\([^)]+\)/g, '')
		.replace(MARKDOWN_LINK, '$1')
		.trim();

	return {
		badge: scope === 'security' ? 'SECURITY' : badge,
		scope,
		text,
		refs: issueRefs(line),
	};
}

export function parseChangelog(markdown: string): ChangelogRelease[] {
	const releases: ChangelogRelease[] = [];
	let badge = '';

	for (const line of markdown.split('\n')) {
		const heading = line.match(RELEASE_HEADING);

		if (heading) {
			releases.push({ version: heading[1], date: heading[2], entries: [] });
			badge = '';
		} else if (line.startsWith('### ')) {
			badge = badgeFor(line.slice(4).trim());
		} else if (line.startsWith('* ') && releases.length > 0) {
			releases[releases.length - 1].entries.push(parseEntry(line, badge));
		}
	}

	return releases;
}
