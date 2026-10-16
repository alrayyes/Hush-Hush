import { beforeEach, expect, test, vi } from 'vitest';
import { load } from './+page';

// The load function reads the address, asks the API for that page and
// reduces the consumer tokens to a count per consumer. The API calls are
// stood in for, so a test decides what the server answers.
const api = vi.hoisted(() => ({
	listConsumersPage: vi.fn(),
	listConsumerTokens: vi.fn(),
}));

vi.mock('#lib/api.js', () => api);

const run = (address: string) => {
	const depends = vi.fn();
	const event = { depends, url: new URL(address, 'http://localhost') };

	return {
		depends,
		result: load(event as unknown as Parameters<typeof load>[0]) as Promise<{
			consumers: unknown[];
			total: number;
			page: number;
			q: string;
			tokenCounts: Map<string, number>;
		}>,
	};
};

const answer = (total: number, consumers: unknown[] = []) => {
	api.listConsumersPage.mockResolvedValue({ consumers, total });
	api.listConsumerTokens.mockResolvedValue([]);
};

beforeEach(() => {
	api.listConsumersPage.mockReset();
	api.listConsumerTokens.mockReset();
});

test('it asks for the page and the filter the address holds, 20 at a time', async () => {
	answer(97);

	await run('/consumers?q=svcfiles&page=2').result;

	expect(api.listConsumersPage).toHaveBeenCalledWith({
		q: 'svcfiles',
		page: 2,
		page_size: 20,
	});
});

test('no filter in the address asks for none', async () => {
	answer(102);

	await run('/consumers').result;

	expect(api.listConsumersPage).toHaveBeenCalledWith({
		q: undefined,
		page: 1,
		page_size: 20,
	});
});

// alrayyes/hush-hush#894: a bookmark past the end lands on the last page.
test('a page past the end redirects to the last page that has rows', async () => {
	answer(102);

	await expect(run('/consumers?page=99').result).rejects.toMatchObject({
		status: 307,
		location: '/consumers?page=6',
	});
});

test('the redirect keeps the search', async () => {
	answer(97);

	await expect(
		run('/consumers?q=svcfiles&page=99').result,
	).rejects.toMatchObject({
		status: 307,
		location: '/consumers?q=svcfiles&page=5',
	});
});

test('it hands back the search and the page it was asked for', async () => {
	answer(97, [{ name: 'a' }]);

	const data = await run('/consumers?q=svcfiles&page=2').result;

	expect(data).toMatchObject({ total: 97, page: 2, q: 'svcfiles' });
});

test('the last page itself does not redirect', async () => {
	answer(102, [{ name: 'a' }]);

	const data = await run('/consumers?page=6').result;

	expect(data).toMatchObject({ total: 102, page: 6, q: '' });
});

test('the directory is refreshed by app:consumers', async () => {
	answer(2);

	const { depends, result } = run('/consumers');
	await result;

	expect(depends).toHaveBeenCalledWith('app:consumers');
});

// The API has no per-consumer filter on tokens, so the page counts them here.
test('it counts consumer tokens per consumer', async () => {
	api.listConsumersPage.mockResolvedValue({ consumers: [], total: 0 });
	api.listConsumerTokens.mockResolvedValue([
		{ consumer: 'a' },
		{ consumer: 'a' },
		{ consumer: 'b' },
	]);

	const { tokenCounts } = await run('/consumers').result;

	expect([...tokenCounts]).toEqual([
		['a', 2],
		['b', 1],
	]);
});
