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
// components.schemas.AuditLogEntry in api/openapi.yaml - ActorType and
// ActorID aren't in that schema yet (alrayyes/hush-hush#214 adds them),
// but are already readable here since RecordAuditLog already writes them.
type AuditLogEntry struct {
	ObjectID  string
	Action    AuditAction
	Timestamp string
	Caller    string
	IP        string
	ActorType string
	ActorID   string
}

// AuditLogFilter narrows a QueryAuditLog call. A zero-value field means
// that filter is unset; every set field combines with AND, per
// api/openapi.yaml's queryAuditLog description.
type AuditLogFilter struct {
	ObjectID string
	Caller   string
	From     time.Time
	To       time.Time
}

// QueryAuditLog returns matching entries, oldest first.
func (s *Store) QueryAuditLog(ctx context.Context, filter AuditLogFilter) ([]AuditLogEntry, error) {
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

	if !filter.From.IsZero() {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, filter.From.UTC().Format(time.RFC3339))
	}

	if !filter.To.IsZero() {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, filter.To.UTC().Format(time.RFC3339))
	}

	query := `SELECT object_id, action, caller, ip, timestamp, actor_type, actor_id FROM audit_log`
	if len(clauses) > 0 {
		// clauses are fixed strings from this function alone ("object_id
		// = ?" and the like) - every actual value travels through args
		// and a placeholder, never through this concatenation.
		query += " WHERE " + strings.Join(clauses, " AND ") //nolint:gosec // clauses are static, values are parameterized
	}

	query += " ORDER BY id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanAuditLogRows(rows)
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

		if err := rows.Scan(&e.ObjectID, &action, &caller, &e.IP, &e.Timestamp, &actorType, &actorID); err != nil {
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
