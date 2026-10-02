import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { imageTag, parseChangelog, releaseUrl } from './changelog';

const base = 'https://github.com/alrayyes/Hush-Hush';

describe('parseChangelog', () => {
	it('reads the version and date from a linked release heading', () => {
		const releases = parseChangelog(
			`## [2.43.1](${base}/compare/v2.43.0...v2.43.1) (2026-09-28)\n\n### Bug Fixes\n\n* **web-ui:** a fix ([#1](${base}/issues/1))\n`,
		);

		expect(releases).toHaveLength(1);
		expect(releases[0].version).toBe('2.43.1');
		expect(releases[0].date).toBe('2026-09-28');
	});

	it('reads a release heading with no compare link', () => {
		const releases = parseChangelog(
			'## 0.0.1 (2026-08-28)\n\n### Features\n\n* first thing\n',
		);

		expect(releases[0].version).toBe('0.0.1');
		expect(releases[0].date).toBe('2026-08-28');
	});

	it('keeps releases in file order, newest first', () => {
		const releases = parseChangelog(
			'## [2.0.0](x) (2026-02-01)\n\n### Features\n\n* b\n\n## [1.0.0](x) (2026-01-01)\n\n### Features\n\n* a\n',
		);

		expect(releases.map((r) => r.version)).toEqual(['2.0.0', '1.0.0']);
	});

	it('maps each section heading to a badge word', () => {
		const releases = parseChangelog(
			[
				'## [1.0.0](x) (2026-01-01)',
				'### ⚠ BREAKING CHANGES',
				'* **api:** one',
				'### Features',
				'* two',
				'### Bug Fixes',
				'* three',
				'### Performance Improvements',
				'* four',
				'### Code Refactoring',
				'* five',
				'### Reverts',
				'* six',
				'### Documentation',
				'* seven',
			].join('\n'),
		);

		expect(releases[0].entries.map((e) => e.badge)).toEqual([
			'BREAKING',
			'FEATURE',
			'FIX',
			'PERF',
			'REFACTOR',
			'REVERT',
			'DOCUMENTATION',
		]);
	});

	it('splits an entry into scope, text and its issue links, dropping the commit link', () => {
		const [release] = parseChangelog(
			`## [1.0.0](x) (2026-01-01)\n\n### Bug Fixes\n\n* **web-ui:** stop committing real build output as the go:embed placeholder ([#472](${base}/issues/472)) ([cb97670](${base}/commit/cb9767063371bc801bf67521004b0c3b950ac2bb))\n`,
		);

		expect(release.entries[0]).toMatchObject({
			scope: 'web-ui',
			text: 'stop committing real build output as the go:embed placeholder',
			refs: [{ label: '#472', url: `${base}/issues/472` }],
		});
	});

	it('keeps a closes link as a second ref', () => {
		const [release] = parseChangelog(
			`## [1.0.0](x) (2026-01-01)\n\n### Bug Fixes\n\n* **web-ui:** table spacing ([#458](${base}/issues/458)) ([2270492](${base}/commit/227049261121d22b8993a387a142210bb82094dc)), closes [#457](${base}/issues/457)\n`,
		);

		expect(release.entries[0].text).toBe('table spacing');
		expect(release.entries[0].refs.map((r) => r.label)).toEqual([
			'#458',
			'#457',
		]);
	});

	it('handles an entry with no scope', () => {
		const [release] = parseChangelog(
			`## [1.0.0](x) (2026-01-01)\n\n### Bug Fixes\n\n* use RELEASE_TOKEN instead of a GitHub App installation token ([#416](${base}/issues/416)) ([ee73990](${base}/commit/ee73990d8881db2f4745184972eeddbe4e4809f7))\n`,
		);

		expect(release.entries[0].scope).toBeUndefined();
		expect(release.entries[0].text).toBe(
			'use RELEASE_TOKEN instead of a GitHub App installation token',
		);
	});

	it('flattens a markdown link inside the text to its label', () => {
		const [release] = parseChangelog(
			`## [1.0.0](x) (2026-01-01)\n\n### Features\n\n* **docs:** see [the guide](https://example.com/guide) for more ([#9](${base}/issues/9))\n`,
		);

		expect(release.entries[0].text).toBe('see the guide for more');
	});

	it('gives a security-scoped entry the SECURITY badge whatever its section', () => {
		const [release] = parseChangelog(
			`## [1.0.0](x) (2026-01-01)\n\n### Bug Fixes\n\n* **security:** reject a bad token ([#5](${base}/issues/5))\n\n* **api:** an ordinary fix ([#6](${base}/issues/6))\n`,
		);

		expect(release.entries.map((e) => e.badge)).toEqual(['SECURITY', 'FIX']);
	});

	it('ignores the page title and any preamble before the first release', () => {
		const releases = parseChangelog(
			'# Changelog\n\nAll notable changes.\n\n## [1.0.0](x) (2026-01-01)\n\n### Features\n\n* a\n',
		);

		expect(releases).toHaveLength(1);
	});

	it('returns nothing for an empty file', () => {
		expect(parseChangelog('')).toEqual([]);
	});

	it("parses this repo's own CHANGELOG.md without dropping a release or an entry", () => {
		const text = readFileSync(
			new URL('../../../../../CHANGELOG.md', import.meta.url),
			'utf8',
		);
		const releaseHeadings = text.match(/^## /gm)?.length ?? 0;
		const entryLines = text.match(/^\* /gm)?.length ?? 0;

		const releases = parseChangelog(text);

		expect(releases).toHaveLength(releaseHeadings);
		expect(releases.flatMap((r) => r.entries)).toHaveLength(entryLines);
		for (const entry of releases.flatMap((r) => r.entries)) {
			expect(entry.text).not.toBe('');
			expect(entry.badge).toMatch(/^[A-Z ]+$/);
		}
	});
});

describe('imageTag', () => {
	it('is the ghcr.io image for a version, with no v prefix', () => {
		expect(imageTag('2.46.0')).toBe('ghcr.io/alrayyes/hush-hush:2.46.0');
	});
});

describe('releaseUrl', () => {
	it('links the GitHub release page, whose tag does carry the v prefix', () => {
		expect(releaseUrl('2.46.0')).toBe(
			'https://github.com/alrayyes/Hush-Hush/releases/tag/v2.46.0',
		);
	});
});
