import { describe, expect, it } from 'vitest';
import {
	CONSUMER_LIST_FILTER_MIN,
	consumersHref,
	filterConsumers,
	parseConsumersQuery,
	secretsOverviewHref,
	summariseConsumers,
	tokensHref,
	totalPages,
	truncateKey,
	variantLabel,
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

describe('tokensHref', () => {
	it('encodes the consumer name into the settings consumer query parameter', () => {
		expect(tokensHref('homelab/vps-docker')).toBe(
			'/settings?consumer=homelab%2Fvps-docker',
		);
	});
});

describe('truncateKey', () => {
	const key = 'age1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqsr5xk3l';

	it('keeps the first 10 and last 6 characters around an ellipsis', () => {
		expect(truncateKey(key)).toBe(`${key.slice(0, 10)}…${key.slice(-6)}`);
	});

	it('leaves a key of 16 characters or fewer unchanged', () => {
		expect(truncateKey('age1short')).toBe('age1short');
		expect(truncateKey('0123456789abcdef')).toBe('0123456789abcdef');
	});

	it('truncates a 17 character key', () => {
		expect(truncateKey('0123456789abcdefg')).toBe('0123456789…bcdefg');
	});
});

describe('summariseConsumers', () => {
	it('shows every consumer when there are two or fewer', () => {
		expect(summariseConsumers([])).toEqual({ shown: [], more: 0 });
		expect(summariseConsumers(['a', 'b'])).toEqual({
			shown: ['a', 'b'],
			more: 0,
		});
	});

	it('shows the first two and counts the rest, however many there are', () => {
		const hundred = Array.from({ length: 100 }, (_, i) => `c${i}`);

		expect(summariseConsumers(hundred)).toEqual({
			shown: ['c0', 'c1'],
			more: 98,
		});
	});

	it('treats a missing list as none', () => {
		expect(summariseConsumers(undefined)).toEqual({ shown: [], more: 0 });
	});
});

describe('filterConsumers', () => {
	const all = [
		'alrayyes/dotfiles',
		'alrayyes/resume',
		'alrayyes/server-dotfiles',
	];

	it('keeps everything for an empty or blank filter', () => {
		expect(filterConsumers(all, '')).toEqual(all);
		expect(filterConsumers(all, '  ')).toEqual(all);
	});

	it('matches a substring, ignoring case and surrounding space', () => {
		expect(filterConsumers(all, ' DOT ')).toEqual([
			'alrayyes/dotfiles',
			'alrayyes/server-dotfiles',
		]);
	});

	it('returns nothing when nothing matches', () => {
		expect(filterConsumers(all, 'zzz')).toEqual([]);
	});
});

describe('CONSUMER_LIST_FILTER_MIN', () => {
	it('is the length above which a list gets a filter box', () => {
		expect(CONSUMER_LIST_FILTER_MIN).toBe(10);
	});
});

describe('variantLabel', () => {
	it('says nothing for a name with one variant', () => {
		expect(variantLabel(['a', 'b'], 1)).toBe('');
	});

	it('names the consumers of one of several variants', () => {
		expect(variantLabel(['consumer_d'], 2)).toBe('Variant for consumer_d');
		expect(variantLabel([], 2)).toBe('Variant with no consumers');
	});

	it('names two and counts the rest, however many there are', () => {
		const hundred = Array.from({ length: 100 }, (_, i) => `c${i}`);
		const label = variantLabel(hundred, 2);

		expect(label).toBe('Variant for c0, c1 and 98 more');
		expect(label).not.toContain('c50');
	});
});
