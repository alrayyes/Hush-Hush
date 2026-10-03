// alrayyes/hush-hush#577: the list pages render twice, a card list under md
// and a table from md up. Nothing tied the two together, so they drifted: a
// desktop visitor had no way into a secret's detail page. These checks read a
// row at a phone width and at a desktop width and fail when an action or a
// fact is in one layout and not the other.
//
// A difference that is meant has to be named in the spec, with why. It is
// then visible in review, not found later by a user.
import { expect, type Page } from '@playwright/test';

export const PHONE = { width: 390, height: 844 };
export const DESKTOP = { width: 1280, height: 800 };

export interface ListParity {
	/** The page the list lives on. */
	path: string;
	/** aria-label of the card list; the table is the one beside it. */
	list: string;
	/** Names of the rows (slugs, consumers) stripped from action names, so
	 * "Copy slug foo" and "Copy slug" compare as the same action. */
	ids: string[];
	/** Spellings that are the same action: { '1 token': '{tokens}' }. */
	same?: Record<string, string>;
	/** Actions that are in one layout only on purpose, with the reason. */
	only?: { cards?: Record<string, string>; table?: Record<string, string> };
	/** Facts a row shows in both layouts. `times` is how often the pattern
	 * must occur, for a value that appears for two fields (created by,
	 * updated by). */
	facts: Record<string, { pattern: RegExp; times?: number }>;
}

interface RowReading {
	layout: 'cards' | 'table' | 'none';
	actions: string[];
	text: string;
}

async function readRow(page: Page, list: string): Promise<RowReading> {
	return page.evaluate((listLabel) => {
		const visible = (el: Element | null) =>
			!!el && (el as HTMLElement).offsetParent !== null;
		const ul = document.querySelector(
			`ul[aria-label="${listLabel}"]`,
		) as HTMLElement | null;
		const table = ul?.parentElement?.querySelector('table.responsive-table');
		const row = visible(ul)
			? ul?.querySelector('li')
			: visible(table ?? null)
				? table?.querySelector('tbody tr')
				: null;

		if (!row) return { layout: 'none', actions: [], text: '' };

		return {
			layout: visible(ul) ? 'cards' : 'table',
			actions: [...row.querySelectorAll('button, a')]
				.filter(visible)
				.map((el) =>
					(el.getAttribute('aria-label') || el.textContent || '')
						.replace(/\s+/g, ' ')
						.trim(),
				),
			text: (row as HTMLElement).innerText.replace(/\s+/g, ' '),
		};
	}, list);
}

function normalise(name: string, spec: ListParity): string {
	let out = name;

	for (const id of spec.ids) out = out.split(id).join('{row}');
	out = out.replace(/\s+/g, ' ').trim();

	return spec.same?.[out] ?? out;
}

async function readAt(
	page: Page,
	spec: ListParity,
	size: { width: number; height: number },
) {
	await page.setViewportSize(size);
	await page.goto(spec.path);
	await expect
		.poll(async () => (await readRow(page, spec.list)).layout, {
			message: `${spec.list} has a visible row at ${size.width}px`,
		})
		.not.toBe('none');
	const reading = await readRow(page, spec.list);

	return {
		reading,
		actions: reading.actions.map((name) => normalise(name, spec)),
	};
}

// expectListParity checks one list on its own page in the same signed-in
// context, so the journey's own page keeps its state.
export async function expectListParity(page: Page, spec: ListParity) {
	const checker = await page.context().newPage();

	try {
		const cards = await readAt(checker, spec, PHONE);
		const table = await readAt(checker, spec, DESKTOP);

		expect(cards.reading.layout).toBe('cards');
		expect(table.reading.layout).toBe('table');

		// alrayyes/hush-hush#578: the table's cells keep their own padding, so
		// neighbouring values don't read as one string, and the table fits the
		// page without scrolling sideways.
		const metrics = await checker.evaluate((listLabel) => {
			const ul = document.querySelector(`ul[aria-label="${listLabel}"]`);
			const cell = ul?.parentElement?.querySelector(
				'table.responsive-table tbody td',
			);

			return {
				padding: cell
					? Number.parseFloat(getComputedStyle(cell).paddingLeft)
					: 0,
				scrollWidth: document.documentElement.scrollWidth,
				clientWidth: document.documentElement.clientWidth,
			};
		}, spec.list);

		expect(
			metrics.padding,
			`${spec.list}: table cells have horizontal padding`,
		).toBeGreaterThan(0);
		expect(
			metrics.scrollWidth,
			`${spec.list}: the table fits a ${DESKTOP.width}px page`,
		).toBeLessThanOrEqual(metrics.clientWidth);

		const without = (actions: string[], skip: Record<string, string> = {}) =>
			actions.filter((name) => !(name in skip)).sort();

		expect(
			without(cards.actions, spec.only?.cards),
			`${spec.list}: card actions match the table's`,
		).toEqual(without(table.actions, spec.only?.table));

		for (const [label, { pattern, times = 1 }] of Object.entries(spec.facts)) {
			const global = new RegExp(pattern.source, `${pattern.flags}g`);

			for (const [layout, text] of [
				['cards', cards.reading.text],
				['table', table.reading.text],
			] as const) {
				expect(
					(text.match(global) ?? []).length,
					`${spec.list}: the ${layout} show ${label} (row: ${text})`,
				).toBeGreaterThanOrEqual(times);
			}
		}
	} finally {
		await checker.close();
	}
}

// expectNavParity checks that the top nav and the bottom nav lead to the same
// places, in the same order.
export async function expectNavParity(page: Page) {
	const checker = await page.context().newPage();

	try {
		await checker.setViewportSize(DESKTOP);
		await checker.goto('/');
		const top = checker.getByRole('navigation', {
			name: 'Primary',
			exact: true,
		});
		await expect(top).toBeVisible();
		const topLinks = await top.getByRole('link').allInnerTexts();

		await checker.setViewportSize(PHONE);
		const bottom = checker.getByRole('navigation', {
			name: 'Primary (mobile)',
		});
		await expect(bottom).toBeVisible();
		await expect(top).toBeHidden();
		const bottomLinks = await bottom.getByRole('link').allInnerTexts();

		expect(topLinks.length).toBeGreaterThan(0);
		expect(bottomLinks.map((name) => name.trim())).toEqual(
			topLinks.map((name) => name.trim()),
		);
	} finally {
		await checker.close();
	}
}
