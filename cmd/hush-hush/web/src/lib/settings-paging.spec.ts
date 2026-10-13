import { describe, expect, it } from 'vitest';
import {
	listPageHref,
	parseListPage,
	SETTINGS_PAGE_SIZE,
} from './settings-paging';

// alrayyes/Hush-Hush#896: the three Settings lists page on their own, 10 at a
// time. Each keeps its page in the address under its own parameter, so a reload
// keeps all three positions and paging one list leaves the others where they are.

const params = (query: string) => new URLSearchParams(query);

describe('SETTINGS_PAGE_SIZE', () => {
	it('is 10', () => {
		expect(SETTINGS_PAGE_SIZE).toBe(10);
	});
});

describe('parseListPage', () => {
	it('is page 1 when the list has no parameter', () => {
		expect(parseListPage(params(''), 'tokens')).toBe(1);
	});

	it('reads each list from its own parameter', () => {
		const query = params('passkey_page=2&token_page=3&consumer_token_page=4');

		expect(parseListPage(query, 'passkeys')).toBe(2);
		expect(parseListPage(query, 'tokens')).toBe(3);
		expect(parseListPage(query, 'consumerTokens')).toBe(4);
	});

	it.each(['0', '-2', '1.5', 'abc', ''])('is page 1 for %j', (value) => {
		expect(parseListPage(params(`token_page=${value}`), 'tokens')).toBe(1);
	});
});

describe('listPageHref', () => {
	it('links to a page of one list', () => {
		expect(listPageHref(params(''), 'tokens', 2)).toBe(
			'/settings?token_page=2',
		);
	});

	it('leaves the other lists and the consumer filter as they are', () => {
		expect(
			listPageHref(
				params('consumer=homelab&consumer_token_page=2'),
				'tokens',
				3,
			),
		).toBe('/settings?consumer=homelab&consumer_token_page=2&token_page=3');
	});

	it.each([
		['passkeys', 'passkey_page'],
		['consumerTokens', 'consumer_token_page'],
	] as const)("uses %s's own parameter, %s", (list, name) => {
		expect(listPageHref(params(''), list, 2)).toBe(`/settings?${name}=2`);
	});

	it('drops the parameter for page 1', () => {
		expect(
			listPageHref(params('token_page=3&passkey_page=2'), 'tokens', 1),
		).toBe('/settings?passkey_page=2');
	});

	it('is the bare path when nothing is set', () => {
		expect(listPageHref(params('token_page=2'), 'tokens', 1)).toBe('/settings');
	});
});
