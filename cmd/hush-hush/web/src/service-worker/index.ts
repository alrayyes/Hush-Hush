import { version } from '$app/env';
import { assets, immutable } from '$app/manifest';
import { resolve } from '$app/paths';
import { self } from '$app/service-worker';

// alrayyes/Hush-Hush#881. This worker exists so the browser will install the
// app, and it keeps the app shell and nothing else. Only the build's own files
// and the static assets are cached, matched by exact path. An API response can
// carry a secret's ciphertext or a token's metadata, so any other request goes
// to the network untouched and its response is never stored. The cache is
// named for the build, so a new release starts clean.

const CACHE = `shell-${version}`;

const SHELL = new Set([
	...immutable.map((file) => resolve(file.path)),
	...assets.map((file) => resolve(file.path)),
]);

self.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(CACHE)
			.then((cache) => cache.addAll([...SHELL]))
			.then(() => self.skipWaiting()),
	);
});

self.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((names) =>
				Promise.all(
					names
						.filter((name) => name !== CACHE)
						.map((name) => caches.delete(name)),
				),
			)
			.then(() => self.clients.claim()),
	);
});

self.addEventListener('fetch', (event) => {
	const { request } = event;
	if (request.method !== 'GET') return;

	const { pathname } = new URL(request.url);
	if (!SHELL.has(pathname)) return;

	event.respondWith(
		caches.match(pathname).then((hit) => hit ?? fetch(request)),
	);
});
