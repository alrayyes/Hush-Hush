import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import CursorPager from './CursorPager.svelte';
import Pager from './Pager.svelte';

// alrayyes/Hush-Hush#892: what the shared pager puts on the page. Rendered on
// the server side of Svelte, so no browser is needed to read the markup. The
// first page wired to it (#894) adds the real click and the axe scan.

const href = (page: number) => `/list?page=${page}`;

// The markup of the one link or button whose text is `text`.
function control(html: string, text: string): string {
	const chunk = html
		.split(/(?=<(?:a|button)\b)/)
		.find((part) => part.includes('>') && part.includes(text));

	expect(chunk, `a control reading "${text}"`).toBeDefined();

	return chunk as string;
}

function numbered(props: {
	page: number;
	total: number;
	pageSize?: number;
	loading?: boolean;
}) {
	return render(Pager, {
		props: { pageSize: 20, href, label: 'Secrets pages', ...props },
	}).body;
}

// Whether the button's own opening tag carries the `disabled` attribute. The
// class list also has `disabled:` variants, so a substring check would lie.
function isDisabled(markup: string): boolean {
	const tag = markup.slice(0, markup.indexOf('>'));

	return /\sdisabled(?:=|\s|$)/.test(tag);
}

describe('Pager', () => {
	it('renders nothing when the list fits on one page', () => {
		const html = numbered({ page: 1, total: 20 });

		expect(html).not.toContain('<nav');
		expect(html).not.toContain('Showing');
	});

	it('is a labelled navigation landmark', () => {
		expect(numbered({ page: 2, total: 137 })).toContain(
			'<nav aria-label="Secrets pages"',
		);
	});

	it('says which rows are showing', () => {
		expect(numbered({ page: 2, total: 137 })).toContain('21–40');
	});

	it('marks the current page and no other', () => {
		const html = numbered({ page: 2, total: 137 });

		expect(html.match(/aria-current="page"/g)).toHaveLength(1);
	});

	it('links Previous and Next to the neighbouring pages', () => {
		const html = numbered({ page: 2, total: 137 });

		expect(html).toContain('href="/list?page=1"');
		expect(html).toContain('href="/list?page=3"');
	});

	it('disables Previous on the first page, in place', () => {
		const html = numbered({ page: 1, total: 137 });

		expect(control(html, 'Previous')).toContain('aria-disabled="true"');
		expect(control(html, 'Previous')).not.toContain('href=');
	});

	it('disables Next on the last page, in place', () => {
		const html = numbered({ page: 7, total: 137 });

		expect(control(html, 'Next')).toContain('aria-disabled="true"');
		expect(control(html, 'Next')).not.toContain('href=');
	});

	it('dims a disabled link, since the button styles only dim a real :disabled', () => {
		const html = numbered({ page: 1, total: 137 });

		expect(control(html, 'Previous')).toContain('aria-disabled:opacity-50');
		expect(control(html, 'Previous')).toContain(
			'aria-disabled:pointer-events-none',
		);
	});

	it('is marked busy while a page is loading', () => {
		expect(numbered({ page: 2, total: 137, loading: true })).toContain(
			'aria-busy="true"',
		);
	});

	it('disables every control while a page is loading', () => {
		const html = numbered({ page: 2, total: 137, loading: true });

		expect(html).not.toContain('href="/list?page=');
	});

	it('shows the last page, not an impossible range, when the page is past the end', () => {
		const html = numbered({ page: 20, total: 137 });

		expect(html).toContain('121–137');
		expect(html).toContain('Page 7 of 7');
	});

	it('gives Previous and Next a 44px target below the sm breakpoint', () => {
		const html = numbered({ page: 2, total: 137 });

		expect(control(html, 'Previous')).toContain('h-11');
		expect(control(html, 'Next')).toContain('h-11');
	});

	it('keeps the scroll position when asked to, for a list low on a long page', () => {
		const html = render(Pager, {
			props: {
				page: 2,
				total: 137,
				pageSize: 20,
				href,
				keepScroll: true,
			},
		}).body;

		expect(control(html, 'Next')).toContain('data-sveltekit-reset="false"');
		expect(control(html, 'Page 3')).toContain('data-sveltekit-reset="false"');
	});

	it('lets the page scroll to the top by default', () => {
		expect(control(numbered({ page: 2, total: 137 }), 'Next')).not.toContain(
			'data-sveltekit-reset',
		);
	});

	it('carries the compact mobile status text', () => {
		expect(numbered({ page: 2, total: 137 })).toContain('Page 2 of 7');
	});
});

describe('CursorPager', () => {
	const cursor = (props: {
		canNewer: boolean;
		canOlder: boolean;
		count?: number;
		loading?: boolean;
	}) =>
		render(CursorPager, {
			props: {
				count: 25,
				label: 'Audit log pages',
				onNewer: () => {},
				onOlder: () => {},
				...props,
			},
		}).body;

	it('renders nothing when there is no other page', () => {
		expect(cursor({ canNewer: false, canOlder: false })).not.toContain('<nav');
	});

	it('says how many entries are showing, with no total', () => {
		const html = cursor({ canNewer: false, canOlder: true });

		expect(html).toContain('Showing');
		expect(html).toContain('25');
		expect(html).toContain('entries');
		expect(html).not.toContain(' of ');
	});

	it('disables Newer on the first page and keeps Older live', () => {
		const html = cursor({ canNewer: false, canOlder: true });

		expect(isDisabled(control(html, 'Newer'))).toBe(true);
		expect(isDisabled(control(html, 'Older'))).toBe(false);
	});

	it('disables Older on the last page', () => {
		const html = cursor({ canNewer: true, canOlder: false });

		expect(isDisabled(control(html, 'Older'))).toBe(true);
		expect(isDisabled(control(html, 'Newer'))).toBe(false);
	});

	it('gives Newer and Older a 44px target below the sm breakpoint', () => {
		const html = cursor({ canNewer: true, canOlder: true });

		expect(control(html, 'Newer')).toContain('h-11');
		expect(control(html, 'Older')).toContain('h-11');
	});

	it('is marked busy while a page is loading', () => {
		expect(cursor({ canNewer: true, canOlder: true, loading: true })).toContain(
			'aria-busy="true"',
		);
	});

	it('says "1 entry" for one', () => {
		expect(cursor({ canNewer: true, canOlder: false, count: 1 })).toContain(
			'entry',
		);
	});
});
