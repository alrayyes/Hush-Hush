import { beforeEach, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ErrorPage from './+error.svelte';

// The page reads the failure's status and the layout's data from $app/state.
const stub = vi.hoisted(() => ({
	status: 404,
	authenticated: true,
}));
vi.mock('$app/state', () => ({
	page: {
		get status() {
			return stub.status;
		},
		get data() {
			return { authenticated: stub.authenticated };
		},
	},
}));

beforeEach(() => {
	stub.status = 404;
	stub.authenticated = true;
});

// alrayyes/hush-hush#790: the app's own error page, in the shell.
test('a 404 says the page was not found, plainly, with the status beside it', async () => {
	const screen = await render(ErrorPage);

	await expect
		.element(screen.getByRole('heading', { level: 1, name: 'Page not found' }))
		.toBeVisible();
	await expect.element(screen.getByText('404', { exact: true })).toBeVisible();
});

test('a 500 says something went wrong and shows no internals', async () => {
	stub.status = 500;
	const screen = await render(ErrorPage);

	await expect
		.element(
			screen.getByRole('heading', { level: 1, name: 'Something went wrong' }),
		)
		.toBeVisible();
	await expect.element(screen.getByText('500', { exact: true })).toBeVisible();
	expect(screen.container.textContent).not.toMatch(
		/\bat \S+\.(js|ts|svelte)|node_modules/,
	);
});

test('signed in, it offers the secrets list and the audit log', async () => {
	const screen = await render(ErrorPage);

	await expect
		.element(screen.getByRole('link', { name: 'Open secrets' }))
		.toHaveAttribute('href', '/');
	await expect
		.element(screen.getByRole('link', { name: 'Open audit log' }))
		.toHaveAttribute('href', '/audit-log');
});

test('signed out, it offers sign in and not the audit log', async () => {
	stub.authenticated = false;
	const screen = await render(ErrorPage);

	await expect
		.element(screen.getByRole('link', { name: 'Go to sign in' }))
		.toHaveAttribute('href', '/');
	expect(
		screen.getByRole('link', { name: 'Open audit log' }).query(),
	).toBeNull();
});

test('Copy CLI command writes the whole command, with a placeholder key', async () => {
	const writeText = vi.fn().mockResolvedValue(undefined);
	vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(writeText);
	const screen = await render(ErrorPage);

	await screen.getByRole('button', { name: 'Copy CLI command' }).click();

	await vi.waitFor(() =>
		expect(writeText).toHaveBeenCalledWith(
			'hush-hush-cli get <slug> --identity "AGE-SECRET-KEY-1..."',
		),
	);
	await expect
		.element(screen.getByRole('button', { name: 'Copied CLI command' }))
		.toBeVisible();
});
