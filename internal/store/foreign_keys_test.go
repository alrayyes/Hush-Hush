package store

import (
	"path/filepath"
	"testing"
)

// PRAGMA foreign_keys is per connection. With it set on one pooled
// connection only, a delete landing on another skips the ON DELETE CASCADE
// and leaves orphan used_by rows that inflate a consumer's secret count
// (alrayyes/hush-hush#800).
func TestEveryPooledConnectionEnforcesForeignKeys(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "hush.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := t.Context()

	const held = 4
	for i := range held {
		conn, err := s.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close() }()

		var on int
		if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&on); err != nil {
			t.Fatal(err)
		}
		if on != 1 {
			t.Errorf("connection %d: foreign_keys = %d, want 1", i, on)
		}
	}
}

func TestOpenDeletesOrphanUsedByRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hush.db")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`PRAGMA foreign_keys = OFF`,
		`INSERT INTO used_by (object_id, consumer) VALUES ('gone', 'alrayyes/dotfiles')`,
		`INSERT INTO tags (object_id, tag) VALUES ('gone', 'old')`,
	} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	_ = conn.Close()
	_ = s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	page, err := s.ListConsumersPage(ctx, ConsumerFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Consumers) != 0 {
		t.Errorf("consumers = %+v, want none: the orphan row should be gone", page.Consumers)
	}

	var tags int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tags`).Scan(&tags); err != nil {
		t.Fatal(err)
	}
	if tags != 0 {
		t.Errorf("tags rows = %d, want 0", tags)
	}
}
