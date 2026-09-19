import { describe, expect, it } from 'vitest';
import type { AuditLogEntry } from './api';
import { actorLabel, attributionByObject } from './attribution';

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

describe('actorLabel', () => {
	it('labels a session actor as admin', () => {
		expect(
			actorLabel(entry({ actor_type: 'session', actor_id: 'admin' })),
		).toBe('admin');
	});

	it('labels a token actor by its id', () => {
		expect(
			actorLabel(entry({ actor_type: 'token', actor_id: 'a1b2c3d4' })),
		).toBe('token:a1b2c3d4');
	});

	it('falls back to the self-reported caller when there is no verified actor', () => {
		expect(actorLabel(entry({ caller: 'homelab/vps-docker' }))).toBe(
			'homelab/vps-docker',
		);
	});

	it('falls back to unknown when there is neither an actor nor a caller', () => {
		expect(actorLabel(entry({}))).toBe('unknown');
	});
});

describe('attributionByObject', () => {
	it('attributes a create entry as both created and updated', () => {
		const result = attributionByObject([
			entry({
				action: 'create',
				actor_type: 'session',
				timestamp: '2026-01-01T00:00:00Z',
			}),
		]);

		expect(result.get('a')).toEqual({
			createdBy: 'admin',
			createdAt: '2026-01-01T00:00:00Z',
			updatedBy: 'admin',
			updatedAt: '2026-01-01T00:00:00Z',
		});
	});

	it('overwrites updated-by/at on a later update, leaving created-by/at alone', () => {
		const result = attributionByObject([
			entry({
				action: 'create',
				actor_type: 'session',
				timestamp: '2026-01-01T00:00:00Z',
			}),
			entry({
				action: 'update',
				actor_type: 'token',
				actor_id: 'a1b2c3d4',
				timestamp: '2026-01-02T00:00:00Z',
			}),
		]);

		expect(result.get('a')).toEqual({
			createdBy: 'admin',
			createdAt: '2026-01-01T00:00:00Z',
			updatedBy: 'token:a1b2c3d4',
			updatedAt: '2026-01-02T00:00:00Z',
		});
	});

	it('ignores read and delete entries', () => {
		const result = attributionByObject([
			entry({ action: 'create', timestamp: '2026-01-01T00:00:00Z' }),
			entry({ action: 'read', timestamp: '2026-01-02T00:00:00Z' }),
			entry({ action: 'delete', timestamp: '2026-01-03T00:00:00Z' }),
		]);

		expect(result.get('a')?.updatedAt).toBe('2026-01-01T00:00:00Z');
	});

	it('keeps each object id separate', () => {
		const result = attributionByObject([
			entry({ object_id: 'a', action: 'create' }),
			entry({ object_id: 'b', action: 'create' }),
		]);

		expect(result.size).toBe(2);
	});
});
