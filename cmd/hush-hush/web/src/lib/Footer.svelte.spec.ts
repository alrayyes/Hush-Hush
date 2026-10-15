import { expect, test } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Footer from './Footer.svelte';

// alrayyes/hush-hush#718: the footer links to the repository, and says it
// leaves the app.
const sourceLink = (screen: Awaited<ReturnType<typeof render>>) =>
	screen.getByRole('link', { name: /^GitHub/ });

test('the source link goes to the repository', async () => {
	const screen = await render(Footer, { version: 'v1.2.3' });

	await expect
		.element(sourceLink(screen))
		.toHaveAttribute('href', 'https://github.com/alrayyes/Hush-Hush');
});

test('the source link opens without handing the new page the app', async () => {
	const screen = await render(Footer, { version: 'v1.2.3' });

	await expect
		.element(sourceLink(screen))
		.toHaveAttribute('rel', 'noopener noreferrer');
});

test('the source link says it leaves the app', async () => {
	const screen = await render(Footer, { version: 'v1.2.3' });

	await expect
		.element(sourceLink(screen))
		.toHaveAccessibleName(/external site/i);
});

test('the mark leads the label and is hidden from assistive tech', async () => {
	const screen = await render(Footer, { version: 'v1.2.3' });

	const link = sourceLink(screen);
	await expect.element(link).toBeVisible();
	// First thing in the link that isn't whitespace: text nodes count, so a
	// label written before the mark is caught.
	const first = [...link.element().childNodes].find(
		(node) => node.nodeType !== Node.TEXT_NODE || node.textContent?.trim(),
	);
	expect((first as Element).tagName.toLowerCase()).toBe('svg');
	expect((first as Element).getAttribute('aria-hidden')).toBe('true');
});

test('the version links to the changelog', async () => {
	const screen = await render(Footer, { version: 'v1.2.3' });

	await expect
		.element(screen.getByRole('link', { name: 'v1.2.3' }))
		.toHaveAttribute('href', '/changelog');
});
