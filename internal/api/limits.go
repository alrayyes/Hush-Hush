package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

// What a request field may hold (alrayyes/hush-hush#617, ADR 31). Each limit
// is a struct tag on the request type, so the rule sits next to the field it
// bounds and a new field can't be added to a handler without seeing it:
//
//	maxLength  characters in a string, or in each entry of a string slice
//	maxItems   entries in a slice
//
// decodeRequest enforces them, the spec states the same numbers, and
// TestSpecStatesTheFieldLimitsTheHandlersEnforce reads RequestLimits to check
// the two agree. A field with no tag is bounded only by the body cap.

// requestTypes names each request type by the schema it matches in
// api/openapi.yaml.
var requestTypes = map[string]any{
	"CreateObjectRequest":        CreateObjectRequest{},
	"UpdateObjectRequest":        UpdateObjectRequest{},
	"AddConsumerRequest":         AddConsumerRequest{},
	"UpdateConsumerRequest":      UpdateConsumerRequest{},
	"CreateTokenRequest":         createTokenBody{},
	"CreateConsumerTokenRequest": createConsumerTokenBody{},
	"CredentialRenameRequest":    CredentialRenameRequest{},
	"RegistrationFinishRequest":  RegistrationFinishRequest{},
}

// FieldLimit is one tagged field's limits, by its JSON name.
type FieldLimit struct {
	JSONName  string
	MaxLength int
	MaxItems  int
}

// RequestLimits is every tagged field's limits, keyed by the spec schema the
// request type matches.
func RequestLimits() map[string][]FieldLimit {
	out := make(map[string][]FieldLimit, len(requestTypes))

	for schema, v := range requestTypes {
		for f := range reflect.TypeOf(v).Fields() {
			if l, ok := fieldLimit(f); ok {
				out[schema] = append(out[schema], l)
			}
		}
	}

	return out
}

func fieldLimit(f reflect.StructField) (FieldLimit, bool) {
	l := FieldLimit{JSONName: jsonName(f)}
	l.MaxLength, _ = strconv.Atoi(f.Tag.Get("maxLength"))
	l.MaxItems, _ = strconv.Atoi(f.Tag.Get("maxItems"))

	return l, l.MaxLength != 0 || l.MaxItems != 0
}

func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "" {
		return f.Name
	}

	return name
}

// checkLimits reports the first tagged field of req, a struct or a pointer to
// one, that is over its limit.
func checkLimits(req any) error {
	v := reflect.Indirect(reflect.ValueOf(req))
	t := v.Type()

	for i := range t.NumField() {
		l, ok := fieldLimit(t.Field(i))
		if !ok {
			continue
		}

		if err := checkField(l, reflect.Indirect(v.Field(i))); err != nil {
			return err
		}
	}

	return nil
}

func checkField(l FieldLimit, v reflect.Value) error {
	switch v.Kind() { //nolint:exhaustive // only strings and string slices carry limits
	case reflect.String:
		if utf8.RuneCountInString(v.String()) > l.MaxLength {
			return fmt.Errorf("%s must be at most %d characters", l.JSONName, l.MaxLength) //nolint:err113 // the message is the point
		}
	case reflect.Slice:
		if l.MaxItems != 0 && v.Len() > l.MaxItems {
			return fmt.Errorf("%s must have at most %d entries", l.JSONName, l.MaxItems) //nolint:err113 // the message is the point
		}

		for i := range v.Len() {
			if err := checkField(FieldLimit{JSONName: l.JSONName, MaxLength: l.MaxLength}, v.Index(i)); err != nil {
				return fmt.Errorf("each entry: %w", err)
			}
		}
	}

	return nil
}

// decodeRequest reads the JSON body into req and checks its field limits.
// When it isn't acceptable it has already answered the caller, and returns
// false.
func decodeRequest(w http.ResponseWriter, r *http.Request, req any) bool {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		status, message := decodeFailure(err)
		writeError(w, r, status, message)

		return false
	}

	if err := checkLimits(req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, err.Error())

		return false
	}

	return true
}
