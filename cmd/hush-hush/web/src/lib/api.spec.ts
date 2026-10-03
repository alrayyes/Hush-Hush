import { afterEach, describe, expect, it, vi } from 'vitest';
import { listConsumerDirectory, updateObject } from './api';
import { CONSUMERS_PAGE_SIZE_MAX } from './api-limits';

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
