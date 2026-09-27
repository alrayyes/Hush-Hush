import { describe, expect, it } from 'vitest';
import { formatTimestamp } from './datetime';

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
