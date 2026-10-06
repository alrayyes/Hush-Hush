// Thin wrapper over fetch for the Go API - same origin, no base URL, per
// design.md's "SvelteKit + adapter-static" decision ("every data access
// goes through the Go API via fetch"). Every state-changing call reads
// the session's CSRF token from the readable csrf_token cookie and
// echoes it back, matching auth/spec.md's double-submit requirement.

import { CONSUMERS_PAGE_SIZE_MAX, PAGE_LIMIT_MAX } from './api-limits';
import type { components } from './api-schema';
import { bytesToBase64 } from './encoding';

type Schemas = components['schemas'];

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
		// One row is enough to prove the session; the list itself isn't wanted.
		await request('/credentials?limit=1');

		return true;
	} catch (err) {
		if (err instanceof ApiError && err.status === 401) {
			return false;
		}

		throw err;
	}
}

export type ObjectMetadata = Schemas['ObjectMetadata'];

export type Actor = Schemas['Actor'];

// listAllPages reads every row of a paged list endpoint: GET /objects,
// /tokens, /consumer-tokens and /credentials take limit and offset and say
// how many rows exist in X-Total-Count (alrayyes/hush-hush#649). The UI
// filters and searches what it has loaded, so it asks for pages of the
// spec's own cap until it has them all, rather than rely on the default
// page size staying what it is (#662). An empty page ends the loop whatever
// the total claims, and so does a short one when no total came back.
async function listAllPages<T>(path: string, query = ''): Promise<T[]> {
	const rows: T[] = [];

	for (;;) {
		const params = `${query ? `${query}&` : ''}limit=${PAGE_LIMIT_MAX}&offset=${rows.length}`;
		const res = await request(`${path}?${params}`);
		const page = (await res.json()) as T[];
		const total = Number(res.headers.get('X-Total-Count'));
		rows.push(...page);

		if (page.length === 0) break;
		if (Number.isFinite(total) && total > 0 && rows.length >= total) break;
		if (!total && page.length < PAGE_LIMIT_MAX) break;
	}

	return rows;
}

// listObjects returns every stored object's metadata, or only those whose
// recorded used_by lineage includes usedBy when given - the consumers
// directory page's "select a consumer" navigation reuses this same
// filter rather than a dedicated endpoint (alrayyes/hush-hush#252).
export async function listObjects(usedBy?: string): Promise<ObjectMetadata[]> {
	const filter = usedBy ? `used_by=${encodeURIComponent(usedBy)}` : '';

	return listAllPages<ObjectMetadata>('/objects', filter);
}

// listConsumers returns every distinct used_by consumer name already
// recorded across every object - the create/edit form's combobox offers
// these instead of relying on free-text recall (alrayyes/hush-hush#251).
export async function listConsumers(): Promise<string[]> {
	const res = await request('/consumers');

	return res.json();
}

export type ConsumerEntry = Schemas['ConsumerEntry'];

export type ConsumersPage = Schemas['ConsumersPage'];

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
// looping past GET /consumers's own page_size cap if the directory
// is bigger than that. The create/edit form's ConsumerCombobox uses this
// (rather than listConsumers's plain name array) to resolve each
// selected consumer into a real age sealing recipient
// (specs/consumers/spec.md's "Secret form offers existing consumers and
// accepts a new one" requirement).
export async function listConsumerDirectory(): Promise<ConsumerEntry[]> {
	const pageSize = CONSUMERS_PAGE_SIZE_MAX;
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

// objectPath addresses one object. A name can hold several variants
// (alrayyes/hush-hush#668, ADR 33), and a session has to name the one it
// means with ?id= or the API answers 409; a name with a single variant
// works without it.
function objectPath(slug: string, id?: string): string {
	const path = `/objects/${encodeURIComponent(slug)}`;

	return id ? `${path}?id=${encodeURIComponent(id)}` : path;
}

export async function getObjectValue(
	slug: string,
	id?: string,
): Promise<string> {
	const res = await request(objectPath(slug, id));
	const bytes = new Uint8Array(await res.arrayBuffer());

	return bytesToBase64(bytes);
}

export type CreateObjectRequest = Schemas['CreateObjectRequest'];

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
	tags?: string[],
	id?: string,
): Promise<ObjectMetadata> {
	const res = await request(objectPath(slug, id), {
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
			// Same rule as used_by: absent leaves the tags alone, [] clears them.
			...(tags === undefined ? {} : { tags }),
			keep_readable_copy: keepReadableCopy ?? false,
		}),
	});

	return res.json();
}

export async function deleteObject(slug: string, id?: string): Promise<void> {
	await request(objectPath(slug, id), { method: 'DELETE' });
}

export type AuditLogEntry = Schemas['AuditLogEntry'];

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

export type AuditActorOption = Schemas['AuditActorOption'];

export type AuditLogFilterOptions = Schemas['AuditLogFilterOptions'];

// queryAuditLogFilterOptions fetches the distinct object ids, actors, and
// callers that actually appear in the audit log - what backs the audit
// log page's own object-id and actor/caller filter select boxes
// (alrayyes/hush-hush#323), rather than a free-text guess.
export async function queryAuditLogFilterOptions(): Promise<AuditLogFilterOptions> {
	const res = await request('/audit-log/filter-options');

	return res.json();
}

export type Credential = Schemas['Credential'];

export async function listCredentials(): Promise<Credential[]> {
	return listAllPages<Credential>('/credentials');
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

// What a token is right now and what may be done to it, by the server's own
// clock (alrayyes/hush-hush#536). Optional in the schema, always sent by this
// server. Hiding a button is cosmetic: the endpoints still enforce the rule
// and a purge of an active token is a 409.
export type TokenStatus = Schemas['TokenStatus'];
export type TokenAction = 'rotate' | 'revoke' | 'purge';

export type TokenMetadata = Schemas['TokenMetadata'];

export type TokenWithValue = Schemas['TokenWithValue'];

export async function listTokens(): Promise<TokenMetadata[]> {
	return listAllPages<TokenMetadata>('/tokens');
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

export type ConsumerTokenMetadata = Schemas['ConsumerTokenMetadata'];

export type ConsumerTokenWithValue = Schemas['ConsumerTokenWithValue'];

export async function listConsumerTokens(): Promise<ConsumerTokenMetadata[]> {
	return listAllPages<ConsumerTokenMetadata>('/consumer-tokens');
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

export type Health = Schemas['Health'];

// getHealth is unauthenticated, same as every page's own footer that
// calls it (web-ui/spec.md's "Footer content is present on every page"
// requirement covers login too, which holds no session yet).
export async function getHealth(): Promise<Health> {
	const res = await request('/healthz');

	return res.json();
}
