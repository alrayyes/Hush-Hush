import { beforeEach, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { ApiError, type AuditLogEntry } from '#lib/api.js';
import AuditLog from './+page.svelte';

// The page reads the address through $app/state and navigates through
// $app/navigation, and fetches through #lib/api.js. All three are stood in
// for, so a test holds a fetch open and settles it when it chooses.
const stub = vi.hoisted(() => ({
	queryAuditLog: vi.fn(),
}));

vi.mock('$app/navigation', () => ({
	afterNavigate: vi.fn(),
	replaceState: vi.fn(),
}));
vi.mock('$app/state', () => ({
	page: {
		url: new URL('http://localhost/audit-log'),
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
