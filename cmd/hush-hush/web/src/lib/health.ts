import type { Health } from './api';

export interface HealthSummary {
	healthy: boolean;
	version: string;
	environment: string | undefined;
}

// healthSummary turns /healthz into what the shared chrome shows: whether
// the server is up, its version for the footer, and the operator's own
// instance label (the optional `environment` field, alrayyes/hush-hush#512).
// A failing /healthz must not take the whole app down with it - this load
// runs at the root, so a rejection here would blank every page - so a
// rejection becomes "not healthy" with an unknown version instead.
export async function healthSummary(
	fetchHealth: () => Promise<Health>,
): Promise<HealthSummary> {
	try {
		const health = await fetchHealth();

		return {
			healthy: health.status === 'ok',
			version: health.version,
			environment: health.environment,
		};
	} catch {
		return { healthy: false, version: 'unknown', environment: undefined };
	}
}
