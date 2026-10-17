import { createRawSnippet } from 'svelte';
import { beforeEach, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Layout from './+layout.svelte';

// The chrome reads the address through $app/state and logs out through
// #lib/api.js and $app/navigation. All are stood in for, so a test sets the
// path it loads at and sees what Log out calls.
const stub = vi.hoisted(() => ({
	url: new URL('http://localhost/'),
	logout: vi.fn(),
	invalidate: vi.fn(),
	goto: vi.fn(),
}));

vi.mock('$app/state', () => ({
	page: {
		get url() {
			return stub.url;
		},
	},
}));
vi.mock('$app/navigation', () => ({
	goto: stub.goto,
	invalidate: stub.invalidate,
}));
vi.mock('#lib/api.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.js')>()),
	logout: stub.logout,
}));

const children = createRawSnippet(() => ({
	render: () => '<main>page</main>',
}));
const data = {
	version: 'v1.2.3',
	healthy: true,
	environment: 'e2e / local',
	authenticated: true,
};

beforeEach(() => {
	for (const fn of [stub.logout, stub.invalidate, stub.goto]) fn.mockReset();
	stub.url = new URL('http://localhost/');
});

// Both navs are in the DOM at every width and CSS picks one, so they are
// found by their landmark label rather than by what is displayed.
const navLinks = (label: string) => [
	...document.querySelectorAll<HTMLAnchorElement>(
		`nav[aria-label="${label}"] a`,
	),
];
const current = (links: HTMLAnchorElement[]) =>
	links
		.filter((link) => link.getAttribute('aria-current') === 'page')
		.map((link) => link.textContent?.trim());

// alrayyes/hush-hush#481: the top bar names the app and the instance, and
// says whether /healthz answered.
test('the top bar carries the brand, the instance label and an ACTIVE pill', async () => {
	const screen = await render(Layout, { data, children });

	const bar = screen.getByRole('banner');
	await expect.poll(() => bar.element().textContent).toContain('HUSH-HUSH');
	expect(bar.element().textContent).toContain('e2e / local');
	expect(bar.element().textContent).toContain('ACTIVE');
	expect(bar.element().textContent).not.toContain('OFFLINE');
	const brand = [...bar.element().querySelectorAll('span')].find(
		(span) => span.textContent === 'HUSH-HUSH',
	);
	expect(brand?.parentElement?.children).toHaveLength(2);
});

test('a failed health check reads OFFLINE and claims neither ACTIVE nor an instance', async () => {
	const screen = await render(Layout, {
		data: { ...data, healthy: false, environment: undefined },
		children,
	});

	const bar = screen.getByRole('banner');
	await expect.poll(() => bar.element().textContent).toContain('OFFLINE');
	expect(bar.element().textContent).not.toContain('ACTIVE');
	expect(bar.element().textContent).not.toContain('e2e / local');
	// No empty box stands where the label would be: the brand is alone.
	const brand = [...bar.element().querySelectorAll('span')].find(
		(span) => span.textContent === 'HUSH-HUSH',
	);
	expect(brand?.parentElement?.children).toHaveLength(1);
});

// #301: the current page's link carries aria-current="page", in the top nav
// and in the phone tab bar, and it moves with the address.
test.each([
	['/', 'Secrets'],
	['/consumers', 'Consumers'],
	['/audit-log', 'Audit log'],
	['/settings', 'Settings'],
])('at %s, only %s is the current page in both navs', async (path, label) => {
	stub.url = new URL(`http://localhost${path}`);
	await render(Layout, { data, children });

	for (const nav of ['Primary', 'Primary (mobile)']) {
		await expect.poll(() => navLinks(nav).length).toBe(4);
		expect(current(navLinks(nav))).toEqual([label]);
	}
});

test('a path under a section is not the section: only an exact path is current', async () => {
	stub.url = new URL('http://localhost/secrets/some_secret');
	await render(Layout, { data, children });

	await expect.poll(() => navLinks('Primary (mobile)').length).toBe(4);
	expect(current(navLinks('Primary (mobile)'))).toEqual([]);
	expect(current(navLinks('Primary'))).toEqual([]);
});

test('signed out, there is no nav, no tab bar and no account menu', async () => {
	const screen = await render(Layout, {
		data: { ...data, authenticated: false },
		children,
	});

	await expect.poll(() => screen.getByRole('banner').query()).not.toBeNull();
	expect(navLinks('Primary')).toEqual([]);
	expect(navLinks('Primary (mobile)')).toEqual([]);
	expect(
		document.querySelector('button[aria-label="Account menu"]'),
	).toBeNull();
});

// #481: below md Log out and the theme toggle sit behind the account menu.
test('the account menu holds Log out and the theme toggle, and Log out leaves for /login', async () => {
	stub.logout.mockResolvedValue(undefined);
	stub.invalidate.mockResolvedValue(undefined);
	const screen = await render(Layout, { data, children });

	await screen.getByRole('button', { name: 'Account menu' }).click();

	const dialog = screen.getByRole('dialog', { name: 'Account' });
	await expect
		.element(
			dialog.getByRole('button', { name: /switch to (dark|light) mode/i }),
		)
		.toBeVisible();
	await dialog.getByRole('button', { name: 'Log out' }).click();
	await vi.waitFor(() => expect(stub.goto).toHaveBeenCalledWith('/login'));
	expect(stub.logout).toHaveBeenCalledOnce();
	expect(stub.invalidate).toHaveBeenCalledWith('app:auth');
});
