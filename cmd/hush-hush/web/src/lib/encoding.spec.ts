import { describe, expect, it } from 'vitest';
import { utf8ToBase64 } from './encoding';

describe('utf8ToBase64', () => {
	it('encodes plain ASCII the same way btoa does', () => {
		expect(utf8ToBase64('hello')).toBe(btoa('hello'));
	});

	it('encodes multi-byte UTF-8 text that plain btoa cannot handle', () => {
		const encoded = utf8ToBase64('café 🔐');

		const decoded = new TextDecoder().decode(
			Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0)),
		);
		expect(decoded).toBe('café 🔐');
	});

	it('round-trips a large value without a call-stack overflow', () => {
		const large = 'x'.repeat(200_000);

		const decoded = new TextDecoder().decode(
			Uint8Array.from(atob(utf8ToBase64(large)), (c) => c.charCodeAt(0)),
		);
		expect(decoded).toBe(large);
	});
});
