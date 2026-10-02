import { describe, expect, it } from 'vitest';
import { tokenStatusLabel } from './tokens';

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
