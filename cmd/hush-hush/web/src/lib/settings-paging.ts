// The Settings page's three lists page on their own, 10 rows at a time
// (alrayyes/Hush-Hush#896). Each keeps its page in the address under its own
// parameter, so a reload keeps all three positions and paging one list leaves
// the others where they are.

export const SETTINGS_PAGE_SIZE = 10;

export type SettingsList = 'passkeys' | 'tokens' | 'consumerTokens';

const PARAM: Record<SettingsList, string> = {
	passkeys: 'passkey_page',
	tokens: 'token_page',
	consumerTokens: 'consumer_token_page',
};

// parseListPage reads one list's page. Anything that isn't a whole number from 1
// up is the first page; a page past the end is clamped where it is shown.
export function parseListPage(
	params: Pick<URLSearchParams, 'get'>,
	list: SettingsList,
): number {
	const page = Number(params.get(PARAM[list]));

	return Number.isInteger(page) && page >= 1 ? page : 1;
}

// listPageHref is the link to a page of one list. Everything else in the
// address stays as it is: the consumer filter and the other lists' pages. Page 1
// has no parameter.
export function listPageHref(
	current: Pick<URLSearchParams, 'toString'>,
	list: SettingsList,
	page: number,
): string {
	const params = new URLSearchParams(current.toString());

	if (page > 1) params.set(PARAM[list], String(page));
	else params.delete(PARAM[list]);

	const query = params.toString();

	return query === '' ? '/settings' : `/settings?${query}`;
}
