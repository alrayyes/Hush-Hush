import { beforeEach, expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { AuditLogEntry, ObjectMetadata } from '#lib/api.js';
import SecretDetail from './+page.svelte';

// The page reads its origin from $app/state for the CLI snippet.
vi.mock('$app/state', () => ({
	page: {
		url: new URL('http://localhost:4173/secrets/mattermost_deploy_webhook'),
	},
}));

const KEY =
	'age1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgp8t5jxw';
const layout = {
	// The layout's own fields, which the page's data type includes.
	environment: 'test',
	version: 'v1.2.3',
	healthy: true,
	authenticated: true,
};
const event = (id: number, action: AuditLogEntry['action']): AuditLogEntry => ({
	id,
	object_id: 'mattermost_deploy_webhook',
	action,
	timestamp: '2026-01-01T00:00:00Z',
	ip: '203.0.113.1',
	actor_type: 'session',
	actor_id: 'user-1',
});
const found = {
	...layout,
	slug: 'mattermost_deploy_webhook',
	notFound: false as const,
	chooseVariant: false as const,
	id: '10000000-0000-4000-8000-000000000001',
	value: btoa('sealed-bytes'),
	events: [event(3, 'read'), event(2, 'update'), event(1, 'create')],
	consumers: [
		{ name: 'homelab', publicKey: KEY },
		{ name: 'ci', publicKey: undefined },
	],
};

const writeText = vi.fn();
beforeEach(() => {
	writeText.mockReset();
	writeText.mockResolvedValue(undefined);
	vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(writeText);
});

// alrayyes/hush-hush#480: the detail page Inspect opens.
test('shows the sealed ciphertext, and Copy puts exactly that on the clipboard', async () => {
	const screen = await render(SecretDetail, { data: found });

	await expect
		.element(
			screen.getByRole('textbox', { name: 'Sealed ciphertext (base64)' }),
		)
		.toHaveValue(found.value);
	await screen.getByRole('button', { name: 'Copy sealed ciphertext' }).click();

	await vi.waitFor(() => expect(writeText).toHaveBeenCalledWith(found.value));
	await expect
		.element(screen.getByRole('button', { name: 'Copy sealed ciphertext' }))
		.toHaveTextContent('Copied');
});

test("the CLI snippet uses the CLI's real flags and a placeholder key", async () => {
	const screen = await render(SecretDetail, { data: found });

	const snippet = screen.getByRole('group', { name: 'Fetch with the CLI' });
	await expect
		.poll(() => snippet.element().textContent)
		.toContain(
			'hush-hush-cli get mattermost_deploy_webhook --identity "AGE-SECRET-KEY-1..."',
		);
	expect(snippet.element().textContent).not.toContain('-i ~/.age');
	expect(snippet.element().textContent).toContain(
		'HUSH_HUSH_SERVER=http://localhost:4173',
	);
});

test('lists each consumer with its key truncated, never whole', async () => {
	const screen = await render(SecretDetail, { data: found });

	const section = screen.getByRole('region', { name: 'Authorized consumers' });
	const items = section.getByRole('listitem');
	await expect.poll(() => items.elements().length).toBe(2);
	const text = section.element().textContent ?? '';
	expect(text).toContain(`${KEY.slice(0, 10)}…${KEY.slice(-6)}`);
	expect(text).not.toContain(KEY);
	expect(text).toContain('homelab');
	expect(text).toContain('not registered');
});

test('lists the recent activity in the order the server gave it', async () => {
	const screen = await render(SecretDetail, { data: found });

	const rows = screen
		.getByRole('region', { name: 'Recent activity' })
		.getByRole('listitem');
	await expect.poll(() => rows.elements().length).toBe(3);
	expect(rows.elements().map((row) => row.textContent)).toEqual([
		expect.stringContaining('read'),
		expect.stringContaining('update'),
		expect.stringContaining('create'),
	]);
});

test('an unknown secret says it was not found instead of rendering empty', async () => {
	const screen = await render(SecretDetail, {
		data: { ...layout, slug: 'does_not_exist', notFound: true as const },
	});

	await expect
		.poll(() => screen.getByRole('alert').element().textContent)
		.toMatch(/not found/i);
});

test('a name with several variants offers the choice, each linked by id', async () => {
	const variant = (id: string, usedBy: string[]): ObjectMetadata => ({
		id,
		slug: 'release_token',
		used_by: usedBy,
		tags: [],
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
	});
	const screen = await render(SecretDetail, {
		data: {
			...layout,
			slug: 'release_token',
			notFound: false as const,
			chooseVariant: true as const,
			variants: [
				variant('aaaaaaaa-0000-4000-8000-000000000001', ['consumer_a']),
				variant('aaaaaaaa-0000-4000-8000-000000000002', []),
			],
		},
	});

	const list = screen
		.getByRole('region', { name: 'Variants' })
		.getByRole('listitem');
	await expect.poll(() => list.elements().length).toBe(2);
	await expect
		.element(screen.getByRole('link', { name: 'consumer_a' }))
		.toHaveAttribute(
			'href',
			'/secrets/release_token?id=aaaaaaaa-0000-4000-8000-000000000001',
		);
	await expect
		.element(screen.getByRole('link', { name: 'No consumers' }))
		.toHaveAttribute(
			'href',
			'/secrets/release_token?id=aaaaaaaa-0000-4000-8000-000000000002',
		);
});
