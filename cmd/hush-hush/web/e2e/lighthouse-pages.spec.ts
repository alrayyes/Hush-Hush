import { readdirSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';
import { expect, test } from '@playwright/test';
import { AUDITED_PAGES } from './lighthouse-pages';

// Every page of the app is audited by Lighthouse. A page added to the router
// and left out of AUDITED_PAGES would be published as untested, which is how
// only /login and an empty overview were ever measured.
function routesOnDisk(dir: string, root = dir): string[] {
	const found: string[] = [];
	for (const name of readdirSync(dir)) {
		const full = join(dir, name);
		if (statSync(full).isDirectory()) {
			found.push(...routesOnDisk(full, root));
		} else if (name === '+page.svelte') {
			// Route groups like (app) are not part of the URL.
			const route = `/${relative(root, dir)}`
				.split('/')
				.filter((segment) => !/^\(.*\)$/.test(segment))
				.join('/');
			found.push(route === '' ? '/' : route);
		}
	}

	return found;
}

test('every route of the app has a Lighthouse audit, and no audit has no route', () => {
	const onDisk = routesOnDisk('src/routes').sort();
	const audited = AUDITED_PAGES.map((page) => page.route).sort();

	expect(audited).toEqual(onDisk);
});

test('report names are unique and become <name>.report.html', () => {
	const names = AUDITED_PAGES.map((page) => page.report);

	expect(new Set(names).size).toBe(names.length);
	for (const name of names) {
		expect(name).toMatch(/^[a-z0-9-]+$/);
	}
});

test('a page that needs a session comes after the page that signs in', () => {
	const order = AUDITED_PAGES.map((page) => page.session);
	const firstSession = order.indexOf(true);

	expect(firstSession).toBeGreaterThan(0);
	expect(order.slice(firstSession).every(Boolean)).toBe(true);
});
