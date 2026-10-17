import { beforeEach, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { ConsumerTokenMetadata, TokenMetadata } from '#lib/api.js';
import Settings from './+page.svelte';

// The page reads the address through $app/state, refreshes through
// $app/navigation and writes through #lib/api.js. All three are stood in for,
// so a test hands the page the tokens the server would send and sees what it
// shows and calls.
const stub = vi.hoisted(() => ({
	purgeToken: vi.fn(),
	purgeConsumerToken: vi.fn(),
	revokeToken: vi.fn(),
	revokeConsumerToken: vi.fn(),
	rotateToken: vi.fn(),
	rotateConsumerToken: vi.fn(),
	createConsumerToken: vi.fn(),
	listConsumerDirectory: vi.fn(),
	invalidate: vi.fn(),
	url: new URL('http://localhost/settings'),
}));

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	invalidate: stub.invalidate,
}));
vi.mock('$app/state', () => ({
	page: {
		get url() {
			return stub.url;
		},
	},
}));
vi.mock('#lib/api.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.js')>()),
	purgeToken: stub.purgeToken,
	purgeConsumerToken: stub.purgeConsumerToken,
	revokeToken: stub.revokeToken,
	revokeConsumerToken: stub.revokeConsumerToken,
	rotateToken: stub.rotateToken,
	rotateConsumerToken: stub.rotateConsumerToken,
	createConsumerToken: stub.createConsumerToken,
	listConsumerDirectory: stub.listConsumerDirectory,
}));

const token = (overrides: Partial<TokenMetadata>): TokenMetadata => ({
	id: 'id',
	description: 'a token',
	owner: 'cli',
	created_at: '2026-01-01T00:00:00Z',
	expires_at: '2099-01-01T00:00:00Z',
	revoked: false,
	...overrides,
});

// An expiry long past by the browser's clock, on a token the server calls
// active: what a browser whose clock runs ahead sees (#536).
const live = token({
	id: 'live',
	description: 'live token',
	expires_at: '2020-01-01T00:00:00Z',
	status: 'active',
	allowed_actions: ['rotate', 'revoke'],
});
const expired = token({
	id: 'expired',
	description: 'expired token',
	status: 'expired',
	allowed_actions: ['purge'],
});
const revoked = token({
	id: 'revoked',
	description: 'revoked token',
	revoked: true,
	status: 'revoked',
	allowed_actions: ['purge'],
});

const consumerToken = (
	overrides: Partial<ConsumerTokenMetadata>,
): ConsumerTokenMetadata => ({
	id: '0123456789abcdef',
	consumer: 'deploy-bot',
	description: 'a read token',
	created_at: '2026-01-01T00:00:00Z',
	expires_at: '2099-01-01T00:00:00Z',
	revoked: false,
	...overrides,
});
const liveConsumerToken = consumerToken({
	id: 'aaaaaaaaaaaaaaaa',
	description: 'live read token',
	expires_at: '2020-01-01T00:00:00Z',
	status: 'active',
	allowed_actions: ['rotate', 'revoke'],
});
const deadConsumerToken = consumerToken({
	id: 'bbbbbbbbbbbbbbbb',
	description: 'dead read token',
	status: 'expired',
	allowed_actions: ['purge'],
});

const data = {
	// The layout's own fields, which the page's data type includes.
	environment: 'test',
	version: 'v1.2.3',
	healthy: true,
	authenticated: true,
	credentials: [],
	tokens: [live, expired, revoked],
	consumerTokens: [liveConsumerToken, deadConsumerToken],
};

beforeEach(() => {
	for (const value of Object.values(stub)) {
		if (typeof value === 'function') value.mockReset();
	}
	stub.url = new URL('http://localhost/settings');
	// The consumer picker reads the directory when it mounts.
	stub.listConsumerDirectory.mockResolvedValue([]);
});

const row = (screen: Awaited<ReturnType<typeof render>>, name: RegExp) =>
	screen.getByRole('row', { name });

test('a token the server calls active shows Active, Rotate and Revoke, whatever the browser clock says', async () => {
	const screen = await render(Settings, { data });
	const live = row(screen, /live token/);

	await expect
		.element(live.getByRole('cell', { name: 'Active' }))
		.toBeVisible();
	await expect
		.element(live.getByRole('button', { name: 'Rotate' }))
		.toBeVisible();
	await expect
		.element(live.getByRole('button', { name: 'Revoke' }))
		.toBeVisible();
	await expect
		.element(live.getByRole('button', { name: 'Delete permanently' }))
		.not.toBeInTheDocument();
});

test('an expired token reads Expired and offers only Delete permanently', async () => {
	const screen = await render(Settings, { data });
	const expired = row(screen, /expired token/);

	await expect
		.element(expired.getByRole('cell', { name: 'Expired' }))
		.toBeVisible();
	await expect
		.element(expired.getByRole('button', { name: 'Delete permanently' }))
		.toBeVisible();
	await expect
		.element(expired.getByRole('button', { name: 'Rotate' }))
		.not.toBeInTheDocument();
	await expect
		.element(expired.getByRole('button', { name: 'Revoke' }))
		.not.toBeInTheDocument();
});

test('a revoked token reads Revoked and offers only Delete permanently', async () => {
	const screen = await render(Settings, { data });
	const revoked = row(screen, /revoked token/);

	await expect
		.element(revoked.getByRole('cell', { name: 'Revoked' }))
		.toBeVisible();
	await expect
		.element(revoked.getByRole('button', { name: 'Delete permanently' }))
		.toBeVisible();
	await expect
		.element(revoked.getByRole('button', { name: 'Rotate' }))
		.not.toBeInTheDocument();
});

test('Delete permanently asks first, then purges that token and reloads the page data', async () => {
	stub.purgeToken.mockResolvedValue(undefined);
	const screen = await render(Settings, { data });

	await row(screen, /expired token/)
		.getByRole('button', { name: 'Delete permanently' })
		.click();
	expect(stub.purgeToken).not.toHaveBeenCalled();
	await screen
		.getByRole('alertdialog', { name: 'Delete this token permanently?' })
		.getByRole('button', { name: 'Delete permanently' })
		.click();

	await vi.waitFor(() =>
		expect(stub.purgeToken).toHaveBeenCalledWith('expired'),
	);
	await vi.waitFor(() =>
		expect(stub.invalidate).toHaveBeenCalledWith('app:settings'),
	);
});

test('a purge the server refuses is shown in the dialog and keeps it open', async () => {
	const { ApiError } = await import('#lib/api.js');
	stub.purgeToken.mockRejectedValue(
		new ApiError(
			409,
			'token must be revoked or expired before it can be purged',
		),
	);
	const screen = await render(Settings, { data });

	await row(screen, /expired token/)
		.getByRole('button', { name: 'Delete permanently' })
		.click();
	const dialog = screen.getByRole('alertdialog', {
		name: 'Delete this token permanently?',
	});
	await dialog.getByRole('button', { name: 'Delete permanently' }).click();

	const alert = dialog.getByRole('alert');
	await expect
		.poll(() => alert.element().textContent)
		.toContain('must be revoked or expired');
	expect(stub.invalidate).not.toHaveBeenCalled();
});

// The consumer token table is a second copy of the same markup (#536).
test('a consumer token the server calls active shows Active, Rotate and Revoke, whatever the browser clock says', async () => {
	const screen = await render(Settings, { data });
	const live = row(screen, /live read token/);

	await expect
		.element(live.getByRole('cell', { name: 'Active' }))
		.toBeVisible();
	await expect
		.element(live.getByRole('button', { name: 'Rotate' }))
		.toBeVisible();
	await expect
		.element(live.getByRole('button', { name: 'Revoke' }))
		.toBeVisible();
	await expect
		.element(live.getByRole('button', { name: 'Delete permanently' }))
		.not.toBeInTheDocument();
});

test('an expired consumer token reads Expired and offers only Delete permanently', async () => {
	const screen = await render(Settings, { data });
	const dead = row(screen, /dead read token/);

	await expect
		.element(dead.getByRole('cell', { name: 'Expired' }))
		.toBeVisible();
	await expect
		.element(dead.getByRole('button', { name: 'Delete permanently' }))
		.toBeVisible();
	await expect
		.element(dead.getByRole('button', { name: 'Rotate' }))
		.not.toBeInTheDocument();
	await expect
		.element(dead.getByRole('button', { name: 'Revoke' }))
		.not.toBeInTheDocument();
});

// The phone cards are TokenCard, which the page hands a token and three
// handlers. TokenCard.svelte.spec.ts pins the card; these pin that the page
// wires each button to its own dialog and its own call, for the right kind of
// token.
const cardActions = [
	[
		'bearer',
		'Bearer token list',
		/live token/,
		'Rotate',
		'dialog',
		'Rotate this token?',
		'Rotate',
		'rotateToken',
		'live',
	],
	[
		'bearer',
		'Bearer token list',
		/live token/,
		'Revoke',
		'alertdialog',
		'Revoke this token?',
		'Revoke',
		'revokeToken',
		'live',
	],
	[
		'bearer',
		'Bearer token list',
		/expired token/,
		'Delete permanently',
		'alertdialog',
		'Delete this token permanently?',
		'Delete permanently',
		'purgeToken',
		'expired',
	],
	[
		'consumer',
		'Consumer token list',
		/live read token/,
		'Rotate',
		'dialog',
		'Rotate this token?',
		'Rotate',
		'rotateConsumerToken',
		'aaaaaaaaaaaaaaaa',
	],
	[
		'consumer',
		'Consumer token list',
		/live read token/,
		'Revoke',
		'alertdialog',
		'Revoke this token?',
		'Revoke',
		'revokeConsumerToken',
		'aaaaaaaaaaaaaaaa',
	],
	[
		'consumer',
		'Consumer token list',
		/dead read token/,
		'Delete permanently',
		'alertdialog',
		'Delete this token permanently?',
		'Delete permanently',
		'purgeConsumerToken',
		'bbbbbbbbbbbbbbbb',
	],
] as const;

test.each(cardActions)(
	'a %s token card: %s %#',
	async (_kind, list, text, button, role, title, confirm, call, id) => {
		stub[call].mockResolvedValue({ ...live, value: 'raw' });
		const screen = await render(Settings, { data });
		const card = screen
			.getByRole('list', { name: list })
			.getByRole('listitem')
			.filter({ hasText: text });

		await card.getByRole('button', { name: button }).click();
		await screen
			.getByRole(role, { name: title })
			.getByRole('button', { name: confirm })
			.click();

		await vi.waitFor(() => expect(stub[call]).toHaveBeenCalledOnce());
		expect(stub[call].mock.calls[0]?.[0]).toBe(id);
	},
);

test('a card shows the status the server sent, and a consumer token its id', async () => {
	const screen = await render(Settings, { data });
	const bearer = screen
		.getByRole('list', { name: 'Bearer token list' })
		.getByRole('listitem')
		.filter({ hasText: /expired token/ });
	const consumer = screen
		.getByRole('list', { name: 'Consumer token list' })
		.getByRole('listitem')
		.filter({ hasText: /dead read token/ });

	await expect.poll(() => bearer.element().textContent).toContain('Expired');
	await expect.poll(() => consumer.element().textContent).toContain('Expired');
	await expect
		.element(consumer.getByTestId('token-id'))
		.toHaveTextContent('bbbbbbbbbbbbbbbb');
});

// alrayyes/Hush-Hush#896: the Settings lists page on their own, 10 at a time,
// each with its page in the address. Twelve of each kind push both past one
// page; the passkeys stay on one.
const pagerToken = (n: string) =>
	token({
		id: `bearer-${n}`,
		description: `pager bearer ${n}`,
		status: 'active',
		allowed_actions: ['rotate', 'revoke'],
	});
const pagerConsumerToken = (n: string) =>
	consumerToken({
		id: `consumer-${n.padStart(16, '0')}`,
		consumer: `pager-consumer-${n}`,
		description: `pager consumer ${n}`,
		status: 'active',
		allowed_actions: ['rotate', 'revoke'],
	});
const nn = Array.from({ length: 12 }, (_, i) => String(i).padStart(2, '0'));
const pagedData = {
	...data,
	tokens: nn.map(pagerToken),
	consumerTokens: nn.map(pagerConsumerToken),
};

const settingsPager = (
	screen: Awaited<ReturnType<typeof render>>,
	name: string,
) => screen.getByRole('navigation', { name });
const rowsLike = (screen: Awaited<ReturnType<typeof render>>, text: RegExp) =>
	screen.getByRole('row').filter({ hasText: text });

test('each list shows 10 rows under its own pager, and a list that fits one page has none', async () => {
	const screen = await render(Settings, { data: pagedData });

	await expect
		.poll(
			() => settingsPager(screen, 'Bearer tokens pages').element().textContent,
		)
		.toContain('Showing 1–10 of 12');
	await expect
		.poll(
			() =>
				settingsPager(screen, 'Consumer tokens pages').element().textContent,
		)
		.toContain('Showing 1–10 of 12');
	await expect
		.poll(() => rowsLike(screen, /pager bearer/).elements().length)
		.toBe(10);
	await expect
		.poll(() => rowsLike(screen, /pager consumer/).elements().length)
		.toBe(10);
	expect(settingsPager(screen, 'Passkeys pages').query()).toBeNull();
});

test('a list takes its page from its own parameter and leaves the other where it is', async () => {
	stub.url = new URL('http://localhost/settings?token_page=2');
	const screen = await render(Settings, { data: pagedData });

	await expect
		.poll(
			() => settingsPager(screen, 'Bearer tokens pages').element().textContent,
		)
		.toContain('Showing 11–12 of 12');
	await expect
		.poll(
			() =>
				settingsPager(screen, 'Consumer tokens pages').element().textContent,
		)
		.toContain('Showing 1–10 of 12');
	await expect
		.poll(() => rowsLike(screen, /pager bearer/).elements().length)
		.toBe(2);
});

test('the consumer list shows its own second page', async () => {
	stub.url = new URL('http://localhost/settings?consumer_token_page=2');
	const screen = await render(Settings, { data: pagedData });

	await expect
		.poll(() => rowsLike(screen, /pager consumer/).elements().length)
		.toBe(2);
	await expect
		.poll(() => rowsLike(screen, /pager bearer/).elements().length)
		.toBe(10);
});

test.each([
	[
		'bearer',
		'consumer_token_page=2',
		'Bearer tokens pages',
		'Page 2',
		'/settings?consumer_token_page=2&token_page=2',
	],
	[
		'consumer',
		'token_page=2',
		'Consumer tokens pages',
		'Next',
		'/settings?token_page=2&consumer_token_page=2',
	],
])(
	"the %s list's page links keep the other list's position",
	async (_kind, address, label, link, href) => {
		stub.url = new URL(`http://localhost/settings?${address}`);
		const screen = await render(Settings, { data: pagedData });

		await expect
			.element(settingsPager(screen, label).getByRole('link', { name: link }))
			.toHaveAttribute('href', href);
	},
);

test.each([
	['Bearer tokens pages', 'Next'],
	['Consumer tokens pages', 'Next'],
])('paging %s does not jump the page back to the top', async (label, link) => {
	const screen = await render(Settings, { data: pagedData });

	await expect
		.element(settingsPager(screen, label).getByRole('link', { name: link }))
		.toHaveAttribute('data-sveltekit-reset', 'false');
});

test.each([
	['token_page', 'Bearer tokens pages'],
	['consumer_token_page', 'Consumer tokens pages'],
])('a %s past the end shows the last page', async (param, label) => {
	stub.url = new URL(`http://localhost/settings?${param}=99`);
	const screen = await render(Settings, { data: pagedData });

	await expect
		.poll(() => settingsPager(screen, label).element().textContent)
		.toContain('Showing 11–12 of 12');
});

test('filtered to one consumer, the list fits one page and shows no pager', async () => {
	stub.url = new URL('http://localhost/settings?consumer=pager-consumer-03');
	const screen = await render(Settings, { data: pagedData });

	await expect
		.poll(() => rowsLike(screen, /pager consumer/).elements().length)
		.toBe(1);
	expect(settingsPager(screen, 'Consumer tokens pages').query()).toBeNull();
});

test('the phone cards page the same way', async () => {
	const screen = await render(Settings, { data: pagedData });

	const bearerCards = screen
		.getByRole('list', { name: 'Bearer token list' })
		.getByRole('listitem');
	const consumerCards = screen
		.getByRole('list', { name: 'Consumer token list' })
		.getByRole('listitem');
	await expect.poll(() => bearerCards.elements().length).toBe(10);
	await expect.poll(() => consumerCards.elements().length).toBe(10);
	await expect
		.poll(
			() => settingsPager(screen, 'Bearer tokens pages').element().textContent,
		)
		.toContain('Page 1 of 2');
});

// alrayyes/hush-hush#537: the create dialog's lifetime field takes its default
// and limits from the API spec. The single-select picker (max 1, no key hint)
// and the repeat warning (#711) are the page's.
const openConsumerTokenDialog = async (
	screen: Awaited<ReturnType<typeof render>>,
) => {
	await screen.getByRole('button', { name: 'New consumer token' }).click();

	return screen.getByRole('dialog', { name: 'Create a consumer token' });
};
const pickConsumer = async (
	screen: Awaited<ReturnType<typeof render>>,
	name: string,
) => {
	await screen.getByLabelText('Consumer').fill(name);
	await screen.getByRole('option', { name: `Add "${name}"` }).click();
};

test('the lifetime defaults to 90 days, between 1 and 365, and says so', async () => {
	const screen = await render(Settings, { data });
	const dialog = await openConsumerTokenDialog(screen);

	const ttl = dialog.getByLabelText('Valid for (days)');
	await expect.element(ttl).toHaveValue(90);
	await expect.element(ttl).toHaveAttribute('min', '1');
	await expect.element(ttl).toHaveAttribute('max', '365');
	await expect
		.poll(() => dialog.element().textContent)
		.toContain('From 1 to 365 days.');
	await ttl.fill('366');
	expect((ttl.element() as HTMLInputElement).validity.rangeOverflow).toBe(true);
});

test('picking a consumer takes the picker away, with no key hint', async () => {
	const screen = await render(Settings, { data });
	const dialog = await openConsumerTokenDialog(screen);
	expect(
		(dialog.element().querySelector('button[type=submit]') as HTMLButtonElement)
			.disabled,
	).toBe(true);

	await pickConsumer(screen, 'ci-runner');

	await expect.poll(() => dialog.getByLabelText('Consumer').query()).toBeNull();
	expect(dialog.element().textContent).toContain('ci-runner');
	expect(dialog.element().textContent).not.toContain('(no key)');
	expect(
		(dialog.element().querySelector('button[type=submit]') as HTMLButtonElement)
			.disabled,
	).toBe(false);
});

test('repeating an active token warns, naming it, and Create becomes Create anyway', async () => {
	const screen = await render(Settings, {
		data: {
			...data,
			consumerTokens: [
				consumerToken({
					id: 'cccccccccccccccc',
					consumer: 'ci-runner',
					description: 'deploy read token',
					status: 'active',
					allowed_actions: ['rotate', 'revoke'],
				}),
			],
		},
	});
	const dialog = await openConsumerTokenDialog(screen);
	await pickConsumer(screen, 'ci-runner');
	expect(dialog.getByRole('alert').query()).toBeNull();

	await dialog.getByLabelText('Description').fill('deploy read token');

	await expect
		.poll(() => dialog.getByRole('alert').element().textContent)
		.toContain('ci-runner already has an active token');
	expect(dialog.getByRole('alert').element().textContent).toContain(
		'ID cccccccccccccccc',
	);
	await expect
		.element(dialog.getByRole('button', { name: 'Create anyway' }))
		.toBeVisible();
});

test('creating sends the consumer, description and lifetime, and shows the value once', async () => {
	stub.createConsumerToken.mockResolvedValue({
		...consumerToken({ id: 'dddddddddddddddd', consumer: 'ci-runner' }),
		value: 'hh_consumer_secret_value',
	});
	const writeText = vi.fn().mockResolvedValue(undefined);
	vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(writeText);
	const screen = await render(Settings, { data });
	const dialog = await openConsumerTokenDialog(screen);
	await pickConsumer(screen, 'ci-runner');
	await dialog.getByLabelText('Description').fill('deploy read token');
	await dialog.getByLabelText('Valid for (days)').fill('30');

	await dialog.getByRole('button', { name: 'Create', exact: true }).click();

	await vi.waitFor(() =>
		expect(stub.createConsumerToken).toHaveBeenCalledWith(
			'ci-runner',
			'deploy read token',
			30 * 86_400,
		),
	);
	const created = screen.getByRole('dialog', { name: 'Token created' });
	await expect
		.element(created.getByLabelText('Token value'))
		.toHaveValue('hh_consumer_secret_value');
	await created.getByRole('button', { name: 'Copy token' }).click();
	await vi.waitFor(() =>
		expect(writeText).toHaveBeenCalledWith('hh_consumer_secret_value'),
	);
});
