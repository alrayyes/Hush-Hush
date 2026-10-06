// Pure URL-building and pagination-math helpers for the consumers
// directory page (alrayyes/hush-hush#252) - kept out of the .svelte file
// so they're unit-testable with vitest, the same split this project
// already uses for attribution.ts and audit-export.ts: the page itself
// is exercised by the Playwright journey test instead.

export const CONSUMERS_PAGE_SIZE = 20;

export interface ConsumersQuery {
	page: number;
	q: string;
}

// parseConsumersQuery reads the consumers page's own page/q state back
// out of a URL - the single source of truth for both, so a reload or a
// shared link reproduces the same filtered page.
export function parseConsumersQuery(url: URL): ConsumersQuery {
	const pageParam = Number(url.searchParams.get('page'));
	const page = Number.isInteger(pageParam) && pageParam > 0 ? pageParam : 1;
	const q = url.searchParams.get('q') ?? '';

	return { page, q };
}

// consumersHref builds the href for a given page/filter combination,
// omitting a default page number or an empty filter rather than writing
// every page link as a needlessly noisy URL.
export function consumersHref(page: number, q: string): string {
	const params = new URLSearchParams();
	if (q !== '') {
		params.set('q', q);
	}
	if (page !== 1) {
		params.set('page', String(page));
	}

	const qs = params.toString();

	return qs ? `/consumers?${qs}` : '/consumers';
}

// totalPages is how many page-number links the directory renders, given
// the API's own total count and the fixed page size - always at least
// one, so an empty or single-page result still shows page 1.
export function totalPages(total: number, pageSize: number): number {
	return Math.max(1, Math.ceil(total / pageSize));
}

// secretsOverviewHref is where selecting a consumer navigates to -
// GET /objects?used_by= already supports exactly this filter
// (design.md's "Selecting a consumer reuses GET /objects?used_by="
// decision), so the directory only needs to link to it with that
// parameter set.
export function secretsOverviewHref(consumer: string): string {
	return `/?used_by=${encodeURIComponent(consumer)}`;
}

// tokensHref is where a consumer's token count links to - the Settings
// page's own Consumer tokens table, filtered client-side to this consumer
// (design.md's "Filtering lives in settings/+page.svelte" decision, since
// GET /consumer-tokens has no server-side per-consumer filter to link a
// query parameter straight into).
export function tokensHref(consumer: string): string {
	return `/settings?consumer=${encodeURIComponent(consumer)}`;
}

// truncateKey shortens a registered age public key for display, keeping
// enough of each end to recognise it by. A key of 16 characters or fewer
// is returned as is, since "truncating" it would make it longer.
export function truncateKey(key: string): string {
	if (key.length <= 16) {
		return key;
	}

	return `${key.slice(0, 10)}…${key.slice(-6)}`;
}

// A variant can have a hundred consumers, so a row names two and counts the
// rest; the full list opens in a dialog with a filter box.
const CONSUMERS_SHOWN_INLINE = 2;

// Above this many consumers a list gets a filter box.
export const CONSUMER_LIST_FILTER_MIN = 10;

export function summariseConsumers(consumers: string[] | undefined): {
	shown: string[];
	more: number;
} {
	const all = consumers ?? [];

	return {
		shown: all.slice(0, CONSUMERS_SHOWN_INLINE),
		more: Math.max(0, all.length - CONSUMERS_SHOWN_INLINE),
	};
}

export function filterConsumers(consumers: string[], query: string): string[] {
	const needle = query.trim().toLowerCase();
	if (needle === '') return consumers;

	return consumers.filter((name) => name.toLowerCase().includes(needle));
}

// variantLabel says which variant of a name a dialog is about. A name with
// one variant needs no label; with several, it is the variant's consumers,
// summarised the way a row summarises them so a hundred of them stay short.
export function variantLabel(
	consumers: string[] | undefined,
	variantCount: number,
): string {
	if (variantCount < 2) return '';
	const { shown, more } = summariseConsumers(consumers);
	if (shown.length === 0) return 'Variant with no consumers';

	return `Variant for ${shown.join(', ')}${more > 0 ? ` and ${more} more` : ''}`;
}
