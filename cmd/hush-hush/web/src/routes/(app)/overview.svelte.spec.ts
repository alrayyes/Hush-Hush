import { beforeEach, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Overview from './+page.svelte';

// The page reads the URL through $app/state and navigates through
// $app/navigation. Both are stood in for, so a test can set the address it
// loads at and see where the page sends the reader.
const router = vi.hoisted(() => ({
	url: new URL('http://localhost/'),
	goto: vi.fn(),
	invalidate: vi.fn(),
}));

vi.mock('$app/navigation', () => ({
	goto: router.goto,
	invalidate: router.invalidate,
}));
vi.mock('$app/state', () => ({
	page: {
		get url() {
			return router.url;
		},
	},
}));

const objects = Array.from({ length: 21 }, (_, i) => ({
	id: `00000000-0000-4000-8000-${String(i).padStart(12, '0')}`,
	slug: `paging_${String(i).padStart(2, '0')}`,
	used_by: [],
	tags: [],
	created_at: '2026-01-01T00:00:00Z',
	updated_at: '2026-01-01T00:00:00Z',
}));
const data = {
	// The layout's own fields, which the page's data type includes.
	environment: 'test',
	version: 'v1.2.3',
	healthy: true,
	authenticated: true,
	objects,
	filters: { usedBy: [] },
	ownerPublicKey: undefined,
};

beforeEach(() => {
	router.goto.mockReset();
	router.url = new URL('http://localhost/');
});

// alrayyes/hush-hush#893: narrowing the list sends the reader back to page 1,
// and loading a page, which is the first run of the same effect, keeps it.
test('loading page 2 keeps page 2', async () => {
	router.url = new URL('http://localhost/?page=2');

	const screen = await render(Overview, { data });

	const pager = screen.getByRole('navigation', { name: 'Secrets pages' });
	await expect
		.poll(() => pager.element().textContent)
		.toMatch(/Showing 21–21 of 21/);
	expect(router.goto).not.toHaveBeenCalled();
});

test('searching from page 2 goes back to page 1, replacing the history entry', async () => {
	router.url = new URL('http://localhost/?page=2');
	const screen = await render(Overview, { data });

	await screen
		.getByRole('searchbox', { name: 'Filter secrets' })
		.fill('paging_');

	await vi.waitFor(() =>
		expect(router.goto).toHaveBeenCalledWith('/', {
			replace: true,
			reset: false,
		}),
	);
});

test('searching from page 1 goes nowhere', async () => {
	const screen = await render(Overview, { data });

	await screen
		.getByRole('searchbox', { name: 'Filter secrets' })
		.fill('paging_');

	await expect
		.element(screen.getByRole('searchbox', { name: 'Filter secrets' }))
		.toHaveValue('paging_');
	expect(router.goto).not.toHaveBeenCalled();
});

test('picking a tag from page 2 goes back to page 1', async () => {
	router.url = new URL('http://localhost/?page=2');
	const tagged = {
		...data,
		objects: objects.map((o) => ({ ...o, tags: ['paging'] })),
	};
	const screen = await render(Overview, { data: tagged });

	await screen
		.getByRole('group', { name: 'Filter by tag' })
		.getByRole('button', { name: 'paging (21)' })
		.click();

	await vi.waitFor(() =>
		expect(router.goto).toHaveBeenCalledWith('/', {
			replace: true,
			reset: false,
		}),
	);
});
