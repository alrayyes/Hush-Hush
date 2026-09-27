import {
	type PublicKeyCredentialCreationOptionsJSON,
	type PublicKeyCredentialRequestOptionsJSON,
	startAuthentication,
	startRegistration,
} from '@simplewebauthn/browser';
import {
	beginLogin,
	beginRegistration,
	finishLogin,
	finishRegistration,
	getAuthStatus,
	type RegistrationEscrow,
} from './api';
import {
	credentialSupportsPrf,
	evaluatePrfSecret,
	generateEscrowedIdentity,
	generateRecoveryPhrase,
	prfSupportProbeExtension,
	recoverEscrowedIdentity,
	wrapIdentityWithPrf,
	wrapIdentityWithRecoveryPhrase,
} from './identity';

// login runs the whole WebAuthn login ceremony - begin, the browser
// prompt, finish - design.md's "WebAuthn library: go-webauthn/webauthn
// server-side, @simplewebauthn/browser client-side" decision. Throws on
// any failure (a rejected/cancelled browser prompt, an invalid
// assertion) - the caller decides how to show that.
export async function login(): Promise<void> {
	// api/openapi.yaml documents LoginOptions as an opaque
	// additionalProperties: true blob, passed straight through to the
	// browser - the real shape is only known by the WebAuthn/SimpleWebAuthn
	// contract on both ends, not by the API's own schema. go-webauthn's
	// BeginLogin returns the full CredentialRequestOptions dictionary
	// (navigator.credentials.get()'s own argument shape, {publicKey: ...}),
	// but startAuthentication wants just the inner options.
	const { publicKey } = (await beginLogin()) as unknown as {
		publicKey: PublicKeyCredentialRequestOptionsJSON;
	};
	const credential = await startAuthentication({ optionsJSON: publicKey });
	await finishLogin(credential);
}

// RegisterPasskeyResult is what registerPasskey hands back to its caller -
// recoveryPhrase is only ever set on the account's first-ever
// registration, and only this one return value ever carries it
// (specs/users/spec.md's "Break-glass recovery phrase" requirement: shown
// once, never re-displayed, never requested back from the server). The
// caller (login/+page.svelte) is responsible for actually showing it;
// this function itself never persists it anywhere.
export interface RegisterPasskeyResult {
	recoveryPhrase?: string;
}

// registerPasskey runs the whole WebAuthn registration ceremony -
// settings' own "add a passkey" action (web-ui/spec.md's "Adding a
// passkey from settings" scenario), and login's own first-account
// bootstrap. Throws on any failure, same as login(). Also drives group
// 3's escrowed-identity flow end to end (design.md's "Escrowed identity:
// client-generated, server stores only wrapped copies" decision):
//
//   - The create() call itself only requests the bare "prf":{} support
//     probe, matching internal/api/register.go's own
//     webauthn.WithExtensionPRFSupport() - go-webauthn's own registration
//     verification rejects a "prf" client extension output the
//     registration options never asked for, so this can't also request
//     eval here the way a purely local assertion can. Actually deriving
//     the secret (if this credential reports support) is a second,
//     separate local assertion (identity.ts's evaluatePrfSecret) against
//     this same just-created credential - no server involved, so nothing
//     there has to match what the create() options requested.
//   - First-ever registration (no account yet): generates the identity
//     and its one-time recovery phrase, wraps the identity with the
//     recovery phrase unconditionally, and - if this credential reports
//     PRF support - also wraps it with this credential's PRF secret.
//   - A later registration (adding another passkey): the identity
//     already exists server-side only in wrapped form, so this recovers
//     it locally through any existing PRF-capable credential before
//     wrapping a fresh copy for the new one. If no existing credential
//     can be recovered from client-side, the new credential still
//     registers, just without a wrapped copy - the same as a non-PRF
//     credential.
export async function registerPasskey(
	nickname?: string,
): Promise<RegisterPasskeyResult> {
	const isFirstRegistration = !(await getAuthStatus());

	const { publicKey } = (await beginRegistration()) as unknown as {
		publicKey: PublicKeyCredentialCreationOptionsJSON;
	};

	const optionsWithPrfProbe: PublicKeyCredentialCreationOptionsJSON = {
		...publicKey,
		extensions: { ...publicKey.extensions, ...prfSupportProbeExtension() },
	};

	const credential = await startRegistration({
		optionsJSON: optionsWithPrfProbe,
	});
	const prfCapable = credentialSupportsPrf(credential.clientExtensionResults);
	const prfSecret = prfCapable
		? await evaluatePrfSecret(credential.id)
		: undefined;

	const escrow: RegistrationEscrow = {};
	let recoveryPhrase: string | undefined;

	if (isFirstRegistration) {
		const identity = await generateEscrowedIdentity();
		recoveryPhrase = generateRecoveryPhrase();

		escrow.public_key = identity.publicKey;
		escrow.recovery_wrapped_identity = await wrapIdentityWithRecoveryPhrase(
			identity.identity,
			recoveryPhrase,
		);

		if (prfSecret) {
			escrow.wrapped_identity = await wrapIdentityWithPrf(
				identity.identity,
				prfSecret,
			);
		}
	} else if (prfSecret) {
		const recoveredIdentity = await recoverEscrowedIdentity();
		if (recoveredIdentity) {
			escrow.wrapped_identity = await wrapIdentityWithPrf(
				recoveredIdentity,
				prfSecret,
			);
		}
	}

	await finishRegistration(credential, nickname, escrow);

	return { recoveryPhrase };
}
