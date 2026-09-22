package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// AuditAction is one of the four actions the audit log records. Matches
// components.schemas.AuditLogEntry's action enum in api/openapi.yaml.
type AuditAction string

// The four actions the audit log records - matches
// components.schemas.AuditLogEntry's action enum in api/openapi.yaml.
const (
	AuditActionCreate AuditAction = "create"
	AuditActionRead   AuditAction = "read"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
)

// RecordAuditLog appends an entry to the audit log. caller may be empty,
// recorded as NULL rather than an empty string, matching the spec's "the
// caller's presented identity, if any." ip is the request's source
// address - unlike caller, always present for a real request. actorType
// and actorID are the verified credential that authenticated the call
// ("token"/a token id, "session"/the admin account, or both empty for an
// unauthenticated read) - kept separate from caller, which stays
// self-reported and unverified (openspec/changes/web-ui/design.md's
// "Audit log actor" decision).
//
// There is deliberately no update or delete method alongside this one -
// the audit-log spec requires entries be immutable once recorded, and the
// simplest way to guarantee that is to never write the code that would
// violate it.
func (s *Store) RecordAuditLog(ctx context.Context, objectID string, action AuditAction, caller, ip, actorType, actorID string) error {
	callerValue := nullableString(caller)
	actorTypeValue := nullableString(actorType)
	actorIDValue := nullableString(actorID)

	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (object_id, action, caller, ip, timestamp, actor_type, actor_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		objectID, string(action), callerValue, ip, now, actorTypeValue, actorIDValue,
	); err != nil {
		return fmt.Errorf("record audit log: %w", err)
	}

	return nil
}

func nullableString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: v, Valid: true}
}

// AuditLogEntry is one recorded audit log entry. Matches
// components.schemas.AuditLogEntry in api/openapi.yaml.
type AuditLogEntry struct {
	ID        int64
	ObjectID  string
	Action    AuditAction
	Timestamp string
	Caller    string
	IP        string
	ActorType string
	ActorID   string
}

// actorFilterNone is AuditLogFilter.Actor's sentinel value matching an
// unauthenticated read (actor_type IS NULL) - no real actor_id ever
// takes this value, since a session's is always "admin" and a token's
// is a generated id, so this doesn't collide with filtering by an actual
// actor (openspec/changes/web-ui-shadcn/design.md's "Audit log filter
// options" decision).
const actorFilterNone = "none"

// AuditLogFilter narrows a QueryAuditLog call. A zero-value field means
// that filter is unset; every set field combines with AND, per
// api/openapi.yaml's queryAuditLog description. After and Limit are the
// cursor-pagination pair (design.md's "Audit log UI" decision) - After
// is the previous page's last entry's own ID, and a zero Limit means no
// limit at all rather than some implicit default, since a direct store
// caller (a test, a future CLI command) may genuinely want everything.
type AuditLogFilter struct {
	ObjectID string
	Caller   string
	// Actor matches actor_id exactly, except for the sentinel "none"
	// (actorFilterNone), which matches an unauthenticated read
	// (actor_type IS NULL) instead.
	Actor string
	From  time.Time
	To    time.Time
	After int64
	Limit int
}

// QueryAuditLog returns matching entries, oldest first.
func (s *Store) QueryAuditLog(ctx context.Context, filter AuditLogFilter) ([]AuditLogEntry, error) {
	query, args := buildAuditLogQuery(filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanAuditLogRows(rows)
}

// buildAuditLogQuery assembles QueryAuditLog's SQL and its parameterized
// arguments - split out of QueryAuditLog itself only to stay under
// golangci-lint's funlen limit.
func buildAuditLogQuery(filter AuditLogFilter) (string, []any) {
	var (
		clauses []string
		args    []any
	)

	if filter.ObjectID != "" {
		clauses = append(clauses, "object_id = ?")
		args = append(args, filter.ObjectID)
	}

	if filter.Caller != "" {
		clauses = append(clauses, "caller = ?")
		args = append(args, filter.Caller)
	}

	switch filter.Actor {
	case "":
	case actorFilterNone:
		clauses = append(clauses, "actor_type IS NULL")
	default:
		clauses = append(clauses, "actor_id = ?")
		args = append(args, filter.Actor)
	}

	if !filter.From.IsZero() {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, filter.From.UTC().Format(time.RFC3339))
	}

	if !filter.To.IsZero() {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, filter.To.UTC().Format(time.RFC3339))
	}

	if filter.After != 0 {
		clauses = append(clauses, "id > ?")
		args = append(args, filter.After)
	}

	query := `SELECT id, object_id, action, caller, ip, timestamp, actor_type, actor_id FROM audit_log`
	if len(clauses) > 0 {
		// clauses are fixed strings from this function alone ("object_id
		// = ?" and the like) - every actual value travels through args
		// and a placeholder, never through this concatenation.
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	query += " ORDER BY id"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	return query, args
}

// scanAuditLogRows drains rows into entries - split out of QueryAuditLog
// itself only to stay under golangci-lint's funlen limit, not because the
// scanning loop is reused anywhere.
func scanAuditLogRows(rows *sql.Rows) ([]AuditLogEntry, error) {
	var entries []AuditLogEntry
	for rows.Next() {
		var (
			e                          AuditLogEntry
			action                     string
			caller, actorType, actorID sql.NullString
		)

		if err := rows.Scan(&e.ID, &e.ObjectID, &action, &caller, &e.IP, &e.Timestamp, &actorType, &actorID); err != nil {
			return nil, fmt.Errorf("scan audit log entry: %w", err)
		}

		e.Action = AuditAction(action)
		e.Caller = caller.String
		e.ActorType = actorType.String
		e.ActorID = actorID.String
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit log: %w", err)
	}

	return entries, nil
}

// AuditActorOption is one selectable value for the actor filter select
// box: Value is what a later QueryAuditLog call's AuditLogFilter.Actor
// field expects back, and Label is what the audit log page's own
// auditActorLabel already renders that actor as.
type AuditActorOption struct {
	Value string
	Label string
}

// AuditLogFilterOptions is the distinct set of values GET /audit-log's
// own filter selects offer, computed from what actually appears in
// audit_log - not from GET /objects's currently-existing objects, since
// an audit entry outlives the object it's about by design (design.md's
// "Audit log filter options" decision).
type AuditLogFilterOptions struct {
	ObjectIDs []string
	Actors    []AuditActorOption
	Callers   []string
}

// QueryAuditLogFilterOptions returns every distinct object id, actor,
// and caller currently recorded in the audit log, sorted.
func (s *Store) QueryAuditLogFilterOptions(ctx context.Context) (AuditLogFilterOptions, error) {
	objectIDs, err := queryDistinctColumn(ctx, s.db, `SELECT DISTINCT object_id FROM audit_log ORDER BY object_id`)
	if err != nil {
		return AuditLogFilterOptions{}, fmt.Errorf("query distinct object ids: %w", err)
	}

	callers, err := queryDistinctColumn(ctx, s.db,
		`SELECT DISTINCT caller FROM audit_log WHERE caller IS NOT NULL ORDER BY caller`)
	if err != nil {
		return AuditLogFilterOptions{}, fmt.Errorf("query distinct callers: %w", err)
	}

	actors, err := queryDistinctActors(ctx, s.db)
	if err != nil {
		return AuditLogFilterOptions{}, fmt.Errorf("query distinct actors: %w", err)
	}

	return AuditLogFilterOptions{ObjectIDs: objectIDs, Actors: actors, Callers: callers}, nil
}

// queryDistinctColumn runs query (expected to select exactly one text
// column) and returns its rows as a plain slice.
func queryDistinctColumn(ctx context.Context, db *sql.DB, query string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query distinct column: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var values []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan value: %w", err)
		}

		values = append(values, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate values: %w", err)
	}

	return values, nil
}

// queryDistinctActors returns one AuditActorOption per distinct
// (actor_type, actor_id) pair recorded in the audit log, labeled the
// same way the frontend's own auditActorLabel renders them.
func queryDistinctActors(ctx context.Context, db *sql.DB) ([]AuditActorOption, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT actor_type, actor_id FROM audit_log ORDER BY actor_type, actor_id`)
	if err != nil {
		return nil, fmt.Errorf("query distinct actors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var actors []AuditActorOption
	for rows.Next() {
		var actorType, actorID sql.NullString
		if err := rows.Scan(&actorType, &actorID); err != nil {
			return nil, fmt.Errorf("scan actor: %w", err)
		}

		switch actorType.String {
		case "session":
			actors = append(actors, AuditActorOption{Value: actorID.String, Label: "admin"})
		case "token":
			actors = append(actors, AuditActorOption{Value: actorID.String, Label: "token:" + actorID.String})
		default:
			actors = append(actors, AuditActorOption{Value: actorFilterNone, Label: actorFilterNone})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate actors: %w", err)
	}

	return actors, nil
}
