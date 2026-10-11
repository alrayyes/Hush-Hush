import { describe, expect, it } from 'vitest';
import { errorCopy } from './error-copy';

describe('errorCopy', () => {
	it('says a 404 is a missing page', () => {
		expect(errorCopy(404).title).toBe('Page not found');
	});

	it('says anything else went wrong, without blaming the visitor', () => {
		for (const status of [400, 500, 502, 503]) {
			const { title, message } = errorCopy(status);

			expect(title).toBe('Something went wrong');
			expect(message).not.toMatch(/stack|trace|exception/i);
		}
	});
});
