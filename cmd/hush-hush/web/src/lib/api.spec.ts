import { afterEach, describe, expect, it, vi } from 'vitest';
import { listConsumerDirectory } from './api';
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
