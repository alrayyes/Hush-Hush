// Client-side age sealing (openspec/changes/client-side-encryption,
// task group 4) - replaces the "New secret" dialog's old plain-text and
// paste-in-ciphertext modes with real encryption in the browser, sealed
// to the resolved consumer recipients, before the value ever reaches the
// server. `age-encryption` (filippo.io's own TypeScript port of age,
// npm's `age-encryption`) is used rather than a WASM implementation:
// it's maintained by age's own author, needs no separate wasm asset for
// this SPA to serve, and its multi-recipient `Encrypter.addRecipient`
// matches ADR 0001's per-object, multi-recipient sealing model exactly.
import { Encrypter } from 'age-encryption';
import type { ConsumerEntry } from './api';
import { bytesToBase64 } from './encoding';

export interface ResolvedRecipients {
	// Registered age public keys, one per selected/added consumer that
	// has one - what the value actually gets sealed to.
	recipients: string[];
	// Selected/added consumer names with no registered public key -
	// specs/consumers/spec.md's "Picking a consumer with no registered
	// public key resolves no recipient" scenario: these contribute no
	// recipient, and the form surfaces them rather than silently sealing
	// to fewer recipients than the admin picked.
	unresolved: string[];
}

// resolveRecipients maps each used_by name to its registered public key
// from the consumer directory (entries), when it has one.
export function resolveRecipients(
	usedBy: string[],
	entries: ConsumerEntry[],
): ResolvedRecipients {
	const keyByName = new Map(
		entries.map((entry) => [entry.name, entry.public_key]),
	);
	const recipients: string[] = [];
	const unresolved: string[] = [];

	for (const name of usedBy) {
		const key = keyByName.get(name);
		if (key) {
			recipients.push(key);
		} else {
			unresolved.push(name);
		}
	}

	return { recipients, unresolved };
}

// sealValue age-encrypts plaintext to every recipient public key given,
// returning the sealed bytes base64-encoded - the same wire shape
// CreateObjectRequest/updateObject's own `value` field already expects
// (api/openapi.yaml is unaffected by this change; only what produces the
// ciphertext it carries is).
export async function sealValue(
	plaintext: string,
	recipients: string[],
): Promise<string> {
	const encrypter = new Encrypter();
	for (const recipient of recipients) {
		encrypter.addRecipient(recipient);
	}

	const ciphertext = await encrypter.encrypt(plaintext);

	return bytesToBase64(ciphertext);
}
