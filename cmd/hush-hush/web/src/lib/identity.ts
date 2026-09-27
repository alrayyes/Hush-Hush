// The user's escrowed writer identity: a real age keypair generated
// client-side once, at first registration - its private key never
// reaches the server unwrapped (openspec/changes/client-side-encryption/
// specs/users/spec.md's "Escrowed writer identity" requirement). This
// module owns generating it, generating its break-glass recovery phrase,
// and wrapping/unwrapping copies of it against a WebAuthn PRF secret or
// the recovery phrase - design.md's "Escrowed identity: client-generated,
// server stores only wrapped copies" decision. The server (internal/api's
// register.go) only ever stores what this module already wrapped; it has
// no code path that unwraps or regenerates any of it.

import { generateMnemonic, mnemonicToSeedSync } from '@scure/bip39';
import { wordlist as englishWordlist } from '@scure/bip39/wordlists/english.js';
import {
	generateIdentity as generateAgeIdentity,
	identityToRecipient,
} from 'age-encryption';
import { listCredentials } from './api';

// PRF_SALT is the fixed, app-wide input to every WebAuthn PRF evaluation
// this app performs (design.md's "an app-wide fixed PRF salt is fine -
// don't over-engineer a per-credential random salt" decision). It isn't
// secret - it's domain separation, so this app's derived secret can never
// collide with another relying party's use of the same authenticator's
// PRF - so a fixed, human-readable value is as good as a random one.
const PRF_SALT = new TextEncoder().encode(
	'hush-hush/escrowed-identity/prf-salt-v1',
);

// PRF_WRAP_INFO/RECOVERY_WRAP_INFO are the HKDF "info" values that
// separate the two wrapping keys derived from a PRF secret and from a
// recovery phrase respectively, so the same underlying secret material
// (unlikely as that collision is) could never produce the same wrapping
// key for both purposes.
const PRF_WRAP_INFO = new TextEncoder().encode(
	'hush-hush/escrowed-identity/prf-wrap-v1',
);
const RECOVERY_WRAP_INFO = new TextEncoder().encode(
	'hush-hush/escrowed-identity/recovery-wrap-v1',
);

// WRAP_FORMAT_VERSION is a one-byte prefix on every wrapped blob this
// module produces, so a future format change can tell an old wrapped
// value apart from a new one instead of guessing from length alone.
const WRAP_FORMAT_VERSION = 1;

const GCM_IV_LENGTH = 12;

// asBufferSource satisfies the strict, buffer-generic BufferSource type
// newer TypeScript DOM lib versions expect from every WebCrypto call - a
// plain Uint8Array's own backing ArrayBufferLike type isn't narrow enough
// on its own to satisfy it structurally. Purely a TypeScript-side
// annotation gap (WebCrypto itself accepts any ArrayBufferView at
// runtime), not a real SharedArrayBuffer concern - nothing here ever
// touches one.
function asBufferSource(bytes: Uint8Array): BufferSource {
	return bytes as BufferSource;
}

function bytesToBase64(bytes: Uint8Array): string {
	let binary = '';
	const chunkSize = 0x8000;
	for (let i = 0; i < bytes.length; i += chunkSize) {
		binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
	}

	return btoa(binary);
}

function base64ToBytes(value: string): Uint8Array {
	const binary = atob(value);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i++) {
		bytes[i] = binary.charCodeAt(i);
	}

	return bytes;
}

// base64UrlToBytes decodes the base64url text form this service's
// credential ids are stored/exposed as (internal/api/webauthn.go's
// encodeCredentialID) - different alphabet and no padding, unlike the
// plain base64 wrap format above.
function base64UrlToBytes(value: string): Uint8Array {
	const padLength = (4 - (value.length % 4)) % 4;
	const padded =
		value.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat(padLength);

	return base64ToBytes(padded);
}

// deriveWrapKey turns a secret (a PRF output or a recovery phrase's seed)
// into an AES-256-GCM key via HKDF-SHA-256 - the secret itself is never
// used directly as a key, and info keeps the PRF-derived and
// recovery-phrase-derived keys from ever colliding.
async function deriveWrapKey(
	secret: Uint8Array,
	info: Uint8Array,
): Promise<CryptoKey> {
	const baseKey = await crypto.subtle.importKey(
		'raw',
		asBufferSource(secret),
		'HKDF',
		false,
		['deriveKey'],
	);

	return crypto.subtle.deriveKey(
		{
			name: 'HKDF',
			hash: 'SHA-256',
			salt: asBufferSource(new Uint8Array(0)),
			info: asBufferSource(info),
		},
		baseKey,
		{ name: 'AES-GCM', length: 256 },
		false,
		['encrypt', 'decrypt'],
	);
}

// wrapWithSecret encrypts plaintext with a key derived from secret,
// returning a base64 blob of [version byte][iv][ciphertext] - the shape
// every wrapped_identity/recovery_wrapped_identity value this app stores
// takes, on either side of the server boundary.
async function wrapWithSecret(
	secret: Uint8Array,
	info: Uint8Array,
	plaintext: Uint8Array,
): Promise<string> {
	const key = await deriveWrapKey(secret, info);
	const iv = crypto.getRandomValues(new Uint8Array(GCM_IV_LENGTH));
	const ciphertext = new Uint8Array(
		await crypto.subtle.encrypt(
			{ name: 'AES-GCM', iv: asBufferSource(iv) },
			key,
			asBufferSource(plaintext),
		),
	);

	const out = new Uint8Array(1 + iv.length + ciphertext.length);
	out[0] = WRAP_FORMAT_VERSION;
	out.set(iv, 1);
	out.set(ciphertext, 1 + iv.length);

	return bytesToBase64(out);
}

// unwrapErrorMessage is a single sentinel string rather than a per-call
// template, so a caller can tell "this wrap can't be opened with this
// secret" apart from a real programming error without parsing prose.
const unwrapErrorMessage = 'unable to unwrap: wrong secret or corrupt data';

async function unwrapWithSecret(
	secret: Uint8Array,
	info: Uint8Array,
	wrapped: string,
): Promise<Uint8Array> {
	const bytes = base64ToBytes(wrapped);
	if (bytes.length < 1 + GCM_IV_LENGTH || bytes[0] !== WRAP_FORMAT_VERSION) {
		throw new Error(unwrapErrorMessage);
	}

	const iv = bytes.slice(1, 1 + GCM_IV_LENGTH);
	const ciphertext = bytes.slice(1 + GCM_IV_LENGTH);
	const key = await deriveWrapKey(secret, info);

	try {
		const plaintext = await crypto.subtle.decrypt(
			{ name: 'AES-GCM', iv: asBufferSource(iv) },
			key,
			asBufferSource(ciphertext),
		);

		return new Uint8Array(plaintext);
	} catch {
		throw new Error(unwrapErrorMessage);
	}
}

export interface EscrowedIdentity {
	// identity is the age private key, "AGE-SECRET-KEY-1..." - never sent
	// to the server unwrapped.
	identity: string;
	// publicKey is the age recipient, "age1..." - safe to store server-side.
	publicKey: string;
}

// generateEscrowedIdentity creates a brand-new age keypair, client-side -
// design.md's "Escrowed identity: client-generated, server stores only
// wrapped copies" decision. Called exactly once per account, at the first
// registration (registerPasskey in auth.ts decides when that is).
export async function generateEscrowedIdentity(): Promise<EscrowedIdentity> {
	const identity = await generateAgeIdentity();
	const publicKey = await identityToRecipient(identity);

	return { identity, publicKey };
}

// generateRecoveryPhrase returns a 12-word BIP-39 mnemonic (128 bits of
// entropy) - reusing the same well-audited wordlist-plus-checksum scheme
// wallets use for seed backup, rather than a bespoke wordlist, since none
// already existed anywhere in this repo (CLAUDE.md's "check for an
// existing SDK before hand-rolling"). @scure/bip39 rather than the
// original bitcoinjs bip39 package: the latter's mnemonicToSeedSync
// returns a Node Buffer and pulls in Node's crypto/Buffer shims, which
// throw "Buffer is not defined" once actually bundled for the browser by
// Vite - @scure/bip39 is pure Uint8Array, from the same audited noble/
// scure ecosystem age-encryption itself already depends on.
export function generateRecoveryPhrase(): string {
	return generateMnemonic(englishWordlist, 128);
}

// recoveryPhraseSecret derives key material from a recovery phrase via
// @scure/bip39's own mnemonicToSeedSync (PBKDF2-HMAC-SHA512, 2048 rounds,
// salted with the fixed string "mnemonic") - the standard, audited way to
// turn a BIP-39 mnemonic into key material, reused here rather than a
// bespoke KDF given the phrase already carries 128 bits of entropy.
function recoveryPhraseSecret(phrase: string): Uint8Array {
	return mnemonicToSeedSync(phrase);
}

// wrapIdentityWithPrf/unwrapIdentityWithPrf wrap and unwrap the escrowed
// identity's private key against one credential's own PRF secret -
// specs/users/spec.md's "Per-credential wrapping of the escrowed
// identity" requirement.
export async function wrapIdentityWithPrf(
	identity: string,
	prfSecret: Uint8Array,
): Promise<string> {
	return wrapWithSecret(
		prfSecret,
		PRF_WRAP_INFO,
		new TextEncoder().encode(identity),
	);
}

export async function unwrapIdentityWithPrf(
	wrapped: string,
	prfSecret: Uint8Array,
): Promise<string> {
	const bytes = await unwrapWithSecret(prfSecret, PRF_WRAP_INFO, wrapped);

	return new TextDecoder().decode(bytes);
}

// wrapIdentityWithRecoveryPhrase wraps the escrowed identity's private
// key against the one-time recovery phrase - specs/users/spec.md's
// "Break-glass recovery phrase" requirement. The phrase itself is shown
// exactly once and never stored, so the wrap has to exist from day one
// even before any UI flow calls unwrapIdentityWithRecoveryPhrase below.
export async function wrapIdentityWithRecoveryPhrase(
	identity: string,
	phrase: string,
): Promise<string> {
	return wrapWithSecret(
		recoveryPhraseSecret(phrase),
		RECOVERY_WRAP_INFO,
		new TextEncoder().encode(identity),
	);
}

// unwrapIdentityWithRecoveryPhrase recovers the escrowed identity from its
// recovery-phrase-wrapped copy - the symmetric counterpart to
// wrapIdentityWithRecoveryPhrase above. Not called from any UI flow yet
// (a break-glass "recover with your phrase" page is future work, not part
// of tasks.md group 3 or 5), but cmd/hush-hush/web/e2e/journey.spec.ts
// already exercises it directly: it's what lets that test recover the
// same escrowed identity registration generated, to prove the
// owner-recipient opt-in checkbox (tasks.md's 5.2) really does add it as
// a decrypt recipient, and only when checked.
export async function unwrapIdentityWithRecoveryPhrase(
	wrapped: string,
	phrase: string,
): Promise<string> {
	const bytes = await unwrapWithSecret(
		recoveryPhraseSecret(phrase),
		RECOVERY_WRAP_INFO,
		wrapped,
	);

	return new TextDecoder().decode(bytes);
}

// prfSupportProbeExtension is the bare "prf":{} extension input this
// app's WebAuthn create() call requests, matching internal/api/register.go's
// own server-side webauthn.WithExtensionPRFSupport() exactly -
// go-webauthn's registration verification rejects a "prf" client
// extension output the registration options never asked for, so the
// client and server have to request the identical thing. This only
// probes for support (the "enabled" output); it never asks the
// authenticator to actually evaluate the extension at registration time,
// since not every authenticator supports that and this app doesn't rely
// on it either way (prfEvalExtension below is what actually derives a
// secret, in a separate, purely local assertion).
export function prfSupportProbeExtension(): { prf: Record<string, never> } {
	return { prf: {} };
}

// prfEvalExtension is the extension input a purely local WebAuthn
// assertion (evaluatePrfSecret below) sends to actually evaluate the PRF
// extension against the app's fixed salt - never sent as part of a
// server-verified ceremony, so it never has to match anything go-webauthn
// itself requested.
function prfEvalExtension(): { prf: { eval: { first: Uint8Array } } } {
	return { prf: { eval: { first: PRF_SALT } } };
}

// credentialSupportsPrf reads whether a WebAuthn ceremony's own extension
// results reported PRF support - specs/users/spec.md's "PRF support is
// detected at registration" requirement. `enabled` is what the spec
// defines as the authoritative signal, independent of whether `results`
// happened to come back on this same call (some authenticators only
// confirm support at registration and require a later assertion to
// actually evaluate the extension).
export function credentialSupportsPrf(
	extensionResults: { prf?: { enabled?: boolean } } | undefined,
): boolean {
	return extensionResults?.prf?.enabled === true;
}

// prfSecretFromResults reads the actual derived PRF secret out of a
// WebAuthn ceremony's extension results, if this call evaluated it.
// `first` is left untyped rather than pinned to BufferSource: the real
// value is always an ArrayBuffer or a typed-array view over one (what the
// WebAuthn spec calls BufferSource), but @simplewebauthn/browser ships
// its own separately-declared copy of that name, which newer TypeScript
// DOM lib versions' generic ArrayBufferView no longer considers identical
// to this module's ambient BufferSource - bufferSourceToBytes handles
// either shape correctly regardless of which declaration the caller's
// value was typed against.
export function prfSecretFromResults(
	extensionResults: { prf?: { results?: { first?: unknown } } } | undefined,
): Uint8Array | undefined {
	const first = extensionResults?.prf?.results?.first;

	return first ? bufferSourceToBytes(first as BufferSource) : undefined;
}

function bufferSourceToBytes(source: BufferSource): Uint8Array {
	return ArrayBuffer.isView(source)
		? new Uint8Array(source.buffer, source.byteOffset, source.byteLength)
		: new Uint8Array(source);
}

// evaluatePrfSecret runs a local, server-independent WebAuthn assertion
// against one specific credential purely to evaluate its PRF extension -
// no server round trip, and no go-webauthn ceremony verification, since
// this isn't authenticating anything: it only needs the authenticator to
// derive the app's fixed-salt PRF secret, whether that's right after
// creating this same credential (registerPasskey in auth.ts, for both the
// first-ever registration and any later one) or against an
// already-registered one (recoverEscrowedIdentity below). Returns
// undefined if the assertion is cancelled or the authenticator doesn't
// return a PRF result (it doesn't support the extension after all, or the
// caller picked the wrong credential id).
export async function evaluatePrfSecret(
	credentialID: string,
): Promise<Uint8Array | undefined> {
	const assertion = (await navigator.credentials.get({
		publicKey: {
			challenge: crypto.getRandomValues(new Uint8Array(32)),
			allowCredentials: [
				{ id: base64UrlToBytes(credentialID), type: 'public-key' },
			],
			userVerification: 'required',
			extensions: prfEvalExtension(),
		},
		// The real lib.dom PublicKeyCredentialRequestOptions type may or may
		// not yet declare the "prf" extension field, depending on the
		// TypeScript version's own WebAuthn Level 3 coverage - this call
		// bypasses @simplewebauthn/browser entirely (there's no server
		// ceremony to verify), so there's no library-provided type to lean
		// on the way beginRegistration/beginLogin's responses do.
	} as unknown as CredentialRequestOptions)) as PublicKeyCredential | null;

	if (!assertion) {
		return undefined;
	}

	const results = assertion.getClientExtensionResults() as {
		prf?: { results?: { first?: BufferSource } };
	};

	return prfSecretFromResults(results);
}

// recoverEscrowedIdentity recovers the account's escrowed identity
// client-side by unwrapping any existing credential's own wrapped copy
// through a fresh local PRF evaluation against that same credential -
// what lets registering another passkey wrap a fresh copy for it without
// the server ever handling the private key (design.md's "Multi-copy
// wrapping over a single shared wrap" decision). Returns undefined if no
// registered credential has a wrapped copy this browser can re-derive the
// PRF secret for (every existing passkey is non-PRF, or the assertion is
// cancelled) - registerPasskey's caller then falls back to registering
// the new credential without a wrapped copy of its own, the same
// treatment as a non-PRF credential.
export async function recoverEscrowedIdentity(): Promise<string | undefined> {
	const credentials = await listCredentials();

	for (const credential of credentials) {
		if (!credential.wrapped_identity) {
			continue;
		}

		const secret = await evaluatePrfSecret(credential.id);
		if (!secret) {
			continue;
		}

		try {
			return await unwrapIdentityWithPrf(credential.wrapped_identity, secret);
		} catch {
			// This credential's own wrapped copy didn't unwrap with the
			// secret its own PRF just produced - shouldn't happen for a
			// copy this app wrapped itself, but falling through to try
			// another credential is safer than failing the whole
			// registration over one corrupt row.
		}
	}

	return undefined;
}
