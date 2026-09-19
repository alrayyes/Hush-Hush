import type { AuditLogEntry } from './api';

// auditActorLabel is deliberately not attribution.ts's actorLabel: that
// one falls back to the self-reported caller when there's no verified
// actor, which is right for the secrets overview's single "created/
// updated by" column. The audit log table shows actor and caller as two
// separate columns, so an entry with no verified actor (an
// unauthenticated read) renders as plain "none" here - web-ui/spec.md's
// "an entry with no actor... renders as 'none'" requirement, not a
// caller-shaped guess.
export function auditActorLabel(entry: AuditLogEntry): string {
	if (entry.actor_type === 'session') return 'admin';
	if (entry.actor_type === 'token') return `token:${entry.actor_id}`;

	return 'none';
}

// toCSV/toJSON are pure functions specifically so audit-log/+page.svelte's
// "export exactly what's currently shown" requirement (web-ui/spec.md's
// "Exporting the visible page" scenario) is checkable without a browser -
// the page itself only has to turn this string into a download.

export function toCSV(entries: AuditLogEntry[]): string {
	const header = 'id,object_id,action,actor,caller,ip,timestamp';
	const rows = entries.map((e) =>
		[
			e.id,
			e.object_id,
			e.action,
			auditActorLabel(e),
			e.caller ?? '',
			e.ip,
			e.timestamp,
		]
			.map((v) => `"${String(v).replaceAll('"', '""')}"`)
			.join(','),
	);

	return [header, ...rows].join('\n');
}

export function toJSON(entries: AuditLogEntry[]): string {
	return JSON.stringify(entries, null, 2);
}
