import {
	type PublicKeyCredentialRequestOptionsJSON,
	startAuthentication,
} from '@simplewebauthn/browser';
import { beginLogin, finishLogin } from './api';

// login runs the whole WebAuthn login ceremony - begin, the browser
// prompt, finish - design.md's "WebAuthn library: go-webauthn/webauthn
// server-side, @simplewebauthn/browser client-side" decision. Throws on
// any failure (a rejected/cancelled browser prompt, an invalid
// assertion) - the caller decides how to show that.
export async function login(): Promise<void> {
	// api/openapi.yaml documents LoginOptions as an opaque
	// additionalProperties: true blob, passed straight through to the
	// browser - the real shape is only known by the WebAuthn/SimpleWebAuthn
	// contract on both ends, not by the API's own schema.
	const optionsJSON =
		(await beginLogin()) as unknown as PublicKeyCredentialRequestOptionsJSON;
	const credential = await startAuthentication({ optionsJSON });
	await finishLogin(credential);
}
