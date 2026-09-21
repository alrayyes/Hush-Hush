import { listConsumersPage } from '$lib/api';
import { CONSUMERS_PAGE_SIZE, parseConsumersQuery } from '$lib/consumers';
import type { PageLoad } from './$types';

// app:consumers is invalidated after a secret create adds a new
// consumer via ConsumerCombobox, so a visitor who's already on this page
// sees it appear without a manual reload.
export const load: PageLoad = async ({ depends, url }) => {
	depends('app:consumers');

	const { page, q } = parseConsumersQuery(url);
	const result = await listConsumersPage({
		q: q || undefined,
		page,
		page_size: CONSUMERS_PAGE_SIZE,
	});

	return {
		consumers: result.consumers,
		total: result.total,
		page,
		q,
	};
};
