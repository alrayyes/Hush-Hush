import { describe, expect, it } from 'vitest';
import { isShellRequest } from './shell-cache';

// alrayyes/Hush-Hush#881: the service worker answers a request from its cache
// only when it is a GET for a file of the app shell. Anything else, an API
// call above all, goes to the network and is never stored.

const shell = new Set([
	'/_app/immutable/entry/app.js',
	'/manifest.webmanifest',
]);

describe('isShellRequest', () => {
	it('is true for a GET of a shell file', () => {
		expect(isShellRequest('GET', '/_app/immutable/entry/app.js', shell)).toBe(
			true,
		);
	});

	it.each([
		'/healthz',
		'/objects/db_password',
		'/tokens',
		'/auth/identity',
		'/',
	])('is false for %s, which is not a shell file', (pathname) => {
		expect(isShellRequest('GET', pathname, shell)).toBe(false);
	});

	it.each(['POST', 'PUT', 'PATCH', 'DELETE'])(
		'is false for a %s of a shell file',
		(method) => {
			expect(isShellRequest(method, '/manifest.webmanifest', shell)).toBe(
				false,
			);
		},
	);
});
