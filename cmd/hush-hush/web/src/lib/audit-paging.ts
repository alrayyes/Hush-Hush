export type AuditOrder = 'asc' | 'desc';

// cursorParams is the cursor for the page after the one whose last entry has
// this id. GET /audit-log's `after` means "newer than this id" in either
// order, so a newest-first log pages onward with `before` instead
// (alrayyes/hush-hush#756). No cursor is the first page.
export function cursorParams(
	order: AuditOrder,
	cursor: number | undefined,
): { after?: number; before?: number } {
	if (cursor === undefined) return {};

	return order === 'desc' ? { before: cursor } : { after: cursor };
}

export function flipOrder(order: AuditOrder): AuditOrder {
	return order === 'desc' ? 'asc' : 'desc';
}
