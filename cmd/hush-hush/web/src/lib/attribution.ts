import type { Actor, AuditLogEntry } from './api';

// How the UI names who did something. The API reports an actor either as a
// {type, id} pair on an object (created_by / updated_by, alrayyes/hush-hush
// #535) or as the fields of an audit log entry; both use the same type
// values, so the names live in one place.

export function actorLabel(entry: AuditLogEntry): string {
	if (entry.actor_type === 'session') {
		return 'admin';
	}

	if (entry.actor_type === 'token') {
		return `token:${entry.actor_id}`;
	}

	return entry.caller || 'unknown';
}

// actorName names an object's creator or last updater. A session is the
// admin account; a write token and a consumer token show their id, kept
// apart because they are different credentials.
export function actorName(actor: Actor): string {
	if (actor.type === 'session') {
		return 'admin';
	}

	if (actor.type === 'token') {
		return `token:${actor.id}`;
	}

	return `consumer-token:${actor.id}`;
}
