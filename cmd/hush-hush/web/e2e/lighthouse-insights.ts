// The Lighthouse 13 insights alrayyes/Hush-Hush#721 asserts: caching, HTML
// latency, render-blocking requests and the critical request chain. Warn-level
// like the thresholds beside them (rules/browser-compat.md), so a miss is
// printed and never fails the run.
export const INSIGHTS = [
	'cache-insight',
	'document-latency-insight',
	'render-blocking-insight',
	'network-dependency-tree-insight',
] as const;

type Audits = Record<string, { score: number | null } | undefined>;

// An insight absent from the report counts as a miss: Lighthouse renaming
// one again must not read as a pass. A null score is "not applicable".
export function missedInsights(audits: Audits): string[] {
	const missed: string[] = [];
	for (const id of INSIGHTS) {
		const audit = audits[id];
		if (!audit) missed.push(`${id} (missing)`);
		else if (audit.score !== null && audit.score < 1)
			missed.push(`${id} (${audit.score})`);
	}

	return missed;
}
