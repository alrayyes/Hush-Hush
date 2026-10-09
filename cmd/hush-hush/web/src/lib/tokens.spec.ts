import { describe, expect, it } from 'vitest';
import {
	TOKEN_TTL_SECONDS_DEFAULT,
	TOKEN_TTL_SECONDS_MAX,
	TOKEN_TTL_SECONDS_MIN,
} from './api-limits';
import {
	findActiveDuplicate,
	TTL_DAYS,
	tokenRemaining,
	tokenStatusLabel,
	ttlDaysToSeconds,
} from './tokens';

describe('tokenStatusLabel', () => {
	it('capitalises the server-reported status for display', () => {
		expect(tokenStatusLabel('active')).toBe('Active');
		expect(tokenStatusLabel('expired')).toBe('Expired');
		expect(tokenStatusLabel('revoked')).toBe('Revoked');
	});

	it('is empty when the server sent no status, rather than guessing one', () => {
		expect(tokenStatusLabel(undefined)).toBe('');
	});
});

describe('TTL_DAYS', () => {
	it("is the spec's default and limits, in the whole days the dialogs ask for", () => {
		expect(TTL_DAYS.default * 86_400).toBe(TOKEN_TTL_SECONDS_DEFAULT);
		expect(TTL_DAYS.max * 86_400).toBeLessThanOrEqual(TOKEN_TTL_SECONDS_MAX);
		expect((TTL_DAYS.max + 1) * 86_400).toBeGreaterThan(TOKEN_TTL_SECONDS_MAX);
		expect(TTL_DAYS.min * 86_400).toBeGreaterThanOrEqual(TOKEN_TTL_SECONDS_MIN);
		expect((TTL_DAYS.min - 1) * 86_400).toBeLessThan(TOKEN_TTL_SECONDS_MIN);
	});
});

describe('ttlDaysToSeconds', () => {
	it('converts whole days to the seconds the API takes', () => {
		expect(ttlDaysToSeconds(1)).toBe(86_400);
		expect(ttlDaysToSeconds(TTL_DAYS.default)).toBe(TOKEN_TTL_SECONDS_DEFAULT);
	});
});

describe('tokenRemaining', () => {
	const now = new Date('2026-10-03T12:00:00Z');
	const in3Days = '2026-10-06T12:00:00Z';

	it('shows the time left on an active token', () => {
		expect(tokenRemaining({ status: 'active', expires_at: in3Days }, now)).toBe(
			'3d left',
		);
	});

	it('names a dead token by its status, whatever the clock says', () => {
		expect(
			tokenRemaining({ status: 'revoked', expires_at: in3Days }, now),
		).toBe('revoked');
		expect(
			tokenRemaining({ status: 'expired', expires_at: in3Days }, now),
		).toBe('expired');
	});

	it('falls back to the status when the browser clock says the time is up', () => {
		expect(
			tokenRemaining(
				{ status: 'active', expires_at: '2026-01-01T00:00:00Z' },
				now,
			),
		).toBe('active');
	});

	it('is empty when the server sent no status and the clock gives no time left', () => {
		expect(tokenRemaining({ expires_at: '2026-01-01T00:00:00Z' }, now)).toBe(
			'',
		);
	});
});

describe('findActiveDuplicate', () => {
	const token = (over: Record<string, unknown> = {}) => ({
		id: 'aaaaaaaaaaaaaaaa',
		consumer: 'release-job',
		description: 'release job consumer read token',
		created_at: '2026-10-06T10:00:00Z',
		status: 'active' as const,
		...over,
	});

	it('finds an active token for the same consumer and description', () => {
		const tokens = [token()];

		expect(
			findActiveDuplicate(
				tokens,
				'release-job',
				'release job consumer read token',
			),
		).toBe(tokens[0]);
	});

	it('ignores spaces around the description', () => {
		expect(
			findActiveDuplicate(
				[token()],
				'release-job',
				'  release job consumer read token ',
			),
		).toBeDefined();
	});

	it('ignores a revoked or expired token', () => {
		for (const status of ['revoked', 'expired'] as const) {
			expect(
				findActiveDuplicate(
					[token({ status })],
					'release-job',
					'release job consumer read token',
				),
			).toBeUndefined();
		}
	});

	it('ignores another consumer or another description', () => {
		expect(
			findActiveDuplicate(
				[token()],
				'ci-runner',
				'release job consumer read token',
			),
		).toBeUndefined();
		expect(
			findActiveDuplicate([token()], 'release-job', 'other'),
		).toBeUndefined();
	});

	it('returns the newest when several match', () => {
		const older = token({ id: 'o', created_at: '2026-10-05T10:00:00Z' });
		const newer = token({ id: 'n', created_at: '2026-10-06T10:00:00Z' });

		expect(
			findActiveDuplicate(
				[older, newer],
				'release-job',
				'release job consumer read token',
			)?.id,
		).toBe('n');
	});

	it('finds nothing without a consumer or description', () => {
		expect(findActiveDuplicate([token()], '', 'x')).toBeUndefined();
		expect(findActiveDuplicate([token()], 'release-job', '  ')).toBeUndefined();
	});
});
