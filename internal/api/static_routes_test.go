package api

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// alrayyes/hush-hush#789: pageRoutes is the server's list of client routes, and
// a page added under src/routes without a line there would answer 404. This
// derives the list from the source tree the way SvelteKit does, so the two
// can't drift.
func TestPageRoutesMatchTheWebUIRoutes(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "cmd", "hush-hush", "web", "src", "routes")

	var want []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "+page.svelte" {
			return err
		}

		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}

		want = append(want, svelteRoute(rel))

		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, want, "found no pages under %s", root)

	got := slices.Clone(pageRoutes)
	slices.Sort(got)
	slices.Sort(want)
	require.Equal(t, want, got)
}

// svelteRoute turns a directory under src/routes into a route pattern: a
// (group) adds no segment and [param] matches one segment, written {param}.
func svelteRoute(dir string) string {
	var segments []string

	for part := range strings.SplitSeq(filepath.ToSlash(dir), "/") {
		switch {
		case part == ".", part == "", strings.HasPrefix(part, "("):
		case strings.HasPrefix(part, "["):
			segments = append(segments, "{"+strings.Trim(part, "[]")+"}")
		default:
			segments = append(segments, part)
		}
	}

	return "/" + strings.Join(segments, "/")
}
