import { beforeEach, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { ApiError, type AuditLogEntry } from '#lib/api.js';
import AuditLog from './+page.svelte';

// The page reads the address through $app/state and navigates through
// $app/navigation, and fetches through #lib/api.js. All three are stood in
// for, so a test holds a fetch open and settles it when it chooses.
const stub = vi.hoisted(() => ({
	queryAuditLog: vi.fn(),
	afterNavigate: vi.fn(),
	replaceState: vi.fn(),
	url: new URL('http://localhost/audit-log'),
}));

vi.mock('$app/navigation', () => ({
	afterNavigate: stub.afterNavigate,
	replaceState: stub.replaceState,
}));
vi.mock('$app/state', () => ({
	page: {
		get url() {
			return stub.url;
		},
		state: {},
	},
}));
vi.mock('#lib/api.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.js')>()),
	queryAuditLog: stub.queryAuditLog,
}));

const entry = (id: number, objectId: string): AuditLogEntry => ({
	id,
	object_id: objectId,
	action: 'read',
	timestamp: '2026-01-01T00:00:00Z',
	ip: '203.0.113.1',
});
const newestFirst = [entry(3, 'newest_a'), entry(2, 'newest_b')];
const oldestFirst = [entry(2, 'oldest_a'), entry(3, 'oldest_b')];

const data = {
	// The layout's own fields, which the page's data type includes.
	environment: 'test',
	version: 'v1.2.3',
	healthy: true,
	authenticated: true,
	entries: newestFirst,
	hasMore: false,
	size: 25,
	filterOptions: { object_ids: [], actors: [], callers: [] },
};

// A fetch the test settles by hand.
function held<T>() {
	let resolve: (value: T) => void = () => {};
	let reject: (reason: unknown) => void = () => {};
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});

	return { promise, resolve, reject };
}

beforeEach(() => {
	stub.queryAuditLog.mockReset();
	stub.afterNavigate.mockReset();
	stub.replaceState.mockReset();
	stub.url = new URL('http://localhost/audit-log');
});

const orderButton = (screen: Awaited<ReturnType<typeof render>>) =>
	screen
		.getByRole('columnheader', { name: 'Timestamp' })
		.getByRole('button', { name: /Timestamp/ });

// alrayyes/Hush-Hush#922: two changes made before the first fetch resolves
// show the second one's results, even when the first arrives last.
test('the newer order change wins when the older fetch arrives last', async () => {
	const toOldest = held<AuditLogEntry[]>();
	const backToNewest = held<AuditLogEntry[]>();
	stub.queryAuditLog
		.mockReturnValueOnce(toOldest.promise)
		.mockReturnValueOnce(backToNewest.promise);

	// Time is the test's: a late response the page applies after a timer or a
	// frame would otherwise land after the assertion and pass it.
	vi.useFakeTimers({ toFake: ['setTimeout', 'requestAnimationFrame'] });
	let screen: Awaited<ReturnType<typeof render>>;
	try {
		screen = await render(AuditLog, { data });
		await orderButton(screen).click();
		await orderButton(screen).click();
		backToNewest.resolve(newestFirst);
		await vi.advanceTimersByTimeAsync(1000);
		toOldest.resolve(oldestFirst);
		await vi.advanceTimersByTimeAsync(1000);

		expect(screen.getByRole('cell', { name: 'oldest_a' }).query()).toBeNull();
	} finally {
		vi.useRealTimers();
	}

	await expect
		.element(screen.getByRole('columnheader', { name: 'Timestamp' }))
		.toHaveAttribute('aria-sort', 'descending');
	await expect
		.element(screen.getByRole('cell', { name: 'newest_a' }))
		.toBeInTheDocument();
});

// alrayyes/Hush-Hush#921: a change whose fetch fails puts the controls back
// and leaves the table alone, so the controls, the table and the curl command
// still describe one query.
test('a failed order change puts the controls back and keeps the table', async () => {
	stub.queryAuditLog.mockRejectedValueOnce(new Error('network'));
	const screen = await render(AuditLog, { data });

	await orderButton(screen).click();

	await expect.element(screen.getByRole('alert')).toBeVisible();
	await expect
		.element(screen.getByRole('columnheader', { name: 'Timestamp' }))
		.toHaveAttribute('aria-sort', 'descending');
	const curl = screen.getByRole('group', { name: 'Query with curl' });
	await expect.poll(() => curl.element().textContent).toContain('order=desc');
	await expect
		.element(screen.getByRole('cell', { name: 'newest_a' }))
		.toBeInTheDocument();
	await expect
		.element(screen.getByRole('table'))
		.toHaveAttribute('aria-busy', 'false');
});

test('the change that follows a failed one goes through and clears the alert', async () => {
	stub.queryAuditLog
		.mockRejectedValueOnce(new Error('network'))
		.mockResolvedValueOnce(oldestFirst);
	const screen = await render(AuditLog, { data });

	await orderButton(screen).click();
	await expect.element(screen.getByRole('alert')).toBeVisible();
	await orderButton(screen).click();

	await expect
		.element(screen.getByRole('columnheader', { name: 'Timestamp' }))
		.toHaveAttribute('aria-sort', 'ascending');
	await expect
		.element(screen.getByRole('cell', { name: 'oldest_a' }))
		.toBeInTheDocument();
	await expect.element(screen.getByRole('alert')).not.toBeInTheDocument();
});

test('a failed filter change puts the filter back too', async () => {
	stub.queryAuditLog.mockRejectedValueOnce(new Error('network'));
	const screen = await render(AuditLog, { data });
	const from = screen.getByLabelText('From');

	await from.fill('2026-01-01T00:00');

	await expect.element(screen.getByRole('alert')).toBeVisible();
	await expect.element(from).toHaveValue('');
});

test('a failure puts the controls back to what the table last showed, not to the start', async () => {
	stub.queryAuditLog
		.mockResolvedValueOnce(oldestFirst)
		.mockRejectedValueOnce(new Error('network'));
	const screen = await render(AuditLog, { data });

	await orderButton(screen).click();
	await expect
		.element(screen.getByRole('columnheader', { name: 'Timestamp' }))
		.toHaveAttribute('aria-sort', 'ascending');
	await orderButton(screen).click();

	await expect.element(screen.getByRole('alert')).toBeVisible();
	await expect
		.element(screen.getByRole('columnheader', { name: 'Timestamp' }))
		.toHaveAttribute('aria-sort', 'ascending');
});

test("the server's own message is what the alert says", async () => {
	stub.queryAuditLog.mockRejectedValueOnce(
		new ApiError(500, 'the audit log is unavailable'),
	);
	const screen = await render(AuditLog, { data });

	await orderButton(screen).click();

	const alert = screen.getByRole('alert');
	await expect
		.poll(() => alert.element().textContent)
		.toContain('the audit log is unavailable');
});

// alrayyes/Hush-Hush#897: rows per page. The selector shows once another page
// exists, so these entries say there is one.
const morePages = { ...data, hasMore: true };
const sizeSelect = (screen: Awaited<ReturnType<typeof render>>) =>
	screen.getByRole('combobox', { name: 'Rows per page' });

test('the audit log offers 25, 50 and 100 rows per page', async () => {
	const screen = await render(AuditLog, { data: morePages });

	await sizeSelect(screen).click();

	const options = screen.getByRole('listbox').getByRole('option');
	await expect
		.poll(() => options.elements().map((option) => option.textContent?.trim()))
		.toEqual(['25', '50', '100']);
});

test('picking a size asks for one row more than it, and puts it in the address', async () => {
	stub.queryAuditLog.mockResolvedValue(newestFirst);
	const screen = await render(AuditLog, { data: morePages });

	await sizeSelect(screen).click();
	await screen.getByRole('option', { name: '50', exact: true }).click();

	await vi.waitFor(() =>
		expect(stub.queryAuditLog).toHaveBeenCalledWith(
			expect.objectContaining({ limit: 51 }),
		),
	);
	const [url] = stub.replaceState.mock.calls[0] ?? [];
	expect((url as URL).searchParams.get('size')).toBe('50');
	const curl = screen.getByRole('group', { name: 'Query with curl' });
	await expect.poll(() => curl.element().textContent).toContain('limit=50');
});

test('picking the default size takes the size out of the address', async () => {
	stub.url = new URL('http://localhost/audit-log?size=50');
	stub.queryAuditLog.mockResolvedValue(newestFirst);
	const screen = await render(AuditLog, {
		data: { ...morePages, size: 50 },
	});

	await sizeSelect(screen).click();
	await screen.getByRole('option', { name: '25', exact: true }).click();

	await vi.waitFor(() => expect(stub.replaceState).toHaveBeenCalled());
	const [url] = stub.replaceState.mock.calls[0] ?? [];
	expect((url as URL).searchParams.has('size')).toBe(false);
});

// The size is the address's: following the nav link to /audit-log, which has
// no ?size=, goes back to the default.
test('following a link to another size reloads at that size', async () => {
	stub.queryAuditLog.mockResolvedValue(newestFirst);
	await render(AuditLog, { data: { ...morePages, size: 50 } });
	const onNavigate = stub.afterNavigate.mock.calls[0]?.[0];

	onNavigate({ to: { url: new URL('http://localhost/audit-log') } });

	await vi.waitFor(() =>
		expect(stub.queryAuditLog).toHaveBeenCalledWith(
			expect.objectContaining({ limit: 26 }),
		),
	);
});

test('following a link to the size it already has changes nothing', async () => {
	// Time is the test's, so a reload deferred by a timer or a frame is seen.
	vi.useFakeTimers({ toFake: ['setTimeout', 'requestAnimationFrame'] });
	try {
		await render(AuditLog, { data: morePages });
		const onNavigate = stub.afterNavigate.mock.calls[0]?.[0];

		onNavigate({ to: { url: new URL('http://localhost/audit-log') } });
		await vi.advanceTimersByTimeAsync(1000);

		expect(stub.queryAuditLog).not.toHaveBeenCalled();
	} finally {
		vi.useRealTimers();
	}
});

// alrayyes/hush-hush#323: the object and actor filters are menus of what the
// log holds. Picking one shows its chip and fetches with it; removing the chip
// or picking the empty choice fetches without.
const filterData = {
	...data,
	filterOptions: {
		object_ids: ['mattermost_deploy_webhook', 'other_secret'],
		actors: [
			{ value: 'admin', label: 'admin' },
			{ value: 'a1b2c3d4e5f6a7b8', label: 'token:a1b2c3d4e5f6a7b8' },
		],
		callers: [],
	},
};
const chip = (screen: Awaited<ReturnType<typeof render>>, text: string) =>
	screen.getByRole('listitem').filter({ hasText: text });
const actorMenu = (screen: Awaited<ReturnType<typeof render>>) =>
	screen.getByRole('combobox', { name: 'Actor', exact: true });
const objectMenu = (screen: Awaited<ReturnType<typeof render>>) =>
	screen.getByRole('combobox', { name: 'Object id', exact: true });

test('the Actor menu offers Any actor and the actors the server listed, and nothing else', async () => {
	const screen = await render(AuditLog, { data: filterData });

	await actorMenu(screen).click();

	const options = screen
		.getByRole('listbox', { name: 'Actor' })
		.getByRole('option');
	await expect
		.poll(() => options.elements().map((o) => o.textContent?.trim()))
		.toEqual(['Any actor', 'admin', 'token:a1b2c3d4e5f6a7b8']);
});

test('picking an actor shows its chip and fetches with that actor', async () => {
	stub.queryAuditLog.mockResolvedValue(newestFirst);
	const screen = await render(AuditLog, { data: filterData });

	await actorMenu(screen).click();
	await screen.getByRole('option', { name: 'admin', exact: true }).click();

	await expect.element(chip(screen, 'actor: admin')).toBeVisible();
	await vi.waitFor(() =>
		expect(stub.queryAuditLog).toHaveBeenCalledWith(
			expect.objectContaining({ actor: 'admin' }),
		),
	);
});

test('the chip names a token actor by its label, and fetches with its id', async () => {
	stub.queryAuditLog.mockResolvedValue(newestFirst);
	const screen = await render(AuditLog, { data: filterData });

	await actorMenu(screen).click();
	await screen
		.getByRole('option', { name: 'token:a1b2c3d4e5f6a7b8', exact: true })
		.click();

	await expect
		.element(chip(screen, 'actor: token:a1b2c3d4e5f6a7b8'))
		.toBeVisible();
	await vi.waitFor(() =>
		expect(stub.queryAuditLog).toHaveBeenCalledWith(
			expect.objectContaining({ actor: 'a1b2c3d4e5f6a7b8' }),
		),
	);
});

test('choosing Any actor afterwards takes the chip away and fetches without an actor', async () => {
	stub.queryAuditLog.mockResolvedValue(newestFirst);
	const screen = await render(AuditLog, { data: filterData });
	await actorMenu(screen).click();
	await screen.getByRole('option', { name: 'admin', exact: true }).click();
	await expect.element(chip(screen, 'actor: admin')).toBeVisible();

	await actorMenu(screen).click();
	await screen.getByRole('option', { name: 'Any actor' }).click();

	await expect.poll(() => chip(screen, 'actor:').query()).toBeNull();
	await vi.waitFor(() =>
		expect(stub.queryAuditLog).toHaveBeenLastCalledWith(
			expect.not.objectContaining({ actor: 'admin' }),
		),
	);
});

test('picking an object shows its chip, and the chip removes it and fetches without', async () => {
	stub.queryAuditLog.mockResolvedValue(newestFirst);
	const screen = await render(AuditLog, { data: filterData });

	await objectMenu(screen).click();
	await screen
		.getByRole('option', { name: 'mattermost_deploy_webhook', exact: true })
		.click();
	await expect
		.element(chip(screen, 'object: mattermost_deploy_webhook'))
		.toBeVisible();
	await vi.waitFor(() =>
		expect(stub.queryAuditLog).toHaveBeenCalledWith(
			expect.objectContaining({ object_id: 'mattermost_deploy_webhook' }),
		),
	);

	await screen.getByRole('button', { name: 'Remove object filter' }).click();

	await expect.poll(() => chip(screen, 'object:').query()).toBeNull();
	await vi.waitFor(() =>
		expect(stub.queryAuditLog).toHaveBeenLastCalledWith(
			expect.not.objectContaining({ object_id: 'mattermost_deploy_webhook' }),
		),
	);
});

test("the actor chip's button removes it and fetches without an actor", async () => {
	stub.queryAuditLog.mockResolvedValue(newestFirst);
	const screen = await render(AuditLog, { data: filterData });
	await actorMenu(screen).click();
	await screen.getByRole('option', { name: 'admin', exact: true }).click();
	await expect.element(chip(screen, 'actor: admin')).toBeVisible();

	await screen.getByRole('button', { name: 'Remove actor filter' }).click();

	await expect.poll(() => chip(screen, 'actor:').query()).toBeNull();
	await vi.waitFor(() =>
		expect(stub.queryAuditLog).toHaveBeenLastCalledWith(
			expect.not.objectContaining({ actor: 'admin' }),
		),
	);
});
