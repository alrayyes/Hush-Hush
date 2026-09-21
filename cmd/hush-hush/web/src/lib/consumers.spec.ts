import { describe, expect, it } from 'vitest';
import {
	consumersHref,
	parseConsumersQuery,
	secretsOverviewHref,
	totalPages,
} from './consumers';

describe('parseConsumersQuery', () => {
	it('defaults to page 1 and an empty filter with no parameters', () => {
		expect(
			parseConsumersQuery(new URL('https://example.com/consumers')),
		).toEqual({ page: 1, q: '' });
	});

	it('reads page and q from the URL', () => {
		expect(
			parseConsumersQuery(
				new URL('https://example.com/consumers?page=3&q=homelab'),
			),
		).toEqual({ page: 3, q: 'homelab' });
	});

	it('falls back to page 1 for a non-positive or non-numeric page', () => {
		expect(
			parseConsumersQuery(new URL('https://example.com/consumers?page=0')).page,
		).toBe(1);
		expect(
			parseConsumersQuery(new URL('https://example.com/consumers?page=nope'))
				.page,
		).toBe(1);
	});
});

describe('consumersHref', () => {
	it('omits both parameters for page 1 with no filter', () => {
		expect(consumersHref(1, '')).toBe('/consumers');
	});

	it('includes q but omits page 1', () => {
		expect(consumersHref(1, 'homelab')).toBe('/consumers?q=homelab');
	});

	it('includes page but omits an empty filter', () => {
		expect(consumersHref(2, '')).toBe('/consumers?page=2');
	});

	it('includes both when neither is the default', () => {
		expect(consumersHref(2, 'homelab')).toBe('/consumers?q=homelab&page=2');
	});
});

describe('totalPages', () => {
	it('is at least 1 even with no results', () => {
		expect(totalPages(0, 20)).toBe(1);
	});

	it('rounds up a partial last page', () => {
		expect(totalPages(21, 20)).toBe(2);
	});

	it('is exact for a total that divides evenly', () => {
		expect(totalPages(40, 20)).toBe(2);
	});
});

describe('secretsOverviewHref', () => {
	it('encodes the consumer name into the used_by query parameter', () => {
		expect(secretsOverviewHref('homelab/vps-docker')).toBe(
			'/?used_by=homelab%2Fvps-docker',
		);
	});
});
