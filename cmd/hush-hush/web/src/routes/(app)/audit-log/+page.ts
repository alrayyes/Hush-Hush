import { queryAuditLog, queryAuditLogFilterOptions } from '#lib/api.js';
import { AUDIT_PAGE_SIZE, splitPage } from '#lib/audit-paging.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	// One row more than a page, to learn whether an older page exists.
	const [rows, filterOptions] = await Promise.all([
		queryAuditLog({ order: 'desc', limit: AUDIT_PAGE_SIZE + 1 }),
		queryAuditLogFilterOptions(),
	]);
	const { entries, hasMore } = splitPage(rows, AUDIT_PAGE_SIZE);

	return { entries, hasMore, filterOptions };
};
