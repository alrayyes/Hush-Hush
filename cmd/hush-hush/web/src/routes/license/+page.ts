import type { PageLoad } from './$types';

// LICENSE ships as a static asset (the "copy-license" build script),
// fetched as plain text - the same file the repo root and the forge's
// own license detection read, not a paraphrase of it. Same pattern as
// changelog/+page.ts's own CHANGELOG.md fetch.
export const load: PageLoad = async ({ fetch }) => {
	const res = await fetch('/LICENSE.txt');
	const text = res.ok ? await res.text() : '';

	return { license: text };
};
