import { describe, expect, it, vi } from 'vitest';

// go-webauthn's BeginRegistration/BeginLogin return the full
// CredentialCreation/CredentialAssertion struct - {"publicKey": {...}} -
// the exact shape navigator.credentials.create()/get() take
// (api/openapi.yaml's RegistrationOptions/LoginOptions description).
// @simplewebauthn/browser's startRegistration/startAuthentication instead
// want just the inner PublicKeyCredentialCreationOptionsJSON/
// PublicKeyCredentialRequestOptionsJSON, unwrapped.
const beginRegistrationResponse = {
	publicKey: { challenge: 'reg-challenge', rp: { id: 'example.test' } },
};
const beginLoginResponse = {
	publicKey: { challenge: 'login-challenge', rpId: 'example.test' },
};

const { startRegistration, startAuthentication } = vi.hoisted(() => ({
	startRegistration: vi.fn().mockResolvedValue({ id: 'new-cred' }),
	startAuthentication: vi.fn().mockResolvedValue({ id: 'existing-cred' }),
}));

vi.mock('@simplewebauthn/browser', () => ({
	startRegistration,
	startAuthentication,
}));

vi.mock('./api', () => ({
	beginRegistration: vi.fn().mockResolvedValue(beginRegistrationResponse),
	finishRegistration: vi.fn().mockResolvedValue(undefined),
	beginLogin: vi.fn().mockResolvedValue(beginLoginResponse),
	finishLogin: vi.fn().mockResolvedValue(undefined),
}));

describe('registerPasskey', () => {
	it('unwraps the publicKey field before handing it to startRegistration', async () => {
		const { registerPasskey } = await import('./auth');

		await registerPasskey();

		expect(startRegistration).toHaveBeenCalledWith({
			optionsJSON: beginRegistrationResponse.publicKey,
		});
	});
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
