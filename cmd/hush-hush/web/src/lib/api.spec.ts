import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	checkSession,
	deleteObject,
	getObjectValue,
	listConsumerDirectory,
	listObjects,
	listTokens,
	updateObject,
} from './api';
import { CONSUMERS_PAGE_SIZE_MAX, PAGE_LIMIT_MAX } from './api-limits';

describe('listConsumerDirectory', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('asks for pages of the size the API spec caps them at', async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			new Response(JSON.stringify({ consumers: [], total: 0 }), {
				status: 200,
			}),
		);
		vi.stubGlobal('fetch', fetchMock);
		vi.stubGlobal('document', { cookie: '' });

		await listConsumerDirectory();

		const url = new URL(fetchMock.mock.calls[0][0], 'http://localhost');
		expect(url.searchParams.get('page_size')).toBe(
			String(CONSUMERS_PAGE_SIZE_MAX),
		);
	});
});

describe('updateObject', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	function stubFetch() {
		const fetchMock = vi
			.fn()
			.mockImplementation(
				async () =>
					new Response(JSON.stringify({ slug: 'a' }), { status: 200 }),
			);
		vi.stubGlobal('fetch', fetchMock);
		vi.stubGlobal('document', { cookie: '' });

		return fetchMock;
	}

	const sentBody = (fetchMock: ReturnType<typeof stubFetch>) =>
		JSON.parse(fetchMock.mock.calls[0][1].body);

	it('leaves tags out of the request when none are passed', async () => {
		const fetchMock = stubFetch();

		await updateObject('a', 'sealed', ['x']);

		expect(sentBody(fetchMock)).not.toHaveProperty('tags');
	});

	it('sends the new tags, and an empty array to clear them', async () => {
		const fetchMock = stubFetch();

		await updateObject('a', 'sealed', ['x'], false, ['prod']);
		expect(sentBody(fetchMock).tags).toEqual(['prod']);

		fetchMock.mockClear();
		await updateObject('a', 'sealed', ['x'], false, []);
		expect(sentBody(fetchMock).tags).toEqual([]);
	});
});

// alrayyes/hush-hush#677: the list endpoints page on request and #662 will
// make a page the default, so the UI asks for pages itself and keeps going
// until it has every row. The page size is the spec's own cap.
describe('the paged list calls', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	const rows = (count: number) =>
		Array.from({ length: count }, (_, i) => ({ id: String(i) }));

	function stubPages(pages: { rows: unknown[]; total?: number }[]) {
		const fetchMock = vi.fn();
		for (const page of pages) {
			fetchMock.mockResolvedValueOnce(
				new Response(JSON.stringify(page.rows), {
					status: 200,
					headers:
						page.total === undefined
							? {}
							: { 'X-Total-Count': String(page.total) },
				}),
			);
		}
		vi.stubGlobal('fetch', fetchMock);
		vi.stubGlobal('document', { cookie: '' });

		return fetchMock;
	}

	const urls = (fetchMock: ReturnType<typeof stubPages>) =>
		fetchMock.mock.calls.map((call) => String(call[0]));

	it('asks for a page the size the spec caps it at', async () => {
		const fetchMock = stubPages([{ rows: rows(2), total: 2 }]);

		await listTokens();

		expect(urls(fetchMock)).toEqual([
			`/tokens?limit=${PAGE_LIMIT_MAX}&offset=0`,
		]);
	});

	it('keeps asking until it has as many rows as X-Total-Count says', async () => {
		const fetchMock = stubPages([
			{ rows: rows(PAGE_LIMIT_MAX), total: PAGE_LIMIT_MAX + 3 },
			{ rows: rows(3), total: PAGE_LIMIT_MAX + 3 },
		]);

		const tokens = await listTokens();

		expect(tokens).toHaveLength(PAGE_LIMIT_MAX + 3);
		expect(urls(fetchMock)).toEqual([
			`/tokens?limit=${PAGE_LIMIT_MAX}&offset=0`,
			`/tokens?limit=${PAGE_LIMIT_MAX}&offset=${PAGE_LIMIT_MAX}`,
		]);
	});

	it('stops on an empty page even when the total says there is more', async () => {
		const fetchMock = stubPages([
			{ rows: rows(PAGE_LIMIT_MAX), total: 10_000 },
			{ rows: [], total: 10_000 },
		]);

		const tokens = await listTokens();

		expect(tokens).toHaveLength(PAGE_LIMIT_MAX);
		expect(fetchMock).toHaveBeenCalledTimes(2);
	});

	it('stops after a short page when the response carries no total', async () => {
		const fetchMock = stubPages([{ rows: rows(4) }]);

		await listTokens();

		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it('keeps the used_by filter on every page of the secrets list', async () => {
		const fetchMock = stubPages([{ rows: rows(1), total: 1 }]);

		await listObjects('homelab');

		expect(urls(fetchMock)).toEqual([
			`/objects?used_by=homelab&limit=${PAGE_LIMIT_MAX}&offset=0`,
		]);
	});

	it('probes the session with one row, not the whole passkey list', async () => {
		const fetchMock = stubPages([{ rows: rows(1), total: 9 }]);

		await checkSession();

		expect(urls(fetchMock)).toEqual(['/credentials?limit=1']);
	});
});

// alrayyes/hush-hush#670: a name can hold several variants, and a session
// has to say which one it means with ?id= or the API answers 409.
describe('variant id', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	function stubFetch() {
		const fetchMock = vi
			.fn()
			.mockImplementation(async () => new Response('{}', { status: 200 }));
		vi.stubGlobal('fetch', fetchMock);
		vi.stubGlobal('document', { cookie: '' });

		return fetchMock;
	}

	const calledUrl = (fetchMock: ReturnType<typeof stubFetch>) =>
		String(fetchMock.mock.calls[0][0]);

	it('reads one variant by id', async () => {
		const fetchMock = stubFetch();

		await getObjectValue('a b', 'u-1');

		expect(calledUrl(fetchMock)).toMatch(/\/objects\/a%20b\?id=u-1$/);
	});

	it('updates one variant by id', async () => {
		const fetchMock = stubFetch();

		await updateObject('a', 'sealed', ['x'], false, undefined, 'u-1');

		expect(calledUrl(fetchMock)).toMatch(/\/objects\/a\?id=u-1$/);
	});

	it('deletes one variant by id', async () => {
		const fetchMock = stubFetch();

		await deleteObject('a', 'u-1');

		expect(calledUrl(fetchMock)).toMatch(/\/objects\/a\?id=u-1$/);
		expect(fetchMock.mock.calls[0][1].method).toBe('DELETE');
	});

	it('leaves the query off when no id is given', async () => {
		const fetchMock = stubFetch();

		await getObjectValue('a');

		expect(calledUrl(fetchMock)).toMatch(/\/objects\/a$/);
	});
});
