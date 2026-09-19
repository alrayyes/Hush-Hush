-- Objects, used_by lineage, and the audit log are all relational access
-- patterns (openspec/changes/secrets-object-store/design.md), hence SQLite
-- over a plain key-value store. CREATE TABLE IF NOT EXISTS rather than a
-- migration framework: the schema is simple enough at v1 that idempotent
-- statements applied on every Open are sufficient, and a real migration
-- tool earns its place the day this schema actually needs to change under
-- existing data.

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

-- No account_id anywhere in this schema: there is exactly one admin
-- account (openspec/changes/web-ui/design.md's "single admin account,
-- multiple passkeys" decision), so a row's mere existence in
-- webauthn_credentials already means it belongs to that one account -
-- nothing to key it against.
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
