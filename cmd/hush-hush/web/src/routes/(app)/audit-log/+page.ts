import { queryAuditLog } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	const entries = await queryAuditLog();

	return { entries };
};
