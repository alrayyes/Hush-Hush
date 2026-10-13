// The service worker's one caching decision (alrayyes/Hush-Hush#881). A request
// is answered from the cache only when it is a GET for a file of the app shell.
// Everything else, an API call above all, goes to the network untouched and its
// response is never stored: it can carry a secret's sealed value or a token's
// metadata. Kept apart from the worker so it can be tested without one.
export function isShellRequest(
	method: string,
	pathname: string,
	shell: ReadonlySet<string>,
): boolean {
	return method === 'GET' && shell.has(pathname);
}
