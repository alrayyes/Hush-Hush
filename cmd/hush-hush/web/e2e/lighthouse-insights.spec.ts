import { expect, test } from '@playwright/test';
import { INSIGHTS, missedInsights } from './lighthouse-insights';

// The four audits #721 is about. Lighthouse 13 renamed them from
// uses-long-cache-ttl and friends, so an assertion on an old ID passes
// forever by finding nothing.
test('the asserted insights are the four Lighthouse 13 names', () => {
	expect([...INSIGHTS]).toEqual([
		'cache-insight',
		'document-latency-insight',
		'render-blocking-insight',
		'network-dependency-tree-insight',
	]);
});

test('an insight that scores under 1 is reported, one that passes is not', () => {
	const audits = {
		'cache-insight': { score: 1 },
		'document-latency-insight': { score: 0.5, title: 'Document latency' },
		'render-blocking-insight': { score: 0 },
		'network-dependency-tree-insight': { score: null },
		'unused-javascript': { score: 0 },
	};

	expect(missedInsights(audits)).toEqual([
		'document-latency-insight (0.5)',
		'render-blocking-insight (0)',
	]);
});

test('an insight missing from the report is a miss, not a pass', () => {
	expect(missedInsights({})).toEqual(INSIGHTS.map((id) => `${id} (missing)`));
});
