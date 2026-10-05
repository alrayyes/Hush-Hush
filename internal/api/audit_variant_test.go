package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// An audit entry says which variant of a name it was about
// (alrayyes/hush-hush#684), so a read of consumer_d's release_token isn't
// indistinguishable from a read of consumer_a's.

func auditEntries(t *testing.T, f variantFixture) []hushhush.AuditLogEntry {
	t.Helper()

	rec := f.do(t, http.MethodGet, "/audit-log?object_id=release_token&limit=500", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got []hushhush.AuditLogEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	return got
}

func variantsByAction(entries []hushhush.AuditLogEntry, action string) []string {
	var ids []string

	for _, e := range entries {
		if e.Action == action {
			ids = append(ids, e.VariantID)
		}
	}

	return ids
}

func TestEveryActionOnAVariantRecordsItsID(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	require.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/objects/release_token", f.consumerToken(t, "a"), nil).Code)
	require.Equal(t, http.StatusOK, f.do(t, http.MethodGet, "/objects/release_token", f.consumerToken(t, "d"), nil).Code)

	put, err := json.Marshal(hushhush.UpdateObjectRequest{Value: sealedFixture})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, f.do(t, http.MethodPut, "/objects/release_token?id="+f.idD, f.write, put).Code)
	require.Equal(t, http.StatusNoContent, f.do(t, http.MethodDelete, "/objects/release_token?id="+f.idD, f.write, nil).Code)

	entries := auditEntries(t, f)

	require.Equal(t, []string{f.idABC, f.idD}, variantsByAction(entries, "create"))
	require.Equal(t, []string{f.idABC, f.idD}, variantsByAction(entries, "read"), "each read names the variant that consumer got")
	require.Equal(t, []string{f.idD}, variantsByAction(entries, "update"))
	require.Equal(t, []string{f.idD}, variantsByAction(entries, "delete"))
}

func TestMCPToolsRecordTheVariantID(t *testing.T) {
	t.Parallel()

	f := newVariantFixture(t)

	require.False(t, callTool(t, f.mux, f.write, "get", map[string]any{"slug": "release_token", "id": f.idD}).IsError)
	require.False(t, callTool(t, f.mux, f.write, "update", map[string]any{"slug": "release_token", "id": f.idD, "value": sealedFixture}).IsError)
	require.False(t, callTool(t, f.mux, f.write, "delete", map[string]any{"slug": "release_token", "id": f.idD}).IsError)

	entries := auditEntries(t, f)

	for _, action := range []string{"read", "update", "delete"} {
		require.Equal(t, []string{f.idD}, variantsByAction(entries, action), action)
	}
}

func TestSpecDocumentsTheAuditEntryVariantID(t *testing.T) {
	t.Parallel()

	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)

	prop := doc.Components.Schemas["AuditLogEntry"].Value.Properties["variant_id"]
	require.NotNil(t, prop, "AuditLogEntry carries the variant's uuid")
	require.Equal(t, "uuid", prop.Value.Format)
	require.EqualValues(t, 36, *prop.Value.MaxLength)

	require.NotContains(t, doc.Components.Schemas["AuditLogEntry"].Value.Required, "variant_id",
		"entries from before the column existed have none")
}
