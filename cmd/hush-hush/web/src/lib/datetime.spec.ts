import { describe, expect, it } from 'vitest';
import { formatRemaining, formatTimestamp, isTokenDead } from './datetime';

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

describe('formatRemaining', () => {
	const now = new Date('2026-10-02T12:00:00Z');
	const inMs = (ms: number) => new Date(now.getTime() + ms).toISOString();
	const minute = 60_000;
	const hour = 60 * minute;
	const day = 24 * hour;

	it('counts whole days once a day or more is left', () => {
		expect(formatRemaining(inMs(3 * day + 5 * hour), now)).toBe('3d left');
		expect(formatRemaining(inMs(day), now)).toBe('1d left');
	});

	it('counts whole hours under a day', () => {
		expect(formatRemaining(inMs(23 * hour + 59 * minute), now)).toBe(
			'23h left',
		);
		expect(formatRemaining(inMs(hour), now)).toBe('1h left');
	});

	it('counts whole minutes under an hour', () => {
		expect(formatRemaining(inMs(59 * minute), now)).toBe('59m left');
		expect(formatRemaining(inMs(minute), now)).toBe('1m left');
	});

	it('says under a minute rather than rounding to zero', () => {
		expect(formatRemaining(inMs(30_000), now)).toBe('<1m left');
	});

	it('says expired at and after the expiry instant', () => {
		expect(formatRemaining(inMs(0), now)).toBe('expired');
		expect(formatRemaining(inMs(-day), now)).toBe('expired');
	});
});
