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
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    object_id TEXT NOT NULL,
    action TEXT NOT NULL,
    caller TEXT,
    timestamp TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_log_object_id ON audit_log (object_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_caller ON audit_log (caller);
CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log (timestamp);
