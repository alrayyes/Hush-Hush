import { queryAuditLog, queryAuditLogFilterOptions } from '#lib/api.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	const [entries, filterOptions] = await Promise.all([
		queryAuditLog({ order: 'desc' }),
		queryAuditLogFilterOptions(),
	]);

	return { entries, filterOptions };
};
