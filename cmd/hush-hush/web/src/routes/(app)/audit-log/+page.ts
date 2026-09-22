import { queryAuditLog, queryAuditLogFilterOptions } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	const [entries, filterOptions] = await Promise.all([
		queryAuditLog(),
		queryAuditLogFilterOptions(),
	]);

	return { entries, filterOptions };
};
