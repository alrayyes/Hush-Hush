package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Actor is who performed an audited write: Type is the audit log's own
// actor_type ("session", "token" or "consumer_token") and ID its actor_id. The zero value
// means unknown.
type Actor struct {
	Type string
	ID   string
}

// parseStoredTime reads a timestamp this package wrote itself (RFC 3339,
// UTC). An unparseable value yields the zero time rather than failing a
// whole listing over one bad row.
func parseStoredTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}

	return t
}

// hydrateListed fills the parts of a listed object that live in other
// tables: its consumers, its tags and who created and last updated it.
func (s *Store) hydrateListed(ctx context.Context, obj *Object) error {
	usedBy, err := s.usedByFor(ctx, obj.ID)
	if err != nil {
		return err
	}

	obj.UsedBy = usedBy

	tags, err := s.tagsFor(ctx, obj.ID)
	if err != nil {
		return err
	}

	obj.Tags = tags

	obj.CreatedBy, obj.UpdatedBy, err = s.attributionFor(ctx, obj.Slug)

	return err
}

// attributionFor reports who created and who last updated the object now
// held under slug, from the audit log. The slug is reusable after a delete,
// so only the newest create event counts, and only updates after it. With
// no update since, the last updater is the creator. An object with no
// audit entry at all (one from before auditing) has no known actor.
func (s *Store) attributionFor(ctx context.Context, slug string) (created, updated Actor, err error) {
	createID, created, err := s.latestActor(ctx, slug, AuditActionCreate, 0)
	if err != nil {
		return Actor{}, Actor{}, err
	}

	_, updated, err = s.latestActor(ctx, slug, AuditActionUpdate, createID)
	if err != nil {
		return Actor{}, Actor{}, err
	}

	if updated == (Actor{}) {
		updated = created
	}

	return created, updated, nil
}

// latestActor returns the actor of the newest audit entry for slug with
// this action and an id above afterID, with that entry's id. Zero values
// when there is none.
func (s *Store) latestActor(ctx context.Context, slug string, action AuditAction, afterID int64) (int64, Actor, error) {
	var (
		id                 int64
		actorType, actorID sql.NullString
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT id, actor_type, actor_id FROM audit_log
		 WHERE object_id = ? AND action = ? AND id > ?
		 ORDER BY id DESC LIMIT 1`,
		slug, string(action), afterID,
	).Scan(&id, &actorType, &actorID)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return afterID, Actor{}, nil
	case err != nil:
		return 0, Actor{}, fmt.Errorf("select audit actor: %w", err)
	}

	return id, Actor{Type: actorType.String, ID: actorID.String}, nil
}
