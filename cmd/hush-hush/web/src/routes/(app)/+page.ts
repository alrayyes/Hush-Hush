import { getOwnerIdentity, listObjects } from '#lib/api.js';
import { parseObjectFilters, unionObjects } from '#lib/object-filters.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ depends, url }) => {
	// app:objects is invalidated after every create/edit/delete, so the
	// overview reflects the change immediately - web-ui/spec.md's
	// scenarios all say "afterward", never "after a reload".
	depends('app:objects');

	// The filters in the URL. Several consumers are several API calls (the
	// server filters on one), unioned here: an object matches when it is used
	// by any of them. The rest (creator, dates) filter the loaded list in the
	// page, since GET /objects has no query for them.
	const filters = parseObjectFilters(url.searchParams);
	const lists =
		filters.usedBy.length === 0
			? [listObjects()]
			: filters.usedBy.map((consumer) => listObjects(consumer));

	const [objects, ownerPublicKey] = await Promise.all([
		Promise.all(lists).then(unionObjects),
		getOwnerIdentity(),
	]);

	return {
		objects,
		filters,
		// Undefined until the account has completed a first registration -
		// the create/edit dialog's owner-recipient checkbox only renders
		// once this is set (tasks.md's 5.2).
		ownerPublicKey,
	};
};
