package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

func TestHealthAnswersOK(t *testing.T) {
	mux := hushhush.NewMux(nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body hushhush.Health
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "ok", body.Status)
}

func TestGetWidget(t *testing.T) {
	widgets := map[string]hushhush.Widget{
		"hammer": {ID: "hammer", Name: "Claw hammer"},
	}
	mux := hushhush.NewMux(widgets)

	t.Run("known id returns the widget", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/widgets/hammer", nil)

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var body hushhush.Widget
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, hushhush.Widget{ID: "hammer", Name: "Claw hammer"}, body)
	})

	t.Run("unknown id returns 404 with an error body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/widgets/nope", nil)

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)

		var body hushhush.Error
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "unknown widget", body.Error)
	})
}
