import type { PageLoad } from './$types';

// CHANGELOG.md ships as a static asset (the "copy-changelog" build
// script), fetched as plain text rather than through the JSON API - it's
// the same file `git log`/the forge render, and web-ui/spec.md's own
// "The changelog page reflects the real changelog" requirement is about
// that file's actual content, not a paraphrase of it.
export const load: PageLoad = async ({ fetch }) => {
	const res = await fetch('/CHANGELOG.md');
	const text = res.ok ? await res.text() : '';

	return { changelog: text };
};
