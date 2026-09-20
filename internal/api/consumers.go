package api

import "net/http"

// handleListConsumers returns every distinct consumer name currently
// present in any object's used_by list, sorted, with no duplicates - the
// secret create/edit form's combobox offers these instead of relying on
// free-text recall (alrayyes/hush-hush#251). Gated the same way
// GET /objects is: a write token or a session, since listing needs no id
// the caller already holds.
func handleListConsumers(s objectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		consumers, err := s.ListConsumers(r.Context())
		if err != nil {
			writeInternalError(w, r, err)

			return
		}

		if consumers == nil {
			consumers = []string{}
		}

		writeJSON(w, http.StatusOK, consumers)
	}
}
