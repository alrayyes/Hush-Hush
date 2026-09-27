import { describe, expect, it } from 'vitest';
import { bytesToBase64 } from './encoding';

describe('bytesToBase64', () => {
	it('encodes plain ASCII bytes the same way btoa does', () => {
		const bytes = new TextEncoder().encode('hello');

		expect(bytesToBase64(bytes)).toBe(btoa('hello'));
	});

	it('round-trips arbitrary binary data, not just text', () => {
		const bytes = Uint8Array.from({ length: 256 }, (_, i) => i);

		const decoded = Uint8Array.from(atob(bytesToBase64(bytes)), (c) =>
			c.charCodeAt(0),
		);
		expect(decoded).toEqual(bytes);
	});

	it('round-trips a large value without a call-stack overflow', () => {
		const bytes = new Uint8Array(200_000).fill(42);

		const decoded = Uint8Array.from(atob(bytesToBase64(bytes)), (c) =>
			c.charCodeAt(0),
		);
		expect(decoded).toEqual(bytes);
	});
});
