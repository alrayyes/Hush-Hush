package api_test

import (
	"strings"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// The spec says what the server enforces (alrayyes/hush-hush#617), and a test
// pins the two together so one can't move without the other, the way
// TestSpecStatesTheSameTTLLimitTheHandlersEnforce does for the token lifetime.

func TestSpecDocumentsA413OnEveryOperationThatTakesABody(t *testing.T) {
	t.Parallel()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	checked := 0

	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			// The two begin routes declare no body and ignore one, but the
			// server still refuses a big one, so they answer 413 as well.
			if op.RequestBody == nil && !strings.HasSuffix(path, "/begin") {
				continue
			}

			checked++

			require.NotNil(t, op.Responses.Value("413"), "%s %s can be sent a body, so it can answer 413", method, path)
		}
	}

	require.Positive(t, checked)
}

func TestSpecStatesTheSameValueLimitTheHandlersEnforce(t *testing.T) {
	t.Parallel()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	// base64 turns every 3 bytes into 4 characters, padded.
	wantMaxLength := uint64((hushhush.MaxValueBytes + 2) / 3 * 4)

	for _, name := range []string{"CreateObjectRequest", "UpdateObjectRequest"} {
		value := doc.Components.Schemas[name].Value.Properties["value"]
		require.NotNil(t, value.Value.MaxLength, "%s.value needs a maxLength", name)
		require.Equal(t, wantMaxLength, *value.Value.MaxLength, name)
	}
}
