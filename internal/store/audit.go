package store

import (
	"context"
	"database/sql"
	"fmt"
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
