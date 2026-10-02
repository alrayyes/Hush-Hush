-- Objects, used_by lineage, and the audit log are all relational access
-- patterns (openspec/changes/secrets-object-store/design.md), hence SQLite
-- over a plain key-value store. CREATE TABLE IF NOT EXISTS rather than a
-- migration framework: the schema is simple enough at v1 that idempotent
-- statements applied on every Open are sufficient, and a real migration
-- tool earns its place the day this schema actually needs to change under
-- existing data.

-- owner_id (added via migrateColumns in store.go, not here) foreign-keys
-- an object to the users row that created it, backfilled for every
-- pre-existing row by store.go's backfillOwnership - see users below.
-- It's accountability metadata, not an access-control mechanism
-- (openspec/changes/client-side-encryption/design.md's "owner_id is
-- accountability metadata, not an access-control mechanism" decision):
-- this migration only backfills existing rows, nothing here yet sets it
-- on a new write.
--
-- slug (added via migrateColumns in store.go, not here, with its own
-- unique index since ALTER TABLE ADD COLUMN can't add a UNIQUE
-- constraint against a table that already has rows) is what a caller
-- addresses this object by - the URL path segment, the CLI argument, the
-- create request field - now that `id` above is an opaque internal
-- identifier a caller never sees or supplies
-- (openspec/changes/client-side-encryption/specs/secret-objects/spec.md's
-- "Internal id decoupled from user-facing slug" requirement). Splitting
-- them means a future "rename a secret's slug" operation only has to
-- update this column - used_by.object_id below keys off the stable
-- internal id, so it never needs touching for a rename.
-- store.go's migrateObjectSlugs backfills slug (and generates a fresh
-- internal id) for every pre-existing row, whose current id becomes its
-- slug verbatim.
CREATE TABLE IF NOT EXISTS objects (
    id TEXT PRIMARY KEY,
    value BLOB NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS used_by (
    object_id TEXT NOT NULL REFERENCES objects (id) ON DELETE CASCADE,
    consumer TEXT NOT NULL,
    PRIMARY KEY (object_id, consumer)
);

-- Free-form labels on an object (alrayyes/hush-hush#500), for grouping and
-- filtering the overview. Metadata only - never part of the sealed value.
CREATE TABLE IF NOT EXISTS tags (
    object_id TEXT NOT NULL REFERENCES objects (id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (object_id, tag)
);

CREATE INDEX IF NOT EXISTS tags_by_tag ON tags (tag);

-- A consumer added directly (alrayyes/hush-hush#324), before any object's
-- used_by references it - used_by.object_id is a NOT NULL foreign key,
-- so there's nowhere else to record a name with zero secrets yet.
-- Additive to used_by's own distinct consumer names, not a replacement:
-- ListConsumers/ListConsumersPage union both (openspec/changes/
-- web-ui-shadcn/design.md's "New consumers table" decision).
--
-- public_key (added via migrateColumns in store.go, not here) is a
-- consumer's registered age public key - safe to store server-side since
-- it's public (openspec/changes/client-side-encryption/design.md's
-- "Consumer public key: a plain stored field, not a directory service"
-- decision). Nullable: most consumers still have none. A consumer that
-- exists only via used_by has no row here at all until a key is
-- registered for it, at which point SetConsumerPublicKey upserts one.
CREATE TABLE IF NOT EXISTS consumers (
    name TEXT PRIMARY KEY,
    created_at TEXT NOT NULL
);

-- No foreign key to objects: an audit entry documents that an action
-- happened, and it must survive the object itself being deleted - that's
-- the whole point of an audit trail.
-- actor_type/actor_id: the verified credential (a token or session) that
-- authenticated a write, kept separate from caller (self-reported,
-- unverified) rather than overwriting it.
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    object_id TEXT NOT NULL,
    action TEXT NOT NULL,
    caller TEXT,
    ip TEXT NOT NULL DEFAULT '',
    timestamp TEXT NOT NULL,
    actor_type TEXT,
    actor_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_log_object_id ON audit_log (object_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_caller ON audit_log (caller);
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log (timestamp);

-- token_hash, never the raw token: a leaked database backup shouldn't
-- also hand out every write credential it was ever meant to protect.
-- owner is NULL for a CLI-issued token, never a guessed value. revoked_at
-- is a soft-delete: NULL while valid, set once revoked, so a revoked
-- token's own audit history stays resolvable afterward.
CREATE TABLE IF NOT EXISTS write_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    owner TEXT,
    revoked_at TEXT
);

-- Scopes a read to one consumer: GET /objects/{slug} accepts this
-- alongside a write token or session, but only for an object whose
-- used_by includes `consumer` (openspec/changes/consumer-read-tokens/
-- design.md's "New consumer_tokens table, not a consumer column bolted
-- onto write_tokens" decision). No foreign key from consumer to
-- consumers.name, matching used_by.consumer's own lack of one - a
-- consumer token is issuable before any object references that consumer
-- yet. Same shape as write_tokens otherwise: hashed, soft-deleted on
-- revoke, rotated in place.
CREATE TABLE IF NOT EXISTS consumer_tokens (
    id TEXT PRIMARY KEY,
    consumer TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    last_used_at TEXT
);

-- Each account is a real row here now, keyed by an id generated the same
-- way this package's other ids are (tokens.go's randomHex) rather than a
-- separate UUID dependency - openspec/changes/client-side-encryption/
-- design.md's "users table and owner_id/user_id foreign keys" decision.
-- Exactly one row exists today: store.go's backfillOwnership inserts it
-- once, for the pre-existing admin account, the first time Open() sees
-- an empty table. Multi-user is explicitly deferred (design.md's
-- Non-Goals) - nothing yet creates a second row.
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL
);

-- user_id (added via migrateColumns in store.go, not here - see that
-- file's own "columns added since v1" convention) foreign-keys each
-- credential to the users row it belongs to. There is still exactly one
-- user today, but that's now a real users row rather than an implicit
-- fact about this table the way it used to be - store.go's
-- backfillOwnership sets user_id for every pre-existing credential.
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id TEXT PRIMARY KEY,
    public_key BLOB NOT NULL,
    sign_count INTEGER NOT NULL DEFAULT 0,
    aaguid TEXT NOT NULL DEFAULT '',
    nickname TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_used_at TEXT
);

-- csrf_token is checked against the CSRF header on state-changing
-- requests, alongside the session cookie itself - openspec/changes/web-ui/
-- design.md's "Session storage" decision.
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    csrf_token TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

-- Short-lived, in-progress WebAuthn registration/login ceremony state -
-- the challenge a BeginRegistration/BeginLogin call issued, kept until
-- the matching Finish call arrives or it expires. `data` is the
-- go-webauthn session data blob (opaque to this schema); `kind`
-- distinguishes a registration ceremony from a login one so a stray
-- Finish call can't be replayed against the wrong kind of challenge.
CREATE TABLE IF NOT EXISTS webauthn_ceremonies (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    data BLOB NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
