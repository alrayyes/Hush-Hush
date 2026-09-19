import { listObjects, queryAuditLog } from '$lib/api';
import { attributionByObject } from '$lib/attribution';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ depends }) => {
	// app:objects is invalidated after every create/edit/delete, so the
	// overview reflects the change immediately - web-ui/spec.md's
	// scenarios all say "afterward", never "after a reload".
	depends('app:objects');

	const [objects, auditLog] = await Promise.all([
		listObjects(),
		queryAuditLog(),
	]);

	return {
		objects,
		attribution: attributionByObject(auditLog),
	};
};
