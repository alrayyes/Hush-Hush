import { describe, expect, it } from 'vitest';
import { formatTimestamp, isTokenDead } from './datetime';

describe('formatTimestamp', () => {
	it('formats an ISO instant as a medium date + short time', () => {
		expect(
			formatTimestamp('2026-09-20T14:53:00Z', {
				locale: 'en-US',
				timeZone: 'UTC',
			}),
		).toBe('Sep 20, 2026, 2:53 PM');
	});

	it('renders a different instant distinctly, not a fixed placeholder', () => {
		expect(
			formatTimestamp('2026-01-01T00:00:00Z', {
				locale: 'en-US',
				timeZone: 'UTC',
			}),
		).toBe('Jan 1, 2026, 12:00 AM');
	});
});

describe('isTokenDead', () => {
	const future = new Date(Date.now() + 1_000_000).toISOString();
	const past = new Date(Date.now() - 1_000_000).toISOString();

	it('is alive when not revoked and not yet expired', () => {
		expect(isTokenDead({ revoked: false, expires_at: future })).toBe(false);
	});

	it('is dead when expired, even if never revoked', () => {
		expect(isTokenDead({ revoked: false, expires_at: past })).toBe(true);
	});

	it('is dead when revoked, even if not yet expired', () => {
		expect(isTokenDead({ revoked: true, expires_at: future })).toBe(true);
	});

	it('is dead when both revoked and expired', () => {
		expect(isTokenDead({ revoked: true, expires_at: past })).toBe(true);
	});
});
