import {
	ApiError,
	getObjectValue,
	listConsumerDirectory,
	listObjects,
	queryAuditLog,
} from '$lib/api';
import { recentEntriesForObject } from '$lib/audit-recent';
import type { PageLoad } from './$types';

// app:objects is invalidated by the secrets mutations, so this page
// refreshes the same way the overview does.
export const load: PageLoad = async ({ depends, params }) => {
	depends('app:objects');

	const slug = params.slug;
	const [objects, directory] = await Promise.all([
		listObjects(),
		listConsumerDirectory(),
	]);
	const usedBy = objects.find((o) => o.slug === slug)?.used_by ?? [];

	// Awaited before the value is fetched so this page's own read isn't
	// one of the entries it shows.
	const events = await recentEntriesForObject(queryAuditLog, slug, 3);

	let value: string;
	try {
		value = await getObjectValue(slug);
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			return { slug, notFound: true as const };
		}
		throw err;
	}

	return {
		slug,
		notFound: false as const,
		value,
		events,
		consumers: usedBy.map((name) => ({
			name,
			publicKey: directory.find((c) => c.name === name)?.public_key,
		})),
	};
};
