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
} from './api';

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

// registerPasskey runs the whole WebAuthn registration ceremony -
// settings' own "add a passkey" action (web-ui/spec.md's "Adding a
// passkey from settings" scenario). Throws on any failure, same as
// login().
export async function registerPasskey(nickname?: string): Promise<void> {
	const { publicKey } = (await beginRegistration()) as unknown as {
		publicKey: PublicKeyCredentialCreationOptionsJSON;
	};
	const credential = await startRegistration({ optionsJSON: publicKey });
	await finishRegistration(credential, nickname);
}
