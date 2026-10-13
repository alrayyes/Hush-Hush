import { queryAuditLog, queryAuditLogFilterOptions } from '#lib/api.js';
import { splitPage } from '#lib/audit-paging.js';
import { AUDIT_SIZES, parsePageSize } from '#lib/pageSize.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url }) => {
	const size = parsePageSize(url.searchParams, AUDIT_SIZES);

	// One row more than a page, to learn whether an older page exists.
	const [rows, filterOptions] = await Promise.all([
		queryAuditLog({ order: 'desc', limit: size + 1 }),
		queryAuditLogFilterOptions(),
	]);
	const { entries, hasMore } = splitPage(rows, size);

	return { entries, hasMore, size, filterOptions };
};
