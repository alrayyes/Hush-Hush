import { getOwnerIdentity, listObjects } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ depends, url }) => {
	// app:objects is invalidated after every create/edit/delete, so the
	// overview reflects the change immediately - web-ui/spec.md's
	// scenarios all say "afterward", never "after a reload".
	depends('app:objects');

	// Set when navigating here from the consumers directory
	// (alrayyes/hush-hush#252) - undefined is listObjects's own "no
	// restriction" default otherwise.
	const usedByFilter = url.searchParams.get('used_by') ?? undefined;

	const [objects, ownerPublicKey] = await Promise.all([
		listObjects(usedByFilter),
		getOwnerIdentity(),
	]);

	return {
		objects,
		usedByFilter,
		// Undefined until the account has completed a first registration -
		// the create/edit dialog's owner-recipient checkbox only renders
		// once this is set (tasks.md's 5.2).
		ownerPublicKey,
	};
};
