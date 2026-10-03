import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

// alrayyes/hush-hush#578 and #579: Tailwind v4 has one `--spacing` step and no
// `--spacing-N` tokens. A var() on a name that doesn't exist makes the whole
// declaration compute to nothing, with no error anywhere. The stylesheet used
// to carry eight of them.
describe('app.css', () => {
	it('uses no var(--spacing-N) token, which does not exist', () => {
		const css = readFileSync(new URL('./app.css', import.meta.url), 'utf8');
		const lines = css
			.split('\n')
			.map((text, index) => ({ line: index + 1, text }))
			.filter(({ text }) => /var\(--spacing-\d/.test(text))
			.map(({ line, text }) => `${line}: ${text.trim()}`);

		expect(lines).toEqual([]);
	});
});
