import * as age from 'age-encryption';
import { describe, expect, it } from 'vitest';
import type { ConsumerEntry } from './api';
import { resolveRecipients, sealValue } from './sealing';

describe('resolveRecipients', () => {
	const entries: ConsumerEntry[] = [
		{ name: 'homelab/vps-docker', secret_count: 1, public_key: 'age1keyed' },
		{ name: 'homelab/no-key', secret_count: 2 },
	];

	it('resolves a consumer with a registered public key to that key', () => {
		const result = resolveRecipients(['homelab/vps-docker'], entries);

		expect(result.recipients).toEqual(['age1keyed']);
		expect(result.unresolved).toEqual([]);
	});

	it('resolves a consumer with no registered public key to no recipient', () => {
		const result = resolveRecipients(['homelab/no-key'], entries);

		expect(result.recipients).toEqual([]);
		expect(result.unresolved).toEqual(['homelab/no-key']);
	});

	it('resolves a brand new, not-yet-recorded consumer to no recipient', () => {
		const result = resolveRecipients(['homelab/unseen'], entries);

		expect(result.recipients).toEqual([]);
		expect(result.unresolved).toEqual(['homelab/unseen']);
	});

	it('resolves a mix of consumers independently', () => {
		const result = resolveRecipients(
			['homelab/vps-docker', 'homelab/no-key'],
			entries,
		);

		expect(result.recipients).toEqual(['age1keyed']);
		expect(result.unresolved).toEqual(['homelab/no-key']);
	});
});

describe('sealValue', () => {
	it('seals to a single recipient, decryptable with the matching identity', async () => {
		const identity = await age.generateIdentity();
		const recipient = await age.identityToRecipient(identity);

		const sealedBase64 = await sealValue('hunter2', [recipient]);

		const decrypter = new age.Decrypter();
		decrypter.addIdentity(identity);
		const ciphertext = Uint8Array.from(atob(sealedBase64), (c) =>
			c.charCodeAt(0),
		);
		const plaintext = await decrypter.decrypt(ciphertext, 'text');

		expect(plaintext).toBe('hunter2');
	});

	it('seals to multiple recipients, each independently able to decrypt', async () => {
		const identityA = await age.generateIdentity();
		const identityB = await age.generateIdentity();
		const recipientA = await age.identityToRecipient(identityA);
		const recipientB = await age.identityToRecipient(identityB);

		const sealedBase64 = await sealValue('shared-secret', [
			recipientA,
			recipientB,
		]);
		const ciphertext = Uint8Array.from(atob(sealedBase64), (c) =>
			c.charCodeAt(0),
		);

		for (const identity of [identityA, identityB]) {
			const decrypter = new age.Decrypter();
			decrypter.addIdentity(identity);
			expect(await decrypter.decrypt(ciphertext, 'text')).toBe('shared-secret');
		}
	});

	it('is not decryptable with an unrelated identity', async () => {
		const identity = await age.generateIdentity();
		const recipient = await age.identityToRecipient(identity);
		const stranger = await age.generateIdentity();

		const sealedBase64 = await sealValue('hunter2', [recipient]);
		const ciphertext = Uint8Array.from(atob(sealedBase64), (c) =>
			c.charCodeAt(0),
		);

		const decrypter = new age.Decrypter();
		decrypter.addIdentity(stranger);
		await expect(decrypter.decrypt(ciphertext, 'text')).rejects.toThrow();
	});
});
