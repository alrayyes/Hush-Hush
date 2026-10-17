import { expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ConsumerList from './ConsumerList.svelte';
import { consumersHref } from './consumers';

// A hundred consumers, three of them the dotfiles repos.
const many = [
	...Array.from(
		{ length: 97 },
		(_, i) => `consumer_${String(i).padStart(3, '0')}`,
	),
	'dotfiles',
	'dotfiles-claude',
	'dotfiles-work',
];

test('a long list says how many are shown and filters by name', async () => {
	const screen = await render(ConsumerList, { consumers: many, id: 'c' });

	const status = screen.getByRole('status');
	await expect
		.poll(() => status.element().textContent?.trim())
		.toBe('100 of 100 shown');
	await screen.getByLabelText('Filter consumers').fill('dotfiles');

	await expect
		.poll(() => status.element().textContent?.trim())
		.toBe('3 of 100 shown');
	await expect
		.poll(() => screen.getByRole('listitem').elements().length)
		.toBe(3);
});

test('a filter that matches nothing says so, naming it', async () => {
	const screen = await render(ConsumerList, { consumers: many, id: 'c' });

	await screen.getByLabelText('Filter consumers').fill('nothing-like-this');

	await expect
		.poll(() => screen.container.textContent)
		.toContain('No consumers match “nothing-like-this”.');
	expect(screen.getByRole('listitem').query()).toBeNull();
});

test('each consumer links to the directory filtered to it', async () => {
	const screen = await render(ConsumerList, { consumers: many, id: 'c' });
	await screen.getByLabelText('Filter consumers').fill('dotfiles-work');

	await expect
		.element(screen.getByRole('link', { name: 'dotfiles-work' }))
		.toHaveAttribute('href', consumersHref(1, 'dotfiles-work'));
});

test('a short list has no filter box and no count', async () => {
	const screen = await render(ConsumerList, {
		consumers: ['homelab', 'ci'],
		id: 'c',
	});

	await expect
		.poll(() => screen.getByRole('listitem').elements().length)
		.toBe(2);
	expect(screen.getByLabelText('Filter consumers').query()).toBeNull();
	expect(screen.getByRole('status').query()).toBeNull();
});
