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

// What the dialogs fetch and delete by is the API's; a test reads which id the
// page asked for.
const api = vi.hoisted(() => ({
	getObjectValue: vi.fn(),
	deleteObject: vi.fn(),
}));
vi.mock('#lib/api.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.js')>()),
	getObjectValue: api.getObjectValue,
	deleteObject: api.deleteObject,
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
	api.getObjectValue.mockReset();
	api.deleteObject.mockReset();
	router.url = new URL('http://localhost/');
});

// alrayyes/hush-hush#893: narrowing the list sends the reader back to page 1,
// and loading a page, which is the first run of the same effect, keeps it.
test('loading page 2 keeps page 2', async () => {
	router.url = new URL('http://localhost/?page=2');

	// Time is the test's: a reset deferred by a timer or a frame after the page
	// mounts would otherwise land after the assertion and pass it.
	vi.useFakeTimers({ toFake: ['setTimeout', 'requestAnimationFrame'] });
	try {
		await render(Overview, { data });
		await vi.advanceTimersByTimeAsync(1000);

		expect(router.goto).not.toHaveBeenCalled();
	} finally {
		vi.useRealTimers();
	}
});

test('loading page 2 shows the second page', async () => {
	router.url = new URL('http://localhost/?page=2');

	const screen = await render(Overview, { data });

	const pager = screen.getByRole('navigation', { name: 'Secrets pages' });
	await expect
		.poll(() => pager.element().textContent)
		.toMatch(/Showing 21–21 of 21/);
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

// alrayyes/hush-hush#522: the list shows each secret's tags and filters by
// one. The three tagged secrets sit among two untagged ones.
let secretCount = 0;
const secret = (slug: string, tags: string[]) => ({
	id: `10000000-0000-4000-8000-${String(secretCount++).padStart(12, '0')}`,
	slug,
	used_by: [],
	tags,
	created_at: '2026-01-01T00:00:00Z',
	updated_at: '2026-01-02T00:00:00Z',
	created_by: { type: 'session', id: 'user-1' },
	// Updated by someone else, so a card that names the creator twice shows.
	updated_by: { type: 'token', id: 'tok-9' },
});
const tagData = {
	...data,
	objects: [
		secret('plain_one', []),
		secret('plain_two', []),
		secret('tagged_one', ['prod', 'homelab']),
		secret('tagged_two', ['prod']),
		secret('tagged_three', ['backup']),
	],
};

const cards = (screen: Awaited<ReturnType<typeof render>>) =>
	screen.getByRole('list', { name: 'Secrets' }).getByRole('listitem');
const pill = (screen: Awaited<ReturnType<typeof render>>, name: string) =>
	screen
		.getByRole('group', { name: 'Filter by tag' })
		.getByRole('button', { name });

test("a card lists the secret's own tags", async () => {
	const screen = await render(Overview, { data: tagData });

	const tags = cards(screen)
		.filter({ hasText: 'tagged_one' })
		.getByTestId('tag');
	await expect
		.poll(() =>
			tags
				.elements()
				.map((tag) => tag.textContent?.trim())
				.sort(),
		)
		.toEqual(['homelab', 'prod']);
});

test('a card says who created the secret and who last updated it', async () => {
	const screen = await render(Overview, { data: tagData });

	const card = cards(screen).filter({ hasText: 'tagged_three' });
	await expect
		.poll(() => card.element().textContent?.replace(/\s+/g, ' '))
		.toMatch(/Created .+ by admin/);
	await expect
		.poll(() => card.element().textContent?.replace(/\s+/g, ' '))
		.toMatch(/Updated .+ by token:tok-9/);
});

test('the pills read All, then each tag by how many secrets carry it', async () => {
	const screen = await render(Overview, { data: tagData });

	const pills = screen
		.getByRole('group', { name: 'Filter by tag' })
		.getByRole('button');
	await expect
		.poll(() => pills.elements().map((p) => p.textContent?.trim()))
		.toEqual(['All (5)', 'prod (2)', 'backup (1)', 'homelab (1)']);
	// Nothing is picked yet, so All is the one pressed.
	await expect
		.element(pill(screen, 'All (5)'))
		.toHaveAttribute('aria-pressed', 'true');
	await expect
		.element(pill(screen, 'prod (2)'))
		.toHaveAttribute('aria-pressed', 'false');
});

test('picking a tag shows only its secrets and marks the pill pressed', async () => {
	const screen = await render(Overview, { data: tagData });

	await pill(screen, 'prod (2)').click();

	await expect
		.element(pill(screen, 'prod (2)'))
		.toHaveAttribute('aria-pressed', 'true');
	await expect
		.element(pill(screen, 'All (5)'))
		.toHaveAttribute('aria-pressed', 'false');
	await expect.poll(() => cards(screen).elements().length).toBe(2);
});

test('a tag and the search box both apply', async () => {
	const screen = await render(Overview, { data: tagData });

	await pill(screen, 'prod (2)').click();
	await screen
		.getByRole('searchbox', { name: 'Filter secrets' })
		.fill('tagged_two');

	await expect.poll(() => cards(screen).elements().length).toBe(1);

	// A secret the search matches but the tag doesn't stays out.
	await screen
		.getByRole('searchbox', { name: 'Filter secrets' })
		.fill('tagged_three');
	await expect.poll(() => cards(screen).elements().length).toBe(0);
});

test('All brings every secret back', async () => {
	const screen = await render(Overview, { data: tagData });

	await pill(screen, 'prod (2)').click();
	await expect.poll(() => cards(screen).elements().length).toBe(2);
	await pill(screen, 'All (5)').click();

	await expect.poll(() => cards(screen).elements().length).toBe(5);
});

// alrayyes/hush-hush#893 and #897: the table pages 20 at a time by default, the
// page and the rows per page are the URL's, and the size can be chosen.
const pager = (screen: Awaited<ReturnType<typeof render>>) =>
	screen.getByRole('navigation', { name: 'Secrets pages' });
const sizeSelect = (screen: Awaited<ReturnType<typeof render>>) =>
	pager(screen).getByRole('combobox', { name: 'Rows per page' });

test('page 1 shows the first 20 of 21 and links Next to page 2', async () => {
	const screen = await render(Overview, { data });

	await expect
		.poll(() => pager(screen).element().textContent)
		.toMatch(/Showing 1–20 of 21/);
	await expect.poll(() => cards(screen).elements().length).toBe(20);
	await expect
		.element(pager(screen).getByRole('link', { name: 'Next' }))
		.toHaveAttribute('href', '/?page=2');
});

test('page 2 shows the one that is left', async () => {
	router.url = new URL('http://localhost/?page=2');
	const screen = await render(Overview, { data });

	await expect.poll(() => cards(screen).elements().length).toBe(1);
	await expect
		.poll(() => cards(screen).first().element().textContent)
		.toContain('paging_20');
});

test('a page past the end shows the last one', async () => {
	router.url = new URL('http://localhost/?page=99');
	const screen = await render(Overview, { data });

	await expect
		.poll(() => pager(screen).element().textContent)
		.toMatch(/Showing 21–21 of 21/);
	await expect.poll(() => cards(screen).elements().length).toBe(1);
});

test('the rows-per-page menu offers 20, 50 and 100', async () => {
	const screen = await render(Overview, { data });

	await sizeSelect(screen).click();

	const options = screen.getByRole('listbox').getByRole('option');
	await expect
		.poll(() => options.elements().map((o) => o.textContent?.trim()))
		.toEqual(['20', '50', '100']);
});

test('picking 50 asks for ?size=50 from page 1', async () => {
	router.url = new URL('http://localhost/?page=2');
	const screen = await render(Overview, { data });

	await sizeSelect(screen).click();
	await screen.getByRole('option', { name: '50', exact: true }).click();

	await vi.waitFor(() =>
		expect(router.goto).toHaveBeenCalledWith('/?size=50', { reset: false }),
	);
});

test('at size 50 everything is on one page, and the selector stays to undo it', async () => {
	router.url = new URL('http://localhost/?size=50');
	const screen = await render(Overview, { data });

	await expect.poll(() => cards(screen).elements().length).toBe(21);
	await expect
		.poll(() => pager(screen).element().textContent)
		.toMatch(/Showing 1–21 of 21/);
	await expect.element(sizeSelect(screen)).toHaveTextContent('50');
	await expect
		.element(pager(screen).getByRole('link', { name: 'Next' }))
		.not.toBeInTheDocument();
});

test('picking 20 again drops the size from the address', async () => {
	router.url = new URL('http://localhost/?size=50');
	const screen = await render(Overview, { data });

	await sizeSelect(screen).click();
	await screen.getByRole('option', { name: '20', exact: true }).click();

	await vi.waitFor(() =>
		expect(router.goto).toHaveBeenCalledWith('/', { reset: false }),
	);
});

// The pager's links carry the chosen size, so paging at 50 stays at 50. The
// fixture needs more than 50 rows for a second page to exist at that size.
const manySecrets = Array.from({ length: 51 }, (_, i) =>
	secret(`many_${String(i).padStart(2, '0')}`, []),
);

test('at size 50 Next keeps the size', async () => {
	router.url = new URL('http://localhost/?size=50');
	const screen = await render(Overview, {
		data: { ...data, objects: manySecrets },
	});

	await expect
		.poll(() => pager(screen).element().textContent)
		.toMatch(/Showing 1–50 of 51/);
	await expect
		.element(pager(screen).getByRole('link', { name: 'Next' }))
		.toHaveAttribute('href', '/?page=2&size=50');
});

test('at size 50 on page 2 Previous goes to page 1 and keeps the size', async () => {
	router.url = new URL('http://localhost/?page=2&size=50');
	const screen = await render(Overview, {
		data: { ...data, objects: manySecrets },
	});

	await expect
		.poll(() => pager(screen).element().textContent)
		.toMatch(/Showing 51–51 of 51/);
	await expect
		.element(pager(screen).getByRole('link', { name: 'Previous' }))
		.toHaveAttribute('href', '/?size=50');
});

test('at size 100 Next carries 100, not a size the test happened to use before', async () => {
	router.url = new URL('http://localhost/?size=100');
	const hundredAndOne = Array.from({ length: 101 }, (_, i) =>
		secret(`hundred_${String(i).padStart(3, '0')}`, []),
	);
	const screen = await render(Overview, {
		data: { ...data, objects: hundredAndOne },
	});

	await expect
		.poll(() => pager(screen).element().textContent)
		.toMatch(/Showing 1–100 of 101/);
	await expect
		.element(pager(screen).getByRole('link', { name: 'Next' }))
		.toHaveAttribute('href', '/?page=2&size=100');
});

// alrayyes/hush-hush#670: one name can hold a value per consumer, each under its
// own id. Which variant a row is, and that its buttons act on that one and not
// the name, are the page's; the conflict a duplicate consumer raises is the
// server's message (conflict_message_test.go), shown as it comes.
const variantA = {
	...secret('release_token', []),
	id: '20000000-0000-4000-8000-00000000000a',
	used_by: ['consumer_a'],
};
const variantD = {
	...secret('release_token', []),
	id: '20000000-0000-4000-8000-00000000000d',
	used_by: ['consumer_d'],
};
const variantData = { ...data, objects: [variantA, variantD] };
const variantCard = (
	screen: Awaited<ReturnType<typeof render>>,
	consumer: string,
) => cards(screen).filter({ hasText: consumer });

test('View opens the variant it was clicked on and fetches that id', async () => {
	api.getObjectValue.mockResolvedValue('c2VhbGVk');
	const screen = await render(Overview, { data: variantData });

	await variantCard(screen, 'consumer_d')
		.getByRole('button', { name: 'View' })
		.click();

	const dialog = screen.getByRole('dialog', { name: 'release_token' });
	await expect
		.poll(() => dialog.element().textContent)
		.toContain('Variant for consumer_d');
	await expect
		.poll(() => dialog.getByRole('listitem').element().textContent)
		.toContain('consumer_d');
	await vi.waitFor(() =>
		expect(api.getObjectValue).toHaveBeenCalledWith(
			'release_token',
			variantD.id,
		),
	);
	await expect
		.element(dialog.getByRole('textbox', { name: 'Ciphertext (base64)' }))
		.toHaveValue('c2VhbGVk');
});

test('Edit says which variant it is editing', async () => {
	const screen = await render(Overview, { data: variantData });

	await variantCard(screen, 'consumer_d')
		.getByRole('button', { name: 'Edit' })
		.click();

	const dialog = screen.getByRole('dialog', { name: 'Edit release_token' });
	await expect
		.poll(() => dialog.element().textContent)
		.toContain('Variant for consumer_d');
	await expect
		.poll(() => dialog.element().textContent)
		.not.toContain('consumer_a');
});

test('Delete confirms the variant and removes that id, not its sibling', async () => {
	api.deleteObject.mockResolvedValue(undefined);
	const screen = await render(Overview, { data: variantData });

	await variantCard(screen, 'consumer_d')
		.getByRole('button', { name: 'Delete' })
		.click();

	const dialog = screen.getByRole('alertdialog', {
		name: 'Delete release_token?',
	});
	await expect
		.poll(() => dialog.element().textContent)
		.toContain('Variant for consumer_d');
	await dialog.getByRole('button', { name: 'Delete', exact: true }).click();
	await vi.waitFor(() =>
		expect(api.deleteObject).toHaveBeenCalledWith('release_token', variantD.id),
	);
	expect(api.deleteObject).toHaveBeenCalledTimes(1);
});

test("Inspect links to the variant's own page by id", async () => {
	const screen = await render(Overview, { data: variantData });

	const inspect = variantCard(screen, 'consumer_d').getByRole('link', {
		name: 'Inspect',
	});

	await expect
		.element(inspect)
		.toHaveAttribute('href', `/secrets/release_token?id=${variantD.id}`);
});

test('a name with one variant carries no variant label', async () => {
	const screen = await render(Overview, {
		data: { ...data, objects: [variantA] },
	});

	await cards(screen).getByRole('button', { name: 'Edit' }).click();

	const dialog = screen.getByRole('dialog', { name: 'Edit release_token' });
	await expect.element(dialog).toBeVisible();
	await expect
		.poll(() => dialog.element().textContent)
		.not.toContain('Variant for');
});

// alrayyes/hush-hush#480: below md the list is one card per secret, with a
// search box that filters by id or description.
const described = {
	...data,
	objects: [
		{
			...secret('mattermost_deploy_webhook', []),
			description: 'prod deploy webhook for homelab/vps-docker',
		},
		{ ...secret('other_secret', []), description: 'something else' },
	],
};
const searchBox = (screen: Awaited<ReturnType<typeof render>>) =>
	screen.getByRole('searchbox', { name: 'Filter secrets' });

test('a card shows the secret, its description and how it is encrypted', async () => {
	const screen = await render(Overview, { data: described });

	const card = cards(screen).filter({ hasText: 'mattermost_deploy_webhook' });
	await expect
		.poll(() => card.element().textContent)
		.toContain('prod deploy webhook for homelab/vps-docker');
	await expect
		.poll(() => card.element().textContent)
		.toContain('age-encrypted (X25519)');
});

test('searching matches the id or the description', async () => {
	const screen = await render(Overview, { data: described });

	await searchBox(screen).fill('vps-docker');
	await expect.poll(() => cards(screen).elements().length).toBe(1);
	await expect
		.poll(() => cards(screen).element().textContent)
		.toContain('mattermost_deploy_webhook');

	await searchBox(screen).fill('other_');
	await expect.poll(() => cards(screen).elements().length).toBe(1);
	await expect
		.poll(() => cards(screen).element().textContent)
		.toContain('other_secret');
});

test('a search that matches nothing says so, and clearing it brings the list back', async () => {
	const screen = await render(Overview, { data: described });

	await searchBox(screen).fill('no-such-secret');

	await expect.poll(() => cards(screen).elements().length).toBe(0);
	await expect
		.poll(() => document.body.textContent)
		.toContain('No secrets match “no-such-secret”.');
	await searchBox(screen).fill('');
	await expect.poll(() => cards(screen).elements().length).toBe(2);
});

test('Delete asks first, and Cancel deletes nothing', async () => {
	const screen = await render(Overview, { data: described });

	await cards(screen)
		.filter({ hasText: 'mattermost_deploy_webhook' })
		.getByRole('button', { name: 'Delete' })
		.click();

	const dialog = screen.getByRole('alertdialog', {
		name: 'Delete mattermost_deploy_webhook?',
	});
	await expect.element(dialog).toBeVisible();
	await dialog.getByRole('button', { name: 'Cancel' }).click();
	await expect.element(dialog).not.toBeInTheDocument();
	expect(api.deleteObject).not.toHaveBeenCalled();
});
