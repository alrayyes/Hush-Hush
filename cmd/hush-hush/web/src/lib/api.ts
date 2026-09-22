// Thin wrapper over fetch for the Go API - same origin, no base URL, per
// design.md's "SvelteKit + adapter-static" decision ("every data access
// goes through the Go API via fetch"). Every state-changing call reads
// the session's CSRF token from the readable csrf_token cookie and
// echoes it back, matching auth/spec.md's double-submit requirement.

export class ApiError extends Error {
	status: number;

	constructor(status: number, message: string) {
		super(message);
		this.status = status;
	}
}

function csrfToken(): string {
	const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/);

	return match ? decodeURIComponent(match[1]) : '';
}

async function request(
	path: string,
	init: RequestInit = {},
): Promise<Response> {
	const method = (init.method ?? 'GET').toUpperCase();
	const headers = new Headers(init.headers);

	if (method !== 'GET' && method !== 'HEAD') {
		headers.set('X-CSRF-Token', csrfToken());
	}

	if (init.body !== undefined && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}

	const res = await fetch(path, {
		...init,
		headers,
		credentials: 'same-origin',
	});

	if (!res.ok) {
		let message = res.statusText;

		try {
			const body = (await res.json()) as { error?: string };
			if (typeof body.error === 'string') {
				message = body.error;
			}
		} catch {
			// No JSON body (or an empty one) - the status text is the best we have.
		}

		throw new ApiError(res.status, message);
	}

	return res;
}

// WebAuthn ceremony payloads are opaque to this client - api/openapi.yaml
// documents RegistrationOptions/LoginOptions/*FinishRequest as
// additionalProperties: true, passed straight through to and from
// @simplewebauthn/browser.

export async function beginLogin(): Promise<Record<string, unknown>> {
	const res = await request('/auth/login/begin', { method: 'POST' });

	return res.json();
}

export async function finishLogin(credential: unknown): Promise<void> {
	await request('/auth/login/finish', {
		method: 'POST',
		body: JSON.stringify({ credential }),
	});
}

// beginRegistration/finishRegistration cover both first-run account
// creation (no session yet) and adding another passkey to the existing
// account from settings (session required) - the server tells the two
// apart on its own (auth/spec.md's "Registering a first passkey" vs.
// "Registering another passkey" scenarios), so this client doesn't need
// to know which case it's in.
export async function beginRegistration(): Promise<Record<string, unknown>> {
	const res = await request('/auth/register/begin', { method: 'POST' });

	return res.json();
}

export async function finishRegistration(
	credential: unknown,
	nickname?: string,
): Promise<void> {
	await request('/auth/register/finish', {
		method: 'POST',
		body: JSON.stringify({ credential, nickname }),
	});
}

export async function logout(): Promise<void> {
	await request('/auth/logout', { method: 'POST' });
}

// getAuthStatus reports whether an admin account exists yet - a
// read-only, side-effect-free check (no cookie set, no ceremony started)
// the login page uses to decide whether to offer registering the first
// passkey or logging in with one
// (openspec/changes/gate-passkey-registration-ui/design.md).
export async function getAuthStatus(): Promise<boolean> {
	const res = await request('/auth/status');
	const body = (await res.json()) as { bootstrapped: boolean };

	return body.bootstrapped;
}

// checkSession reports whether the current visitor holds a valid session,
// via a session-gated endpoint that carries no secret data of its own -
// there's no dedicated "who am I" endpoint to call instead.
export async function checkSession(): Promise<boolean> {
	try {
		await request('/credentials');

		return true;
	} catch (err) {
		if (err instanceof ApiError && err.status === 401) {
			return false;
		}

		throw err;
	}
}

export interface ObjectMetadata {
	id: string;
	description?: string;
	used_by?: string[];
}

// listObjects returns every stored object's metadata, or only those whose
// recorded used_by lineage includes usedBy when given - the consumers
// directory page's "select a consumer" navigation reuses this same
// filter rather than a dedicated endpoint (alrayyes/hush-hush#252).
export async function listObjects(usedBy?: string): Promise<ObjectMetadata[]> {
	const qs = usedBy ? `?used_by=${encodeURIComponent(usedBy)}` : '';
	const res = await request(`/objects${qs}`);

	return res.json();
}

// listConsumers returns every distinct used_by consumer name already
// recorded across every object - the create/edit form's combobox offers
// these instead of relying on free-text recall (alrayyes/hush-hush#251).
export async function listConsumers(): Promise<string[]> {
	const res = await request('/consumers');

	return res.json();
}

export interface ConsumerEntry {
	name: string;
	secret_count: number;
}

export interface ConsumersPage {
	consumers: ConsumerEntry[];
	total: number;
}

export interface ConsumersQuery {
	q?: string;
	page: number;
	page_size: number;
}

// listConsumersPage fetches one page of the consumer directory, each
// entry carrying its secret count and the total matching count for
// page-number navigation - GET /consumers's paginated response shape,
// returned because this always sends page/page_size
// (alrayyes/hush-hush#252's own compatibility requirement: the
// unfiltered, unpaginated array above is what a call with none of
// q/page/page_size still gets back).
export async function listConsumersPage(
	query: ConsumersQuery,
): Promise<ConsumersPage> {
	const params = new URLSearchParams();
	if (query.q) {
		params.set('q', query.q);
	}
	params.set('page', String(query.page));
	params.set('page_size', String(query.page_size));

	const res = await request(`/consumers?${params.toString()}`);

	return res.json();
}

// renameConsumer and deleteConsumer send name (and, for a rename, newName)
// unencoded in the path - api/openapi.yaml's consumerName parameter
// deliberately isn't URL-safe (a consumer name routinely contains "/",
// e.g. homelab/vps-docker) and documents matching everything after
// /consumers/ verbatim, so there's no %2F-escaping for this client to get
// right or wrong either.

export async function renameConsumer(
	name: string,
	newName: string,
): Promise<ConsumerEntry> {
	const res = await request(`/consumers/${name}`, {
		method: 'PATCH',
		body: JSON.stringify({ name: newName }),
	});

	return res.json();
}

export async function deleteConsumer(name: string): Promise<void> {
	await request(`/consumers/${name}`, { method: 'DELETE' });
}

// addConsumer adds name to the directory with no secret referencing it
// yet (alrayyes/hush-hush#324) - POST /consumers, distinct from every
// other consumer above which only ever exists because some object's
// used_by recorded it. Rejected with a 409 ApiError if name is already
// in the directory.
export async function addConsumer(name: string): Promise<ConsumerEntry> {
	const res = await request('/consumers', {
		method: 'POST',
		body: JSON.stringify({ name }),
	});

	return res.json();
}

export async function getObjectValue(id: string): Promise<string> {
	const res = await request(`/objects/${encodeURIComponent(id)}`);
	const bytes = new Uint8Array(await res.arrayBuffer());

	// Chunked rather than String.fromCharCode(...bytes): spreading a large
	// typed array as call arguments risks "Maximum call stack size
	// exceeded", and a sealed value has no size limit this client can
	// assume.
	let binary = '';
	const chunkSize = 0x8000;
	for (let i = 0; i < bytes.length; i += chunkSize) {
		binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
	}

	return btoa(binary);
}

export interface CreateObjectRequest {
	id: string;
	value: string;
	description?: string;
	used_by?: string[];
}

export async function createObject(
	req: CreateObjectRequest,
): Promise<ObjectMetadata> {
	const res = await request('/objects', {
		method: 'POST',
		body: JSON.stringify(req),
	});

	return res.json();
}

export async function updateObject(
	id: string,
	value: string,
	usedBy?: string[],
): Promise<ObjectMetadata> {
	const res = await request(`/objects/${encodeURIComponent(id)}`, {
		method: 'PUT',
		// usedBy is omitted entirely (rather than sent as []) when the
		// caller doesn't pass it - api/openapi.yaml's UpdateObjectRequest
		// treats an absent used_by as "leave it as it is" and an empty
		// array as "clear it", so those two have to stay distinguishable
		// on the wire.
		body: JSON.stringify(
			usedBy === undefined ? { value } : { value, used_by: usedBy },
		),
	});

	return res.json();
}

export async function deleteObject(id: string): Promise<void> {
	await request(`/objects/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export interface AuditLogEntry {
	id: number;
	object_id: string;
	action: 'create' | 'read' | 'update' | 'delete';
	timestamp: string;
	caller?: string;
	ip: string;
	actor_type?: 'token' | 'session';
	actor_id?: string;
}

export interface AuditLogQuery {
	object_id?: string;
	actor?: string;
	caller?: string;
	from?: string;
	to?: string;
	after?: number;
	limit?: number;
}

// queryAuditLog fetches one page of the audit log - unfiltered and
// unpaginated (the whole log up to /audit-log's own default limit) is
// enough for the secrets overview's own per-object attribution lookup;
// the dedicated audit log page (alrayyes/hush-hush#215) passes real
// filters and walks pages via after/limit (design.md's "Audit log UI"
// decision - id-based cursor, not offset).
export async function queryAuditLog(
	query: AuditLogQuery = {},
): Promise<AuditLogEntry[]> {
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(query)) {
		if (value !== undefined && value !== '') {
			params.set(key, String(value));
		}
	}

	const qs = params.toString();
	const res = await request(qs ? `/audit-log?${qs}` : '/audit-log');

	return res.json();
}

export interface Credential {
	id: string;
	nickname?: string;
	created_at: string;
	last_used_at?: string;
}

export async function listCredentials(): Promise<Credential[]> {
	const res = await request('/credentials');

	return res.json();
}

export async function renameCredential(
	id: string,
	nickname: string,
): Promise<Credential> {
	const res = await request(`/credentials/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		body: JSON.stringify({ nickname }),
	});

	return res.json();
}

export async function deleteCredential(id: string): Promise<void> {
	await request(`/credentials/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export interface TokenMetadata {
	id: string;
	description: string;
	owner?: string;
	created_at: string;
	expires_at: string;
	revoked: boolean;
	last_used_at?: string;
}

export interface TokenWithValue extends TokenMetadata {
	value: string;
}

export async function listTokens(): Promise<TokenMetadata[]> {
	const res = await request('/tokens');

	return res.json();
}

export async function createToken(
	description: string,
	ttlSeconds: number,
): Promise<TokenWithValue> {
	const res = await request('/tokens', {
		method: 'POST',
		body: JSON.stringify({ description, ttl_seconds: ttlSeconds }),
	});

	return res.json();
}

export async function revokeToken(id: string): Promise<void> {
	await request(`/tokens/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export interface Health {
	status: string;
	version: string;
}

// getHealth is unauthenticated, same as every page's own footer that
// calls it (web-ui/spec.md's "Footer content is present on every page"
// requirement covers login too, which holds no session yet).
export async function getHealth(): Promise<Health> {
	const res = await request('/healthz');

	return res.json();
}
