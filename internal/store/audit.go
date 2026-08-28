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
// caller's presented identity, if any."
//
// There is deliberately no update or delete method alongside this one -
// the audit-log spec requires entries be immutable once recorded, and the
// simplest way to guarantee that is to never write the code that would
// violate it.
func (s *Store) RecordAuditLog(ctx context.Context, objectID string, action AuditAction, caller string) error {
	var callerValue sql.NullString
	if caller != "" {
		callerValue = sql.NullString{String: caller, Valid: true}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (object_id, action, caller, timestamp) VALUES (?, ?, ?, ?)`,
		objectID, string(action), callerValue, now,
	); err != nil {
		return fmt.Errorf("record audit log: %w", err)
	}

	return nil
}

// AuditLogEntry is one recorded audit log entry. Matches
// components.schemas.AuditLogEntry in api/openapi.yaml.
type AuditLogEntry struct {
	ObjectID  string
	Action    AuditAction
	Timestamp string
	Caller    string
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

	query := `SELECT object_id, action, caller, timestamp FROM audit_log`
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

	var entries []AuditLogEntry
	for rows.Next() {
		var (
			e      AuditLogEntry
			action string
			caller sql.NullString
		)

		if err := rows.Scan(&e.ObjectID, &action, &caller, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan audit log entry: %w", err)
		}

		e.Action = AuditAction(action)
		e.Caller = caller.String
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit log: %w", err)
	}

	return entries, nil
}
