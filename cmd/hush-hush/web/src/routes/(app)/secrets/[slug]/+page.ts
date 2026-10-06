import {
	ApiError,
	getObjectValue,
	listConsumerDirectory,
	listObjects,
	queryAuditLog,
} from '$lib/api';
import type { PageLoad } from './$types';

// app:objects is invalidated by the secrets mutations, so this page
// refreshes the same way the overview does.
export const load: PageLoad = async ({ depends, params, url }) => {
	depends('app:objects');

	const slug = params.slug;
	const [objects, directory] = await Promise.all([
		listObjects(),
		listConsumerDirectory(),
	]);
	const variants = objects.filter((o) => o.slug === slug);

	// A name can hold several variants (ADR 33), and the API won't pick one
	// for a session. Without ?id= there is nothing to show but the choice.
	const requestedId = url.searchParams.get('id');
	if (!requestedId && variants.length > 1) {
		return {
			slug,
			notFound: false as const,
			chooseVariant: true as const,
			variants,
		};
	}

	const variant = requestedId
		? variants.find((o) => o.id === requestedId)
		: variants[0];
	if (!variant) {
		return { slug, notFound: true as const };
	}
	const usedBy = variant.used_by ?? [];

	// Awaited before the value is fetched so this page's own read isn't
	// one of the entries it shows. A name's entries are about the name, so
	// with several variants a wider window is read and narrowed to this
	// one; an entry from before variant_id was recorded can't be placed.
	const several = variants.length > 1;
	const entries = await queryAuditLog({
		object_id: slug,
		order: 'desc',
		limit: several ? 50 : 3,
	});
	const events = several
		? entries.filter((e) => e.variant_id === variant.id).slice(0, 3)
		: entries;

	let value: string;
	try {
		value = await getObjectValue(slug, variant.id);
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			return { slug, notFound: true as const };
		}
		throw err;
	}

	return {
		slug,
		notFound: false as const,
		chooseVariant: false as const,
		id: variant.id,
		value,
		events,
		consumers: usedBy.map((name) => ({
			name,
			publicKey: directory.find((c) => c.name === name)?.public_key,
		})),
	};
};
