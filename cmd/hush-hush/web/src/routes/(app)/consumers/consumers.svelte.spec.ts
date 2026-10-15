import { beforeEach, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { ApiError } from '#lib/api.js';
import { tokensHref, truncateKey } from '#lib/consumers.js';
import Consumers from './+page.svelte';

// The page refreshes through $app/navigation and writes through #lib/api.js.
// Both are stood in for, so a test hands the page the directory the server
// would send and decides what each write answers.
const stub = vi.hoisted(() => ({
	addConsumer: vi.fn(),
	setConsumerPublicKey: vi.fn(),
	invalidate: vi.fn(),
}));

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	invalidate: stub.invalidate,
}));
vi.mock('#lib/api.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.js')>()),
	addConsumer: stub.addConsumer,
	setConsumerPublicKey: stub.setConsumerPublicKey,
}));

const KEY = 'age1xl8amwv0vaxfk3gkrsgfv8l9nwkfuyqf448jwauhkn4vsw9jfqzq5290c1';
const data = {
	// The layout's own fields, which the page's data type includes.
	environment: 'test',
	version: 'v1.2.3',
	healthy: true,
	authenticated: true,
	consumers: [
		{ name: 'homelab-new', secret_count: 0 },
		{ name: 'keyed-consumer', secret_count: 2, public_key: KEY },
	],
	total: 2,
	page: 1,
	q: '',
	tokenCounts: new Map([['keyed-consumer', 2]]),
};

beforeEach(() => {
	for (const fn of Object.values(stub)) fn.mockReset();
});

const screenOf = (r: Awaited<ReturnType<typeof render>>) => r;
const row = (screen: ReturnType<typeof screenOf>, name: string) =>
	screen.getByRole('row').filter({ hasText: name });
const card = (screen: ReturnType<typeof screenOf>, name: string) =>
	screen
		.getByRole('list', { name: 'Consumers' })
		.getByRole('listitem')
		.filter({ hasText: name });

// #476: a consumer with no consumer tokens shows a plain 0, not a link.
test('a consumer with no tokens shows a plain 0, and one with tokens links to them', async () => {
	const screen = await render(Consumers, { data });

	const none = row(screen, 'homelab-new').getByRole('cell').nth(2);
	await expect.poll(() => none.element().textContent?.trim()).toBe('0');
	expect(none.getByRole('link').query()).toBeNull();
	await expect
		.element(row(screen, 'keyed-consumer').getByRole('link', { name: '2' }))
		.toHaveAttribute('href', tokensHref('keyed-consumer'));
});

test('each row shows how many secrets name the consumer', async () => {
	const screen = await render(Consumers, { data });

	const none = row(screen, 'homelab-new').getByRole('cell').nth(1);
	const some = row(screen, 'keyed-consumer').getByRole('cell').nth(1);
	await expect.poll(() => none.element().textContent?.trim()).toBe('0');
	await expect.poll(() => some.element().textContent?.trim()).toBe('2');
});

// #780: a key is truncated with Replace and Copy; none reads not registered.
test('a consumer without a key reads not registered with a Set key button', async () => {
	const screen = await render(Consumers, { data });
	const without = row(screen, 'homelab-new');

	await expect
		.poll(() => without.element().textContent)
		.toContain('not registered');
	const setKey = without.getByRole('button', {
		name: 'Set key for homelab-new',
	});
	await expect.poll(() => setKey.element().textContent?.trim()).toBe('Set key');
	expect(without.getByRole('button', { name: /^Copy/ }).query()).toBeNull();
});

test('a consumer with a key shows it truncated, with Copy and Replace key', async () => {
	const screen = await render(Consumers, { data });
	const keyed = row(screen, 'keyed-consumer');

	await expect
		.poll(() => keyed.element().textContent)
		.toContain(truncateKey(KEY));
	const replaceKey = keyed.getByRole('button', {
		name: 'Replace key for keyed-consumer',
	});
	await expect
		.poll(() => replaceKey.element().textContent?.trim())
		.toBe('Replace key');
	await expect
		.element(
			keyed.getByRole('button', { name: 'Copy public key keyed-consumer' }),
		)
		.toBeVisible();
});

// #482: on a phone the card says the same, from the same data.
test('a phone card says not registered and offers no Copy for a consumer without a key', async () => {
	const screen = await render(Consumers, { data });
	const without = card(screen, 'homelab-new');

	await expect
		.poll(() => without.element().textContent)
		.toContain('not registered');
	await expect.poll(() => without.element().textContent).toMatch(/0 secrets\b/);
	expect(without.getByRole('button', { name: /^Copy/ }).query()).toBeNull();
	const setKey = without.getByRole('button', {
		name: 'Set key for homelab-new',
	});
	await expect.poll(() => setKey.element().textContent?.trim()).toBe('Set key');
});

test("a phone card links to the consumer's tokens, and says 0 plainly without any", async () => {
	const screen = await render(Consumers, { data });

	await expect
		.element(
			card(screen, 'keyed-consumer').getByRole('link', { name: '2 tokens' }),
		)
		.toHaveAttribute('href', tokensHref('keyed-consumer'));
	const none = card(screen, 'homelab-new');
	await expect.poll(() => none.element().textContent).toContain('0 tokens');
	expect(none.getByRole('link', { name: /tokens/ }).query()).toBeNull();
});

test('a phone card offers Replace key and Copy for a consumer with a key', async () => {
	const screen = await render(Consumers, { data });
	const keyed = card(screen, 'keyed-consumer');

	const replaceKey = keyed.getByRole('button', {
		name: 'Replace key for keyed-consumer',
	});
	await expect
		.poll(() => replaceKey.element().textContent?.trim())
		.toBe('Replace key');
	await expect
		.element(
			keyed.getByRole('button', { name: 'Copy public key keyed-consumer' }),
		)
		.toBeVisible();
});

// #324: adding a name the server already has is refused, in the server's words.
test('adding a consumer the server already has shows its message and keeps the dialog', async () => {
	stub.addConsumer.mockRejectedValue(
		new ApiError(409, 'consumer already exists'),
	);
	const screen = await render(Consumers, { data });

	await screen.getByRole('button', { name: 'Add consumer' }).click();
	const dialog = screen.getByRole('dialog', { name: 'Add a consumer' });
	await dialog.getByLabelText('Name', { exact: true }).fill('homelab-new');
	await dialog.getByRole('button', { name: 'Add' }).click();

	await expect
		.poll(() => dialog.getByRole('alert').element().textContent)
		.toBe('consumer already exists');
	expect(stub.addConsumer).toHaveBeenCalledWith('homelab-new');
	expect(stub.invalidate).not.toHaveBeenCalled();
});

// #780: the server validates a key, so a bad one shows the server's message
// and is never echoed back.
test('a key the server refuses shows its message, and not the key', async () => {
	stub.setConsumerPublicKey.mockRejectedValue(
		new ApiError(400, 'public_key must be an age public key (age1...)'),
	);
	const screen = await render(Consumers, { data });

	await row(screen, 'homelab-new')
		.getByRole('button', { name: 'Set key for homelab-new' })
		.click();
	const dialog = screen.getByRole('dialog', { name: 'Set public key' });
	await dialog.getByLabelText('Public key', { exact: true }).fill('not-a-key');
	await dialog.getByRole('button', { name: 'Save' }).click();

	const alert = dialog.getByRole('alert');
	await expect
		.poll(() => alert.element().textContent)
		.toContain('public_key must be an age public key');
	expect(alert.element().textContent).not.toContain('not-a-key');
	expect(stub.invalidate).not.toHaveBeenCalled();
});

test('a valid key is saved for that consumer, and the page data reloads', async () => {
	stub.setConsumerPublicKey.mockResolvedValue(undefined);
	const screen = await render(Consumers, { data });

	await row(screen, 'homelab-new')
		.getByRole('button', { name: 'Set key for homelab-new' })
		.click();
	const dialog = screen.getByRole('dialog', { name: 'Set public key' });
	await dialog.getByLabelText('Public key', { exact: true }).fill(KEY);
	await dialog.getByRole('button', { name: 'Save' }).click();

	await vi.waitFor(() =>
		expect(stub.setConsumerPublicKey).toHaveBeenCalledWith('homelab-new', KEY),
	);
	await vi.waitFor(() =>
		expect(stub.invalidate).toHaveBeenCalledWith('app:consumers'),
	);
});

test('a consumer with a key opens a Replace dialog, one without opens a Set dialog', async () => {
	const screen = await render(Consumers, { data });

	await row(screen, 'keyed-consumer')
		.getByRole('button', { name: 'Replace key for keyed-consumer' })
		.click();

	await expect
		.element(screen.getByRole('dialog', { name: 'Replace public key' }))
		.toBeVisible();
});
