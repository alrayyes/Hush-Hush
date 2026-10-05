package api

import (
	"fmt"
	"net/http"
	"unicode/utf8"
)

// What a query parameter or a header may hold (alrayyes/hush-hush#617,
// ADR 31). Without a limit here, a header or a query value is bounded only by
// MaxHeaderBytes, and X-Caller is stored in the audit log. Every route checks
// them, and TestSpecStatesTheSameParameterLimitsTheServerEnforces compares
// each with the spec's own maxLength.
//
// A parameter that isn't a free string (page, limit, order, a tag) is
// validated by its handler, which already rejects anything out of range.
var (
	queryLimits = map[string]int{
		"q": 128, "used_by": 128, "object_id": 128, "caller": 128, "actor": 128,
		// An RFC 3339 timestamp is at most 35 characters, so this is generous.
		"from": 64, "to": 64,
	}
	headerLimits = map[string]int{"X-Caller": 128, "X-CSRF-Token": 128}
)

// checkRequestMetadata reports the first header or query value over its limit.
func checkRequestMetadata(r *http.Request) error {
	for name, limit := range headerLimits {
		if utf8.RuneCountInString(r.Header.Get(name)) > limit {
			return fmt.Errorf("%s must be at most %d characters", name, limit) //nolint:err113 // the message is the point
		}
	}

	for name, values := range r.URL.Query() {
		limit, limited := queryLimits[name]
		if !limited {
			continue
		}

		for _, v := range values {
			if utf8.RuneCountInString(v) > limit {
				return fmt.Errorf("query parameter %s must be at most %d characters", name, limit) //nolint:err113 // the message is the point
			}
		}
	}

	return nil
}
