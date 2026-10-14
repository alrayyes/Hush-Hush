package api_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMCPToolsThatTakeASlugRefuseOneOverTheLimit(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)
	long := strings.Repeat("a", 129)

	for _, tool := range []string{"get", "delete", "used_by"} {
		// Sequential: the subtests share one store, and each authenticated call writes.
		t.Run(tool, func(t *testing.T) {
			result := callTool(t, mux, token, tool, map[string]any{"slug": long})
			require.True(t, result.IsError)
			require.Contains(t, result.Content[0].Text, "128")
		})
	}
}
