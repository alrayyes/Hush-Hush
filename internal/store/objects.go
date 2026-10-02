package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrAlreadyExists is returned by CreateObject when an object already exists
// under the given id.
var ErrAlreadyExists = errors.New("object already exists")

// ErrNotFound is returned when no object exists under the given id.
var ErrNotFound = errors.New("object not found")

// ErrUnknownConsumer is returned by RenameConsumer and DeleteConsumer when
// no stored object's used_by list currently records the given name.
var ErrUnknownConsumer = errors.New("unknown consumer")

// ErrConsumerAlreadyExists is returned by AddConsumer when name already
// appears in the directory, whether added directly or recorded by an
// object's used_by list.
var ErrConsumerAlreadyExists = errors.New("consumer already exists")

// Object is a stored secret object: its sealed value, its recorded used_by
// lineage, and its description. The service never decrypts Value - it is
// opaque ciphertext.
//
// ID and Slug are deliberately two different things
// (specs/secret-objects/spec.md's "Internal id decoupled from user-facing
// slug" requirement). Slug is what a caller addresses this object by - the
// URL path segment, the CLI argument, the create request field - and every
// store method below takes it as its own slug parameter. ID is the opaque
// internal identifier used_by.object_id actually keys off underneath,
// informational only: never something a caller supplies, and never
// something the API layer should hand back as an address.
type Object struct {
	ID          string
	Slug        string
	Value       []byte
	UsedBy      []string
	Tags        []string
	Description string
	// CreatedAt and UpdatedAt come from the object's own row. CreatedBy and
	// UpdatedBy come from the audit log (see attributionFor) and are only
	// filled by ListObjects; the zero Actor means no audit entry says.
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy Actor
	UpdatedBy Actor
	// OwnerID is the users row that created this object, captured once at
	// creation (specs/secret-objects/spec.md's "Owner recorded from the
	// creating session" scenario) - accountability and audit metadata
	// only, per design.md's "owner_id is accountability metadata, not an
	// access-control mechanism" decision: it never makes the owner a
	// decrypt recipient by itself. Empty for a pre-existing row from
	// before this field started being set, until store.go's
	// backfillOwnership backfills it.
	OwnerID string
}

// ObjectOption sets an optional field on CreateObject or UpdateObject, so
// a new field doesn't ripple through every existing call site.
type ObjectOption func(*objectOptions)

type objectOptions struct {
	tags *[]string
}

// WithTags sets the object's tags. On UpdateObject it replaces them, and
// an empty, non-nil slice clears them; leaving the option off leaves them
// unchanged (the same nil-versus-empty split usedBy has).
func WithTags(tags []string) ObjectOption {
	if tags == nil {
		tags = []string{}
	}

	return func(o *objectOptions) { o.tags = &tags }
}

func applyObjectOptions(opts []ObjectOption) objectOptions {
	var o objectOptions
	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// CreateObject stores a new object under a freshly generated internal id,
// addressable afterward by slug, recording ownerID as its owner.
// description is fixed at creation, the same as usedBy and ownerID -
// there is no way to change any of them later (specs/secret-objects/
// spec.md). It returns ErrAlreadyExists if an object already exists under
// that slug - existence is checked and the insert performed in the same
// transaction, so this is race-safe against concurrent creates under the
// same slug.
func (s *Store) CreateObject(ctx context.Context, slug string, value []byte, usedBy []string, description, ownerID string, opts ...ObjectOption) error {
	options := applyObjectOptions(opts)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	switch err := tx.QueryRowContext(ctx, `SELECT 1 FROM objects WHERE slug = ?`, slug).Scan(&exists); {
	case err == nil:
		return ErrAlreadyExists
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("check existing object: %w", err)
	}

	id, err := randomHex(16)
	if err != nil {
		return fmt.Errorf("generate object id: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO objects (id, slug, value, description, owner_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, slug, value, description, sql.NullString{String: ownerID, Valid: ownerID != ""}, now, now,
	); err != nil {
		return fmt.Errorf("insert object: %w", err)
	}

	for _, consumer := range usedBy {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO used_by (object_id, consumer) VALUES (?, ?)`,
			id, consumer,
		); err != nil {
			return fmt.Errorf("insert used_by: %w", err)
		}
	}

	if options.tags != nil {
		if err := replaceTags(ctx, tx, id, *options.tags); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetObject fetches an object's sealed value and used_by lineage by slug.
// It returns ErrNotFound if no object exists under that slug.
func (s *Store) GetObject(ctx context.Context, slug string) (Object, error) {
	obj := Object{Slug: slug}
	var ownerID sql.NullString

	switch err := s.db.QueryRowContext(ctx, `SELECT id, value, description, owner_id FROM objects WHERE slug = ?`, slug).Scan(&obj.ID, &obj.Value, &obj.Description, &ownerID); {
	case errors.Is(err, sql.ErrNoRows):
		return Object{}, ErrNotFound
	case err != nil:
		return Object{}, fmt.Errorf("select object: %w", err)
	}
	obj.OwnerID = ownerID.String

	usedBy, err := s.usedByFor(ctx, obj.ID)
	if err != nil {
		return Object{}, err
	}
	obj.UsedBy = usedBy

	tags, err := s.tagsFor(ctx, obj.ID)
	if err != nil {
		return Object{}, err
	}
	obj.Tags = tags

	return obj, nil
}

// tagsFor returns id's tags, sorted.
func (s *Store) tagsFor(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tag FROM tags WHERE object_id = ? ORDER BY tag`, id)
	if err != nil {
		return nil, fmt.Errorf("select tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}

	return tags, nil
}

// replaceTags makes tags the whole tag set of the object with this
// internal id.
func replaceTags(ctx context.Context, tx *sql.Tx, id string, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE object_id = ?`, id); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}

	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tags (object_id, tag) VALUES (?, ?)`, id, tag); err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}

	return nil
}

// usedByFor returns id's recorded used_by lineage, consumer names sorted -
// shared by GetObject and ListObjects rather than each querying it inline.
func (s *Store) usedByFor(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT consumer FROM used_by WHERE object_id = ? ORDER BY consumer`, id)
	if err != nil {
		return nil, fmt.Errorf("select used_by: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var usedBy []string
	for rows.Next() {
		var consumer string
		if err := rows.Scan(&consumer); err != nil {
			return nil, fmt.Errorf("scan used_by: %w", err)
		}
		usedBy = append(usedBy, consumer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate used_by: %w", err)
	}

	return usedBy, nil
}

// ObjectFilter narrows a ListObjects call. The zero value matches every
// stored object.
type ObjectFilter struct {
	// UsedBy restricts the result to objects whose recorded used_by
	// lineage includes this consumer. Empty means no restriction.
	UsedBy string
	// Tags restricts the result to objects carrying every one of these
	// tags. Empty means no restriction.
	Tags []string
}

// ListObjects returns every stored object's metadata (slug, used_by,
// description - never the sealed value, and never the internal id as
// something addressable), sorted by slug. filter narrows the result; its
// zero value returns everything.
func (s *Store) ListObjects(ctx context.Context, filter ObjectFilter) ([]Object, error) {
	query := `SELECT DISTINCT o.id, o.slug, o.description, o.created_at, o.updated_at FROM objects o`

	var (
		conds []string
		args  []any
	)

	if filter.UsedBy != "" {
		query += ` JOIN used_by u ON u.object_id = o.id`

		conds = append(conds, `u.consumer = ?`)
		args = append(args, filter.UsedBy)
	}

	// One fixed-text EXISTS per tag - values only ever travel through args.
	for _, tag := range filter.Tags {
		conds = append(conds, `EXISTS (SELECT 1 FROM tags t WHERE t.object_id = o.id AND t.tag = ?)`)
		args = append(args, tag)
	}

	if len(conds) > 0 {
		query += ` WHERE ` + strings.Join(conds, ` AND `)
	}

	rows, err := s.db.QueryContext(ctx, query+` ORDER BY o.slug`, args...)
	if err != nil {
		return nil, fmt.Errorf("select objects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var objs []Object
	for rows.Next() {
		var (
			obj                  Object
			createdAt, updatedAt string
		)

		if err := rows.Scan(&obj.ID, &obj.Slug, &obj.Description, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan object: %w", err)
		}

		obj.CreatedAt = parseStoredTime(createdAt)
		obj.UpdatedAt = parseStoredTime(updatedAt)
		objs = append(objs, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate objects: %w", err)
	}

	for i := range objs {
		if err := s.hydrateListed(ctx, &objs[i]); err != nil {
			return nil, err
		}
	}

	return objs, nil
}

// UpdateObject replaces the stored value for the object addressed by slug,
// leaving description untouched - there is no way to change it after
// creation (specs/secret-objects/spec.md). usedBy is left untouched too
// when nil; given non-nil (including an empty, non-nil slice), it fully
// replaces the object's recorded used_by lineage the same way CreateObject
// populates it - the pointer is what tells "the caller didn't send
// used_by" apart from "the caller sent an empty list to clear it"
// (alrayyes/hush-hush#299). It returns ErrNotFound if no object exists
// under that slug.
func (s *Store) UpdateObject(ctx context.Context, slug string, value []byte, usedBy *[]string, opts ...ObjectOption) error {
	options := applyObjectOptions(opts)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)

	result, err := tx.ExecContext(ctx,
		`UPDATE objects SET value = ?, updated_at = ? WHERE slug = ?`,
		value, now, slug,
	)
	if err != nil {
		return fmt.Errorf("update object: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	if err := replaceObjectRelations(ctx, tx, slug, usedBy, options.tags); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// replaceObjectRelations rewrites an updated object's used_by and tags
// rows, each only when given - split out of UpdateObject to keep it
// readable.
func replaceObjectRelations(ctx context.Context, tx *sql.Tx, slug string, usedBy, tags *[]string) error {
	if usedBy != nil {
		// used_by.object_id keys off the internal id, not slug (schema.sql's
		// own comment on why) - both statements below resolve it from slug
		// via the same subquery rather than a separate round trip, since the
		// caller's UPDATE already proved a matching row exists.
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM used_by WHERE object_id = (SELECT id FROM objects WHERE slug = ?)`,
			slug,
		); err != nil {
			return fmt.Errorf("clear used_by: %w", err)
		}

		for _, consumer := range *usedBy {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO used_by (object_id, consumer) VALUES ((SELECT id FROM objects WHERE slug = ?), ?)`,
				slug, consumer,
			); err != nil {
				return fmt.Errorf("insert used_by: %w", err)
			}
		}
	}

	if tags == nil {
		return nil
	}

	var id string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM objects WHERE slug = ?`, slug).Scan(&id); err != nil {
		return fmt.Errorf("select object id: %w", err)
	}

	return replaceTags(ctx, tx, id, *tags)
}

// AddConsumer adds name to the directory with no secret referencing it yet
// (alrayyes/hush-hush#324) - used_by has no way to record a bare name
// itself (object_id is a NOT NULL foreign key), hence the separate
// consumers table. Returns ErrConsumerAlreadyExists if name already
// appears in the directory, whether added directly or recorded by an
// object's used_by list.
func (s *Store) AddConsumer(ctx context.Context, name string) error {
	var exists int
	if err := s.db.QueryRowContext(ctx, `
		SELECT 1 WHERE EXISTS (SELECT 1 FROM consumers WHERE name = ?)
			OR EXISTS (SELECT 1 FROM used_by WHERE consumer = ?)`,
		name, name,
	).Scan(&exists); err == nil {
		return ErrConsumerAlreadyExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check existing consumer: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO consumers (name, created_at) VALUES (?, ?)`, name, now,
	); err != nil {
		return fmt.Errorf("insert consumer: %w", err)
	}

	return nil
}

// ListConsumers returns every distinct consumer name currently present in
// any object's used_by list or added directly via AddConsumer, sorted,
// with no duplicates - the secret create/edit form offers these instead
// of relying on free-text recall (alrayyes/hush-hush#251).
func (s *Store) ListConsumers(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT consumer FROM used_by
		UNION
		SELECT name FROM consumers
		ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("list consumers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var consumers []string
	for rows.Next() {
		var consumer string
		if err := rows.Scan(&consumer); err != nil {
			return nil, fmt.Errorf("scan consumer: %w", err)
		}

		consumers = append(consumers, consumer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate consumers: %w", err)
	}

	return consumers, nil
}

// ConsumerFilter narrows a ListConsumersPage call.
type ConsumerFilter struct {
	// Name restricts results to consumers whose name contains this
	// substring, case-insensitive. Empty means no restriction.
	Name string
	// Page is the 1-based page number and PageSize the maximum number of
	// consumers per page. Both must be positive.
	Page     int
	PageSize int
}

// ConsumerEntry is one consumer returned by ListConsumersPage: its name,
// how many stored secret objects record it in their used_by list, and its
// registered age public key (empty if none has been set -
// openspec/changes/client-side-encryption/specs/consumers/spec.md's
// "Consumer public key registration" requirement).
type ConsumerEntry struct {
	Name        string
	SecretCount int
	PublicKey   string
}

// ConsumerPage is one page of ListConsumersPage's filtered result, plus
// the total count of matching consumers across every page - what a
// caller needs to render page-number navigation.
type ConsumerPage struct {
	Consumers []ConsumerEntry
	Total     int
}

// ListConsumersPage returns one page of distinct consumer names (recorded
// by an object's used_by list, added directly via AddConsumer, or both)
// whose name contains filter.Name (case-insensitive, every consumer when
// empty), each with a count of the secret objects whose used_by includes
// it - 0 for a name that only exists via AddConsumer - sorted by name.
// The consumer directory page's own listing (alrayyes/hush-hush#252),
// distinct from ListConsumers's plain, unpaginated array that
// consumer-combobox still relies on.
func (s *Store) ListConsumersPage(ctx context.Context, filter ConsumerFilter) (ConsumerPage, error) {
	pattern := "%" + escapeLike(filter.Name) + "%"

	const namesCTE = `WITH names AS (
		SELECT consumer AS name FROM used_by
		UNION
		SELECT name FROM consumers
	)`

	var total int
	if err := s.db.QueryRowContext(ctx,
		namesCTE+` SELECT COUNT(*) FROM names WHERE name LIKE ? ESCAPE '\'`,
		pattern,
	).Scan(&total); err != nil {
		return ConsumerPage{}, fmt.Errorf("count consumers: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, namesCTE+`
		SELECT n.name, COUNT(u.object_id) AS secret_count, c.public_key
		FROM names n
		LEFT JOIN used_by u ON u.consumer = n.name
		LEFT JOIN consumers c ON c.name = n.name
		WHERE n.name LIKE ? ESCAPE '\'
		GROUP BY n.name
		ORDER BY n.name
		LIMIT ? OFFSET ?`,
		pattern, filter.PageSize, (filter.Page-1)*filter.PageSize,
	)
	if err != nil {
		return ConsumerPage{}, fmt.Errorf("list consumers page: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var consumers []ConsumerEntry
	for rows.Next() {
		var entry ConsumerEntry
		var publicKey sql.NullString
		if err := rows.Scan(&entry.Name, &entry.SecretCount, &publicKey); err != nil {
			return ConsumerPage{}, fmt.Errorf("scan consumer: %w", err)
		}
		entry.PublicKey = publicKey.String

		consumers = append(consumers, entry)
	}

	if err := rows.Err(); err != nil {
		return ConsumerPage{}, fmt.Errorf("iterate consumers: %w", err)
	}

	return ConsumerPage{Consumers: consumers, Total: total}, nil
}

// RenameConsumer replaces oldName with newName in every stored object's
// used_by list that currently records oldName, returning the resulting
// entry under newName. Consumers aren't a stored resource of their own
// (alrayyes/hush-hush#282, ADR 0002) - this is a bulk rewrite across
// used_by, not a rename of a row with its own identity, except for a
// name added directly via AddConsumer, or one with a registered public
// key, which does have its own row in consumers to move.
//
// If newName already has its own recorded objects, the two merge: an
// object recording both ends up with a single used_by entry, not a
// (object_id, consumer) primary-key violation, and the returned
// SecretCount covers every object now recording newName, old and
// already-there combined. A registered public key survives the merge too:
// newName's own key wins if it already had one, otherwise oldName's key
// (if any) carries over - the surviving identity is newName, so its key
// takes precedence rather than being silently overwritten by the one
// being merged away. Renaming a name to itself is a no-op. It returns
// ErrUnknownConsumer if oldName isn't currently recorded anywhere, in
// used_by or in consumers.
func (s *Store) RenameConsumer(ctx context.Context, oldName, newName string) (ConsumerEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConsumerEntry{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	switch err := tx.QueryRowContext(ctx, `
		SELECT 1 WHERE EXISTS (SELECT 1 FROM used_by WHERE consumer = ?)
			OR EXISTS (SELECT 1 FROM consumers WHERE name = ?)`,
		oldName, oldName,
	).Scan(&exists); {
	case errors.Is(err, sql.ErrNoRows):
		return ConsumerEntry{}, ErrUnknownConsumer
	case err != nil:
		return ConsumerEntry{}, fmt.Errorf("check existing consumer: %w", err)
	}

	// Read both names' registered public keys before oldName's consumers
	// row (if any) is deleted below - otherwise a key registered under
	// oldName would be lost rather than carried over to newName.
	resolvedKey, err := resolveRenamedPublicKey(ctx, tx, oldName, newName)
	if err != nil {
		return ConsumerEntry{}, err
	}

	if oldName != newName {
		if err := rewriteUsedByForRename(ctx, tx, oldName, newName); err != nil {
			return ConsumerEntry{}, err
		}
	}

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM used_by WHERE consumer = ?`, newName).Scan(&count); err != nil {
		return ConsumerEntry{}, fmt.Errorf("count renamed consumer: %w", err)
	}

	// newName needs its own consumers row whenever it has no used_by rows
	// of its own yet (kept visible in the directory the same way
	// AddConsumer does) or there's a resolved public key to persist -
	// upserting either way rather than only inserting when missing, since
	// oldName's key (if any) has to land on newName's row even when
	// newName already has used_by rows of its own.
	if count == 0 || resolvedKey != "" {
		if err := upsertConsumerRow(ctx, tx, newName, resolvedKey); err != nil {
			return ConsumerEntry{}, fmt.Errorf("record renamed consumer: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return ConsumerEntry{}, fmt.Errorf("commit transaction: %w", err)
	}

	return ConsumerEntry{Name: newName, SecretCount: count, PublicKey: resolvedKey}, nil
}

// rewriteUsedByForRename is RenameConsumer's own used_by/consumers rewrite
// for the oldName != newName case: drop the old entry wherever the object
// already records newName (otherwise the UPDATE below would try to insert
// a second (object_id, newName) row and violate used_by's primary key),
// move every remaining used_by row from oldName to newName, then drop
// oldName's own consumers row - its public key, if any, was already read
// by resolveRenamedPublicKey before this runs.
func rewriteUsedByForRename(ctx context.Context, tx renameSQLTx, oldName, newName string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM used_by
		WHERE consumer = ? AND object_id IN (
			SELECT object_id FROM used_by WHERE consumer = ?
		)`, oldName, newName,
	); err != nil {
		return fmt.Errorf("drop merged used_by rows: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE used_by SET consumer = ? WHERE consumer = ?`, newName, oldName); err != nil {
		return fmt.Errorf("rename used_by rows: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM consumers WHERE name = ?`, oldName); err != nil {
		return fmt.Errorf("drop old consumer entry: %w", err)
	}

	return nil
}

// renameSQLTx is the subset of *sql.Tx that resolveRenamedPublicKey and
// upsertConsumerRow need - just enough to run a query or exec against the
// transaction RenameConsumer already opened, without passing the whole
// *sql.Tx type further than necessary.
type renameSQLTx interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// resolveRenamedPublicKey reads oldName's and newName's currently
// registered public keys (within tx, before RenameConsumer's own rewrite
// deletes oldName's row) and returns which one survives the rename:
// newName's own key if it has one, otherwise oldName's (empty if neither
// does).
func resolveRenamedPublicKey(ctx context.Context, tx renameSQLTx, oldName, newName string) (string, error) {
	var oldKey, newKey sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT public_key FROM consumers WHERE name = ?`, oldName).Scan(&oldKey); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read old consumer public key: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT public_key FROM consumers WHERE name = ?`, newName).Scan(&newKey); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read new consumer public key: %w", err)
	}

	if newKey.String != "" {
		return newKey.String, nil
	}

	return oldKey.String, nil
}

// upsertConsumerRow inserts a consumers row for name if none exists, or
// updates its public_key if one already does - shared by RenameConsumer
// (persisting a resolved key, or just keeping a used_by-only name visible
// in the directory) and SetConsumerPublicKey. An empty publicKey stores
// NULL, not an empty string.
func upsertConsumerRow(ctx context.Context, tx renameSQLTx, name, publicKey string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO consumers (name, created_at, public_key) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET public_key = excluded.public_key`,
		name, now, sql.NullString{String: publicKey, Valid: publicKey != ""},
	); err != nil {
		return fmt.Errorf("upsert consumer: %w", err)
	}

	return nil
}

// SetConsumerPublicKey registers or updates name's registered age public
// key, upserting a consumers row if none exists yet - a name that only
// exists via some object's used_by list has nowhere else to persist a key
// until now, and AddConsumer's own existing-name check doesn't apply
// here: registering a key for an already-referenced consumer is exactly
// the expected use of this method, not a duplicate-add attempt.
// publicKey must never be a private key - the caller (handleUpdateConsumer)
// only ever forwards what the request body calls public_key, and the
// server has no way to tell a private key apart from a public one, so
// that boundary is enforced by never asking for anything else, not by
// inspecting the value. Returns the resulting entry: name, the number of
// stored secret objects whose used_by list references it (0 for a name
// that exists only via this call), and the public key just set.
func (s *Store) SetConsumerPublicKey(ctx context.Context, name, publicKey string) (ConsumerEntry, error) {
	if err := upsertConsumerRow(ctx, s.db, name, publicKey); err != nil {
		return ConsumerEntry{}, fmt.Errorf("set consumer public key: %w", err)
	}

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM used_by WHERE consumer = ?`, name).Scan(&count); err != nil {
		return ConsumerEntry{}, fmt.Errorf("count consumer: %w", err)
	}

	return ConsumerEntry{Name: name, SecretCount: count, PublicKey: publicKey}, nil
}

// DeleteConsumer strips name from the used_by list of every stored object
// that currently records it, and removes its consumers row if it has
// one. The objects themselves aren't touched otherwise, and none are
// deleted even if this empties their used_by list. It returns
// ErrUnknownConsumer if name isn't currently recorded anywhere, in
// used_by or in consumers.
func (s *Store) DeleteConsumer(ctx context.Context, name string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	usedByResult, err := tx.ExecContext(ctx, `DELETE FROM used_by WHERE consumer = ?`, name)
	if err != nil {
		return fmt.Errorf("delete consumer from used_by: %w", err)
	}

	usedByRows, err := usedByResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("check used_by rows affected: %w", err)
	}

	consumersResult, err := tx.ExecContext(ctx, `DELETE FROM consumers WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete consumer entry: %w", err)
	}

	consumersRows, err := consumersResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("check consumers rows affected: %w", err)
	}

	if usedByRows == 0 && consumersRows == 0 {
		return ErrUnknownConsumer
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// escapeLike escapes SQLite LIKE's own wildcard characters (and the
// escape character itself) in s, so a name filter containing a literal
// "%" or "_" matches only that literal text rather than acting as a
// wildcard.
func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

	return replacer.Replace(s)
}

// DeleteObject permanently removes the object addressed by slug, its
// used_by rows cascading with it (schema.sql's ON DELETE CASCADE, keyed
// off the internal id regardless of how the row was located here). It
// returns ErrNotFound if no object exists under that slug.
func (s *Store) DeleteObject(ctx context.Context, slug string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM objects WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
