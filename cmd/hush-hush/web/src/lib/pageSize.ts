// Rows per page, for the two lists that let the reader choose
// (alrayyes/Hush-Hush#897): Secrets and the audit log. The size is a URL
// parameter, ?size=, beside ?page=, so a reload or a shared link keeps it.

export const SECRETS_SIZES = [20, 50, 100] as const;
export const AUDIT_SIZES = [25, 50, 100] as const;

// parsePageSize reads ?size=. Only an offered size counts: anything else, or no
// size at all, is the first (smallest, default) option.
export function parsePageSize(
	params: Pick<URLSearchParams, 'get'>,
	options: readonly number[],
): number {
	const size = Number(params.get('size'));

	return options.includes(size) ? size : options[0];
}

// A list that knows its total shows the selector once it has more rows than the
// smallest option: below that, no size could change what is on screen. It stays
// when one page holds everything, so a size picked too big can be undone.
export function showSizeSelector(
	total: number,
	options: readonly number[],
): boolean {
	return total > options[0];
}

// A cursor list has no total. It shows the selector when another page exists at
// this size, or when the size isn't the smallest option, so someone who picked
// 100 can always pick fewer again.
export function showCursorSizeSelector(
	size: number,
	options: readonly number[],
	hasPrevious: boolean,
	hasMore: boolean,
): boolean {
	return hasPrevious || hasMore || size !== options[0];
}
