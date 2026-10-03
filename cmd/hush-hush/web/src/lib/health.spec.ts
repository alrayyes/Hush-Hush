import { describe, expect, it } from 'vitest';
import type { Health } from './api';
import { healthSummary } from './health';

describe('healthSummary', () => {
	it('is healthy and carries the version and instance label when /healthz answers', async () => {
		const fetchHealth = async (): Promise<Health> => ({
			status: 'ok',
			version: '2.47.0',
			environment: 'prod / homelab',
		});

		expect(await healthSummary(fetchHealth)).toEqual({
			healthy: true,
			version: '2.47.0',
			environment: 'prod / homelab',
		});
	});

	it('leaves the label undefined when the server has none', async () => {
		const fetchHealth = async (): Promise<Health> => ({
			status: 'ok',
			version: '2.47.0',
		});

		const summary = await healthSummary(fetchHealth);

		expect(summary.healthy).toBe(true);
		expect(summary.environment).toBeUndefined();
	});

	it('is not healthy when /healthz rejects, instead of throwing', async () => {
		const fetchHealth = async (): Promise<Health> => {
			throw new Error('network down');
		};

		expect(await healthSummary(fetchHealth)).toEqual({
			healthy: false,
			version: 'unknown',
			environment: undefined,
		});
	});

	it('is not healthy when the server answers with a status other than ok', async () => {
		// The spec only allows "ok"; this is an answer from outside it, such
		// as a proxy's error page, which the client must still not trust.
		const fetchHealth = async (): Promise<Health> => ({
			status: 'degraded' as Health['status'],
			version: '2.47.0',
			environment: 'prod / homelab',
		});

		const summary = await healthSummary(fetchHealth);

		expect(summary.healthy).toBe(false);
		expect(summary.version).toBe('2.47.0');
	});
});
