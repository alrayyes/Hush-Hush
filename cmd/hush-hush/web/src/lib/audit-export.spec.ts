import { describe, expect, it } from 'vitest';
import type { AuditLogEntry } from './api';
import { auditActorLabel, toCSV, toJSON } from './audit-export';

function entry(overrides: Partial<AuditLogEntry>): AuditLogEntry {
	return {
		id: 1,
		object_id: 'a',
		action: 'create',
		timestamp: '2026-01-01T00:00:00Z',
		ip: '203.0.113.1',
		...overrides,
	};
}

describe('auditActorLabel', () => {
	it('labels a session actor as admin', () => {
		expect(auditActorLabel(entry({ actor_type: 'session' }))).toBe('admin');
	});

	it('labels a token actor by its id', () => {
		expect(
			auditActorLabel(entry({ actor_type: 'token', actor_id: 'a1b2c3d4' })),
		).toBe('token:a1b2c3d4');
	});

	// web-ui/spec.md's "an entry with no actor... renders as 'none'"
	// requirement - unlike attribution.ts's actorLabel, this never falls
	// back to the caller, which has its own column in this table.
	it('renders none for an unauthenticated read, even with a caller present', () => {
		expect(auditActorLabel(entry({ caller: 'homelab/vps-docker' }))).toBe(
			'none',
		);
	});
});

describe('toCSV', () => {
	it('exports exactly the given entries, nothing more', () => {
		const csv = toCSV([
			entry({ id: 1, object_id: 'a', action: 'create', actor_type: 'session' }),
			entry({
				id: 2,
				object_id: 'b',
				action: 'read',
				caller: 'homelab/vps-docker',
			}),
		]);
		const lines = csv.split('\n');

		expect(lines).toHaveLength(3);
		expect(lines[0]).toBe('id,object_id,action,actor,caller,ip,timestamp');
		expect(lines[1]).toContain('"1","a","create","admin",""');
		expect(lines[2]).toContain('"2","b","read","none","homelab/vps-docker"');
	});

	it('escapes an embedded quote', () => {
		const csv = toCSV([entry({ caller: 'say "hi"' })]);

		expect(csv).toContain('"say ""hi"""');
	});

	it('produces only the header for an empty page', () => {
		expect(toCSV([])).toBe('id,object_id,action,actor,caller,ip,timestamp');
	});
});

describe('toJSON', () => {
	it('round-trips exactly the given entries', () => {
		const entries = [entry({ id: 1 }), entry({ id: 2, object_id: 'b' })];

		expect(JSON.parse(toJSON(entries))).toEqual(entries);
	});
});
