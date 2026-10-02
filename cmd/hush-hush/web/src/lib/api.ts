// Thin wrapper over fetch for the Go API - same origin, no base URL, per
// design.md's "SvelteKit + adapter-static" decision ("every data access
// goes through the Go API via fetch"). Every state-changing call reads
// the session's CSRF token from the readable csrf_token cookie and
// echoes it back, matching auth/spec.md's double-submit requirement.

import { bytesToBase64 } from './encoding';

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

// RegistrationEscrow carries the escrowed writer identity fields
// identity.ts's registerPasskey flow adds on top of the raw WebAuthn
// attestation - wrapped_identity for any PRF-capable credential,
// public_key/recovery_wrapped_identity only on the account's first-ever
// registration (api/openapi.yaml's RegistrationFinishRequest schema).
export interface RegistrationEscrow {
	wrapped_identity?: string;
	public_key?: string;
	recovery_wrapped_identity?: string;
}

export async function finishRegistration(
	credential: unknown,
	nickname?: string,
	escrow?: RegistrationEscrow,
): Promise<void> {
	await request('/auth/register/finish', {
		method: 'POST',
		body: JSON.stringify({ credential, nickname, ...escrow }),
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

// getOwnerIdentity returns the calling session's own escrowed identity
// public key, or undefined if that account hasn't completed a first
// registration yet - what the create/edit dialog's owner-recipient opt-in
// checkbox offers as an additional sealing recipient when checked
// (openspec/changes/client-side-encryption/specs/secret-objects/spec.md's
// "Opt-in owner-recipient inclusion at create time" requirement).
export async function getOwnerIdentity(): Promise<string | undefined> {
	const res = await request('/auth/identity');
	const body = (await res.json()) as { public_key?: string };

	return body.public_key;
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
	slug: string;
	description?: string;
	used_by?: string[];
	// Labels for grouping and filtering, always present and empty when the
	// object has none (alrayyes/hush-hush#500).
	tags: string[];
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
	// public_key is the consumer's registered age public key, absent
	// entirely when none has been registered - api/openapi.yaml's
	// ConsumerEntry schema, extended by alrayyes/Hush-Hush#393. Safe to
	// hold client-side since it's public; the matching private key never
	// reaches this API or this client.
	public_key?: string;
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

// listConsumerDirectory returns every consumer entry - name, secret
// count, and registered public key when one is set - across every page,
// looping past GET /consumers's own page_size cap (100) if the directory
// is bigger than that. The create/edit form's ConsumerCombobox uses this
// (rather than listConsumers's plain name array) to resolve each
// selected consumer into a real age sealing recipient
// (specs/consumers/spec.md's "Secret form offers existing consumers and
// accepts a new one" requirement).
export async function listConsumerDirectory(): Promise<ConsumerEntry[]> {
	const pageSize = 100;
	const entries: ConsumerEntry[] = [];
	let page = 1;

	for (;;) {
		const result = await listConsumersPage({ page, page_size: pageSize });
		entries.push(...result.consumers);

		if (result.consumers.length === 0 || entries.length >= result.total) {
			break;
		}

		page += 1;
	}

	return entries;
}

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

export async function getObjectValue(slug: string): Promise<string> {
	const res = await request(`/objects/${encodeURIComponent(slug)}`);
	const bytes = new Uint8Array(await res.arrayBuffer());

	return bytesToBase64(bytes);
}

export interface CreateObjectRequest {
	slug: string;
	value: string;
	description?: string;
	used_by?: string[];
	// keep_readable_copy requests that the owner's own escrowed identity
	// public key be included as an additional sealing recipient -
	// api/openapi.yaml's KeepReadableCopy schema. This is a request-shape
	// field only: the server never adds the recipient itself, so the
	// caller has to have already added the owner's public key
	// (getOwnerIdentity) to value's own recipients before sealing it.
	keep_readable_copy?: boolean;
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
	slug: string,
	value: string,
	usedBy?: string[],
	keepReadableCopy?: boolean,
): Promise<ObjectMetadata> {
	const res = await request(`/objects/${encodeURIComponent(slug)}`, {
		method: 'PUT',
		// usedBy is omitted entirely (rather than sent as []) when the
		// caller doesn't pass it - api/openapi.yaml's UpdateObjectRequest
		// treats an absent used_by as "leave it as it is" and an empty
		// array as "clear it", so those two have to stay distinguishable
		// on the wire. keep_readable_copy has no such distinction to make
		// (CreateObjectRequest's own doc comment) - always sent as a plain
		// boolean.
		body: JSON.stringify({
			value,
			...(usedBy === undefined ? {} : { used_by: usedBy }),
			keep_readable_copy: keepReadableCopy ?? false,
		}),
	});

	return res.json();
}

export async function deleteObject(slug: string): Promise<void> {
	await request(`/objects/${encodeURIComponent(slug)}`, { method: 'DELETE' });
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
	// 'desc' is newest first, so with limit it returns the newest entries.
	order?: 'asc' | 'desc';
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

export interface AuditActorOption {
	value: string;
	label: string;
}

export interface AuditLogFilterOptions {
	object_ids: string[];
	actors: AuditActorOption[];
	callers: string[];
}

// queryAuditLogFilterOptions fetches the distinct object ids, actors, and
// callers that actually appear in the audit log - what backs the audit
// log page's own object-id and actor/caller filter select boxes
// (alrayyes/hush-hush#323), rather than a free-text guess.
export async function queryAuditLogFilterOptions(): Promise<AuditLogFilterOptions> {
	const res = await request('/audit-log/filter-options');

	return res.json();
}

export interface Credential {
	id: string;
	nickname?: string;
	created_at: string;
	last_used_at?: string;
	// wrapped_identity is this credential's own copy of the escrowed
	// writer identity's private key, wrapped against this credential's
	// PRF secret - absent for a credential that doesn't support PRF
	// (openspec/changes/client-side-encryption/specs/users/spec.md's
	// "Per-credential wrapping of the escrowed identity" requirement).
	wrapped_identity?: string;
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

export async function rotateToken(
	id: string,
	ttlSeconds: number,
): Promise<TokenWithValue> {
	const res = await request(`/tokens/${encodeURIComponent(id)}/rotate`, {
		method: 'POST',
		body: JSON.stringify({ ttl_seconds: ttlSeconds }),
	});

	return res.json();
}

export async function purgeToken(id: string): Promise<void> {
	await request(`/tokens/${encodeURIComponent(id)}/purge`, {
		method: 'DELETE',
	});
}

export interface ConsumerTokenMetadata {
	id: string;
	consumer: string;
	description: string;
	created_at: string;
	expires_at: string;
	revoked: boolean;
	last_used_at?: string;
}

export interface ConsumerTokenWithValue extends ConsumerTokenMetadata {
	value: string;
}

export async function listConsumerTokens(): Promise<ConsumerTokenMetadata[]> {
	const res = await request('/consumer-tokens');

	return res.json();
}

export async function createConsumerToken(
	consumer: string,
	description: string,
	ttlSeconds: number,
): Promise<ConsumerTokenWithValue> {
	const res = await request('/consumer-tokens', {
		method: 'POST',
		body: JSON.stringify({ consumer, description, ttl_seconds: ttlSeconds }),
	});

	return res.json();
}

export async function revokeConsumerToken(id: string): Promise<void> {
	await request(`/consumer-tokens/${encodeURIComponent(id)}`, {
		method: 'DELETE',
	});
}

export async function rotateConsumerToken(
	id: string,
	ttlSeconds: number,
): Promise<ConsumerTokenWithValue> {
	const res = await request(
		`/consumer-tokens/${encodeURIComponent(id)}/rotate`,
		{
			method: 'POST',
			body: JSON.stringify({ ttl_seconds: ttlSeconds }),
		},
	);

	return res.json();
}

export async function purgeConsumerToken(id: string): Promise<void> {
	await request(`/consumer-tokens/${encodeURIComponent(id)}/purge`, {
		method: 'DELETE',
	});
}

export interface Health {
	status: string;
	version: string;
	// The operator's own label for this instance (the INSTANCE_LABEL setting),
	// absent when none is set - alrayyes/hush-hush#512.
	environment?: string;
}

// getHealth is unauthenticated, same as every page's own footer that
// calls it (web-ui/spec.md's "Footer content is present on every page"
// requirement covers login too, which holds no session yet).
export async function getHealth(): Promise<Health> {
	const res = await request('/healthz');

	return res.json();
}
