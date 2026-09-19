import type { AuditLogEntry } from './api';

// ObjectMetadata (GET /objects' response shape) carries no created/updated
// attribution of its own - that lives entirely in the audit log
// (audit-log/spec.md's "Verified actor attribution" requirement), so the
// secrets overview derives it from a create/update entry's actor rather
// than a field on the object itself.

export interface Attribution {
	createdBy: string;
	createdAt: string;
	updatedBy: string;
	updatedAt: string;
}

export function actorLabel(entry: AuditLogEntry): string {
	if (entry.actor_type === 'session') {
		return 'admin';
	}

	if (entry.actor_type === 'token') {
		return `token:${entry.actor_id}`;
	}

	return entry.caller || 'unknown';
}

// attributionByObject expects entries oldest-first (QueryAuditLog's own
// order), so the first create and the last update naturally win by
// overwriting - "updated" defaults to the create entry until a real
// update happens.
export function attributionByObject(
	entries: AuditLogEntry[],
): Map<string, Attribution> {
	const result = new Map<string, Attribution>();

	for (const entry of entries) {
		if (entry.action === 'create') {
			const label = actorLabel(entry);
			result.set(entry.object_id, {
				createdBy: label,
				createdAt: entry.timestamp,
				updatedBy: label,
				updatedAt: entry.timestamp,
			});
		} else if (entry.action === 'update') {
			const existing = result.get(entry.object_id);
			if (existing) {
				result.set(entry.object_id, {
					...existing,
					updatedBy: actorLabel(entry),
					updatedAt: entry.timestamp,
				});
			}
		}
	}

	return result;
}
