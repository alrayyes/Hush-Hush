import { listConsumersPage, listConsumerTokens } from '#lib/api.js';
import { CONSUMERS_PAGE_SIZE, parseConsumersQuery } from '#lib/consumers.js';
import type { PageLoad } from './$types';

// app:consumers is invalidated after a secret create adds a new
// consumer via ConsumerCombobox, so a visitor who's already on this page
// sees it appear without a manual reload.
export const load: PageLoad = async ({ depends, url }) => {
	depends('app:consumers');

	const { page, q } = parseConsumersQuery(url);
	const [result, consumerTokens] = await Promise.all([
		listConsumersPage({
			q: q || undefined,
			page,
			page_size: CONSUMERS_PAGE_SIZE,
		}),
		listConsumerTokens(),
	]);

	// GET /consumer-tokens has no per-consumer filter to ask for a count
	// from directly (design.md's "Counting lives in consumers/+page.ts"
	// decision), so it's reduced here rather than in the component.
	const tokenCounts = new Map<string, number>();
	for (const token of consumerTokens) {
		tokenCounts.set(token.consumer, (tokenCounts.get(token.consumer) ?? 0) + 1);
	}

	return {
		consumers: result.consumers,
		total: result.total,
		page,
		q,
		tokenCounts,
	};
};
