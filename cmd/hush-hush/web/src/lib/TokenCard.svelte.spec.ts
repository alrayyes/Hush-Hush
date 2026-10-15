import { expect, test, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import TokenCard from './TokenCard.svelte';

// What a token card shows and offers is what the server says (status and
// allowed_actions), never what the browser's own clock suggests
// (alrayyes/hush-hush#536). tokens.spec.ts pins the functions; this pins that
// the card uses them.
const pastExpiry = '2020-01-01T00:00:00Z';
const token = {
	description: 'deploy',
	created_at: '2019-01-01T00:00:00Z',
	expires_at: pastExpiry,
	revoked: false,
};
const handlers = { onrotate: vi.fn(), onrevoke: vi.fn(), onpurge: vi.fn() };

test('a token the server calls active keeps Rotate and Revoke when the browser clock says it expired', async () => {
	const screen = await render(TokenCard, {
		token: {
			...token,
			status: 'active',
			allowed_actions: ['rotate', 'revoke'],
		},
		subtitle: 'ci-runner',
		...handlers,
	});

	await expect.element(screen.getByText('Active')).toBeVisible();
	await expect
		.element(screen.getByRole('button', { name: 'Rotate' }))
		.toBeVisible();
	await expect
		.element(screen.getByRole('button', { name: 'Revoke' }))
		.toBeVisible();
	await expect
		.element(screen.getByRole('button', { name: 'Delete permanently' }))
		.not.toBeInTheDocument();
});

test('a token the server calls expired offers only Delete permanently', async () => {
	const screen = await render(TokenCard, {
		token: { ...token, status: 'expired', allowed_actions: ['purge'] },
		subtitle: 'ci-runner',
		...handlers,
	});

	await expect
		.element(screen.getByTestId('ttl-badge'))
		.toHaveTextContent('expired');
	await expect
		.element(screen.getByRole('button', { name: 'Delete permanently' }))
		.toBeVisible();
	await expect
		.element(screen.getByRole('button', { name: 'Rotate' }))
		.not.toBeInTheDocument();
	await expect
		.element(screen.getByRole('button', { name: 'Revoke' }))
		.not.toBeInTheDocument();
});

test('clicking a button calls its own handler', async () => {
	const onrevoke = vi.fn();
	const screen = await render(TokenCard, {
		token: {
			...token,
			status: 'active',
			allowed_actions: ['rotate', 'revoke'],
		},
		subtitle: 'ci-runner',
		...handlers,
		onrevoke,
	});

	await screen.getByRole('button', { name: 'Revoke' }).click();

	expect(onrevoke).toHaveBeenCalledOnce();
});
