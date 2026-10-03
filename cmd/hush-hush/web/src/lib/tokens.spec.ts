import { describe, expect, it } from 'vitest';
import {
	TOKEN_TTL_SECONDS_DEFAULT,
	TOKEN_TTL_SECONDS_MAX,
	TOKEN_TTL_SECONDS_MIN,
} from './api-limits';
import { TTL_DAYS, tokenStatusLabel, ttlDaysToSeconds } from './tokens';

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
