import { afterEach, describe, expect, it, vi } from 'vitest';
import { unwrapIdentityWithPrf, wrapIdentityWithPrf } from './identity';

// go-webauthn's BeginRegistration/BeginLogin return the full
// CredentialCreation/CredentialAssertion struct - {"publicKey": {...}} -
// the exact shape navigator.credentials.create()/get() take
// (api/openapi.yaml's RegistrationOptions/LoginOptions description).
// @simplewebauthn/browser's startRegistration/startAuthentication instead
// want just the inner PublicKeyCredentialCreationOptionsJSON/
// PublicKeyCredentialRequestOptionsJSON, unwrapped. Declared inside
// vi.hoisted alongside the mocks that reference them - vi.mock/vi.hoisted
// bodies are hoisted above ordinary top-level declarations, so a plain
// `const` referenced from inside one would throw a TDZ error.
const {
	beginRegistrationResponse,
	beginLoginResponse,
	getAuthStatus,
	beginRegistration,
	finishRegistration,
	listCredentials,
	beginLogin,
	finishLogin,
} = vi.hoisted(() => {
	const beginRegistrationResponse = {
		publicKey: { challenge: 'reg-challenge', rp: { id: 'example.test' } },
	};
	const beginLoginResponse = {
		publicKey: { challenge: 'login-challenge', rpId: 'example.test' },
	};

	return {
		beginRegistrationResponse,
		beginLoginResponse,
		getAuthStatus: vi.fn(),
		beginRegistration: vi.fn().mockResolvedValue(beginRegistrationResponse),
		finishRegistration: vi.fn().mockResolvedValue(undefined),
		listCredentials: vi.fn().mockResolvedValue([]),
		beginLogin: vi.fn().mockResolvedValue(beginLoginResponse),
		finishLogin: vi.fn().mockResolvedValue(undefined),
	};
});

const { startRegistration, startAuthentication } = vi.hoisted(() => ({
	startRegistration: vi.fn(),
	startAuthentication: vi.fn().mockResolvedValue({ id: 'existing-cred' }),
}));

vi.mock('@simplewebauthn/browser', () => ({
	startRegistration,
	startAuthentication,
}));

// identity.ts's own recoverEscrowedIdentity imports listCredentials from
// this same module - mocking it once here covers both auth.ts's direct
// calls and identity.ts's.
vi.mock('./api', () => ({
	getAuthStatus,
	beginRegistration,
	finishRegistration,
	listCredentials,
	beginLogin,
	finishLogin,
}));

afterEach(() => {
	vi.unstubAllGlobals();
	startRegistration.mockReset();
	finishRegistration.mockClear();
	listCredentials.mockReset().mockResolvedValue([]);
});

describe('login', () => {
	it('unwraps the publicKey field before handing it to startAuthentication', async () => {
		const { login } = await import('./auth');

		await login();

		expect(startAuthentication).toHaveBeenCalledWith({
			optionsJSON: beginLoginResponse.publicKey,
		});
	});
});

// clientExtensionResults shapes a real browser's create() call hands back
// from getClientExtensionResults() - specs/users/spec.md's "PRF support
// is detected at registration" requirement is about telling these two
// apart, not about any particular authenticator vendor's quirks. create()
// only ever reports "enabled" (the app requests the bare support probe,
// never eval, at registration - identity.ts's prfSupportProbeExtension) -
// a real derived secret only ever comes from a separate, purely local
// assertion this app runs afterward, stubbed below via
// navigator.credentials.get.
const prfCapableExtensionResults = { prf: { enabled: true } };
const nonPrfExtensionResults = {};

// stubPrfAssertion makes the next local navigator.credentials.get call -
// identity.ts's evaluatePrfSecret, which never goes through
// @simplewebauthn/browser or a server ceremony - resolve with secret as
// this call's derived PRF output. Each call in the queue serves one
// evaluatePrfSecret invocation, in the order auth.ts makes them.
function stubPrfAssertion(...secrets: Uint8Array[]) {
	const get = vi.fn();
	for (const secret of secrets) {
		get.mockResolvedValueOnce({
			getClientExtensionResults: () => ({
				prf: { results: { first: secret.buffer } },
			}),
		});
	}

	vi.stubGlobal('navigator', { credentials: { get } });
}

describe('registerPasskey', () => {
	it('unwraps the publicKey field before handing it to startRegistration', async () => {
		getAuthStatus.mockResolvedValue(true);
		startRegistration.mockResolvedValue({
			id: 'new-cred',
			clientExtensionResults: nonPrfExtensionResults,
		});

		const { registerPasskey } = await import('./auth');
		await registerPasskey();

		expect(startRegistration).toHaveBeenCalledWith({
			optionsJSON: expect.objectContaining(beginRegistrationResponse.publicKey),
		});
	});

	// tasks.md 3.1: every registration requests the bare "prf":{} support
	// probe - matching internal/api/register.go's own server-side
	// webauthn.WithExtensionPRFSupport(), which is why this can't also ask
	// for eval here (go-webauthn's own registration verification rejects a
	// "prf" client extension output the registration options never asked
	// for) - covered together with the non-PRF case below so the same test
	// file proves the detection tells the two apart, not just that one of
	// them works.
	it('requests the bare prf support probe at registration', async () => {
		getAuthStatus.mockResolvedValue(true);
		startRegistration.mockResolvedValue({
			id: 'new-cred',
			clientExtensionResults: nonPrfExtensionResults,
		});

		const { registerPasskey } = await import('./auth');
		await registerPasskey();

		const [{ optionsJSON }] = startRegistration.mock.calls[0];
		expect(optionsJSON.extensions.prf).toEqual({});
	});

	// tasks.md 3.2/3.3: the first-ever registration (no account yet)
	// generates the escrowed identity and its one-time recovery phrase,
	// wraps the identity with the recovery phrase unconditionally, and -
	// since this simulated authenticator reports PRF support - also wraps
	// it with the credential's own PRF secret.
	it('on a first-ever registration with a PRF-capable authenticator, generates the identity, shows the recovery phrase once, and wraps a copy for the credential', async () => {
		getAuthStatus.mockResolvedValue(false);
		const prfSecret = crypto.getRandomValues(new Uint8Array(32));
		startRegistration.mockResolvedValue({
			id: 'new-cred',
			clientExtensionResults: prfCapableExtensionResults,
		});
		stubPrfAssertion(prfSecret);

		const { registerPasskey } = await import('./auth');
		const result = await registerPasskey();

		// The recovery phrase is returned exactly once, to the caller that
		// drives the UI - never persisted or requested back from anywhere
		// (specs/users/spec.md's "Recovery phrase shown once at setup"
		// scenario).
		expect(result.recoveryPhrase).toBeTruthy();
		expect(result.recoveryPhrase?.split(' ')).toHaveLength(12);

		expect(finishRegistration).toHaveBeenCalledTimes(1);
		const [, , escrow] = finishRegistration.mock.calls[0];
		expect(escrow.public_key).toMatch(/^age1/);
		expect(escrow.recovery_wrapped_identity).toBeTruthy();
		expect(escrow.wrapped_identity).toBeTruthy();

		// The wrapped copy actually unwraps back to an age identity with
		// this same PRF secret - proof this isn't just any string, but a
		// real wrap of the real generated identity.
		const recovered = await unwrapIdentityWithPrf(
			escrow.wrapped_identity,
			prfSecret,
		);
		expect(recovered).toMatch(/^AGE-SECRET-KEY-1/);
	});

	// tasks.md 3.1/3.3 and specs/users/spec.md's "Authenticator without PRF
	// support registers without wrapping" scenario: the identity and
	// recovery phrase are still generated (the recovery phrase is
	// unconditional, design.md's own decision), but no wrapped_identity is
	// sent for this credential.
	it('on a first-ever registration with a non-PRF authenticator, still generates the identity and recovery phrase but sends no wrapped copy', async () => {
		getAuthStatus.mockResolvedValue(false);
		startRegistration.mockResolvedValue({
			id: 'new-cred',
			clientExtensionResults: nonPrfExtensionResults,
		});

		const { registerPasskey } = await import('./auth');
		const result = await registerPasskey();

		expect(result.recoveryPhrase?.split(' ')).toHaveLength(12);

		const [, , escrow] = finishRegistration.mock.calls[0];
		expect(escrow.public_key).toMatch(/^age1/);
		expect(escrow.recovery_wrapped_identity).toBeTruthy();
		expect(escrow.wrapped_identity).toBeUndefined();
	});

	// specs/users/spec.md's "Registering a second PRF-capable passkey
	// wraps a second copy" scenario, end to end on the client: adding a
	// second passkey to an already-bootstrapped account never resends
	// public_key/recovery_wrapped_identity (the identity was already
	// escrowed), recovers the identity locally through the first
	// credential's own wrapped copy and PRF secret, and wraps a fresh,
	// independent copy for the new credential.
	it('on a later registration with a PRF-capable authenticator, recovers the existing identity and wraps a fresh copy for the new credential', async () => {
		getAuthStatus.mockResolvedValue(true);

		const firstCredentialSecret = crypto.getRandomValues(new Uint8Array(32));
		const existingIdentity = 'AGE-SECRET-KEY-1EXISTINGTESTVECTOR';
		const existingWrapped = await wrapIdentityWithPrf(
			existingIdentity,
			firstCredentialSecret,
		);

		listCredentials.mockResolvedValue([
			{
				id: 'first-cred',
				created_at: '2026-01-01T00:00:00Z',
				wrapped_identity: existingWrapped,
			},
		]);

		const secondCredentialSecret = crypto.getRandomValues(new Uint8Array(32));
		// auth.ts's own call order: first evaluatePrfSecret is for the
		// just-created second credential itself, second is
		// recoverEscrowedIdentity's own call against the first credential.
		stubPrfAssertion(secondCredentialSecret, firstCredentialSecret);

		startRegistration.mockResolvedValue({
			id: 'second-cred',
			clientExtensionResults: prfCapableExtensionResults,
		});

		const { registerPasskey } = await import('./auth');
		const result = await registerPasskey();

		expect(result.recoveryPhrase).toBeUndefined();

		const [, , escrow] = finishRegistration.mock.calls[0];
		expect(escrow.public_key).toBeUndefined();
		expect(escrow.recovery_wrapped_identity).toBeUndefined();
		expect(escrow.wrapped_identity).toBeTruthy();
		expect(escrow.wrapped_identity).not.toEqual(existingWrapped);

		const recovered = await unwrapIdentityWithPrf(
			escrow.wrapped_identity,
			secondCredentialSecret,
		);
		expect(recovered).toBe(existingIdentity);
	});

	// No existing credential this browser can recover the identity from
	// (every one is non-PRF, or the local assertion is cancelled) - the
	// new credential still registers, just without a wrapped copy, same
	// as a non-PRF credential.
	it('on a later registration, registers without a wrapped copy when the existing identity cannot be recovered locally', async () => {
		getAuthStatus.mockResolvedValue(true);
		listCredentials.mockResolvedValue([
			{ id: 'first-cred', created_at: '2026-01-01T00:00:00Z' },
		]);

		const secondCredentialSecret = crypto.getRandomValues(new Uint8Array(32));
		// Only ever called once - the new credential's own eval;
		// recoverEscrowedIdentity never calls it again since the one
		// existing credential has no wrapped_identity to recover.
		stubPrfAssertion(secondCredentialSecret);

		startRegistration.mockResolvedValue({
			id: 'second-cred',
			clientExtensionResults: prfCapableExtensionResults,
		});

		const { registerPasskey } = await import('./auth');
		const result = await registerPasskey();

		expect(result.recoveryPhrase).toBeUndefined();
		const [, , escrow] = finishRegistration.mock.calls[0];
		expect(escrow.wrapped_identity).toBeUndefined();
		expect(escrow.public_key).toBeUndefined();
	});
});
