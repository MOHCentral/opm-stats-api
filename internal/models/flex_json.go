package models

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// rawEventFieldMap caches JSON tag -> struct field index mappings
var (
	rawEventFieldMap     map[string]int
	rawEventFieldMapOnce sync.Once
)

func getRawEventFieldMap() map[string]int {
	rawEventFieldMapOnce.Do(func() {
		t := reflect.TypeOf(RawEvent{})
		rawEventFieldMap = make(map[string]int, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			tag := t.Field(i).Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			rawEventFieldMap[name] = i
		}
	})
	return rawEventFieldMap
}

// UnmarshalJSON implements flexible JSON unmarshaling that accepts both
// string-encoded and native JSON types. Game engines (OpenMOHAA's make_json)
// may serialize all values as quoted strings; this handles coercion to the
// correct Go types transparently.
func (e *RawEvent) UnmarshalJSON(data []byte) error {
	// Alias prevents infinite recursion
	type Alias RawEvent
	a := (*Alias)(e)

	// Fast path: try standard unmarshal (works when all types match natively)
	if err := json.Unmarshal(data, a); err == nil {
		return nil
	}

	// Slow path: field-by-field with string-to-native coercion
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("flex unmarshal: %w", err)
	}

	fieldMap := getRawEventFieldMap()
	v := reflect.ValueOf(a).Elem()

	for key, rawVal := range raw {
		idx, ok := fieldMap[key]
		if !ok {
			continue
		}

		fv := v.Field(idx)
		if !fv.CanSet() {
			continue
		}

		// Try direct unmarshal first
		ptr := reflect.New(fv.Type())
		if err := json.Unmarshal(rawVal, ptr.Interface()); err == nil {
			fv.Set(ptr.Elem())
			continue
		}

		// Value is a JSON string but target is numeric/bool — coerce
		if len(rawVal) > 1 && rawVal[0] == '"' {
			var s string
			if err := json.Unmarshal(rawVal, &s); err != nil {
				continue
			}
			if s == "" {
				continue
			}
			coerceStringToField(fv, s)
		}
	}

	return nil
}

// coerceStringToField converts a string value to the field's native type.
func coerceStringToField(fv reflect.Value, s string) {
	switch fv.Kind() {
	case reflect.Float32, reflect.Float64:
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			fv.SetFloat(n)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// ParseFloat handles "28.5" → truncate to int
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			fv.SetInt(int64(n))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, err := strconv.ParseFloat(s, 64); err == nil && n >= 0 {
			fv.SetUint(uint64(n))
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(s); err == nil {
			fv.SetBool(b)
		}
	case reflect.String:
		fv.SetString(s)
	}
}
