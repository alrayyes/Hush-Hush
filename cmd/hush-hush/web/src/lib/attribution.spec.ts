import { describe, expect, it } from 'vitest';
import type { AuditLogEntry } from './api';
import { actorLabel, actorName } from './attribution';

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

describe('actorName', () => {
	it('labels a session actor as admin', () => {
		expect(actorName({ type: 'session', id: 'abc' })).toBe('admin');
	});

	it('labels a write token by its id', () => {
		expect(actorName({ type: 'token', id: 'tok-1' })).toBe('token:tok-1');
	});

	it('labels a consumer token by its id, apart from a write token', () => {
		expect(actorName({ type: 'consumer_token', id: 'ct-9' })).toBe(
			'consumer-token:ct-9',
		);
	});
});

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
