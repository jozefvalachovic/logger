package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"
)

// Helper functions to convert different integer types to int64
func toInt64(v any) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	default:
		return 0
	}
}

// Helper function to convert different unsigned integer types to uint64
func toUint64(v any) uint64 {
	switch val := v.(type) {
	case uint:
		return uint64(val)
	case uint8:
		return uint64(val)
	case uint16:
		return uint64(val)
	case uint32:
		return uint64(val)
	case uint64:
		return val
	default:
		return 0
	}
}

// Helper function to convert different float types to float64
func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float32:
		return float64(val)
	case float64:
		return val
	default:
		return 0.0
	}
}

// BodyToKeyValues converts a body to key-value pairs for logInternal (exported for middleware)
func BodyToKeyValues(key string, body []byte) []any {
	return bodyToKeyValues(key, body)
}

// bodyToKeyValues converts a body to key-value pairs for logInternal (internal)
func bodyToKeyValues(key string, body []byte) []any {
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		// Valid JSON: flatten to key-value pairs under "body." prefix
		// to prevent collision with internal keys like __method, __status.
		keyValues := make([]any, 0, len(obj)*2)
		for k, v := range obj {
			keyValues = append(keyValues, "body."+k, v)
		}
		return keyValues
	}
	// Not JSON: return as [key, string(body)]
	return []any{key, string(body)}
}

// handleStruct converts struct to JSON-like representation
func handleStruct(key string, value any) slog.Attr {
	// Try JSON marshaling first (respects json tags and MarshalJSON)
	if jsonData, err := json.Marshal(value); err == nil {
		if len(jsonData) > 0 && jsonData[0] == '{' {
			// Decode with UseNumber so int64 values above 2^53 keep their precision.
			dec := json.NewDecoder(bytes.NewReader(jsonData))
			dec.UseNumber()
			var result map[string]any
			if dec.Decode(&result) == nil {
				return slog.Any(key, result)
			}
		}
		// A struct whose MarshalJSON yields a string, number or array.
		return jsonAttr(key, jsonData)
	}

	// Fallback to reflection
	rv := reflect.ValueOf(value)

	fields := make(map[string]any)
	for field, fieldValue := range rv.Fields() {
		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Use json tag if available, otherwise use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			if commaIdx := strings.Index(jsonTag, ","); commaIdx > 0 {
				fieldName = jsonTag[:commaIdx]
			} else {
				fieldName = jsonTag
			}
		}

		fields[fieldName] = fieldValue.Interface()
	}

	return slog.Any(key, fields)
}

// handleSliceOrArray converts slices and arrays
func handleSliceOrArray(key string, value any) slog.Attr {
	rv := reflect.ValueOf(value)

	// Empty slice/array
	if rv.Len() == 0 {
		return slog.Any(key, []any{})
	}

	// Convert to slice of any
	result := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		result[i] = rv.Index(i).Interface()
	}

	return slog.Any(key, result)
}

// handleMap converts maps
func handleMap(key string, value any) slog.Attr {
	rv := reflect.ValueOf(value)

	if rv.Len() == 0 {
		return slog.Any(key, map[string]any{})
	}

	result := make(map[string]any)
	for mapKey, mapValue := range rv.Seq2() {
		keyStr := fmt.Sprintf("%v", mapKey.Interface())
		result[keyStr] = mapValue.Interface()
	}

	return slog.Any(key, result)
}

// marshalAsJSON fallback to JSON marshaling
func marshalAsJSON(key string, value any) slog.Attr {
	if jsonData, err := json.Marshal(value); err == nil {
		return jsonAttr(key, jsonData)
	}
	// Final fallback
	return slog.String(key, reflect.TypeOf(value).String())
}

// jsonAttr turns encoded JSON into an attr. A JSON string becomes a plain
// string attr, so it prints once-quoted and pattern redaction still applies;
// anything else (numbers, arrays, objects) is embedded verbatim.
func jsonAttr(key string, data []byte) slog.Attr {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if json.Unmarshal(data, &s) == nil {
			return slog.String(key, s)
		}
	}
	return slog.Any(key, json.RawMessage(data))
}

// errorString returns err's message, or "<nil>" for a typed nil pointer.
// fmt recovers from a panicking Error method.
func errorString(err error) string {
	if rv := reflect.ValueOf(err); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return "<nil>"
	}
	return fmt.Sprint(err)
}

// convertToSlogAttr converts any value to appropriate slog.Attr
func convertToSlogAttr(key string, value any) slog.Attr {
	switch v := value.(type) {
	case string:
		return slog.String(key, v)
	case int, int8, int16, int32, int64:
		return slog.Int64(key, toInt64(v))
	case uint, uint8, uint16, uint32, uint64:
		return slog.Uint64(key, toUint64(v))
	case float32, float64:
		return slog.Float64(key, toFloat64(v))
	case bool:
		return slog.Bool(key, v)
	case nil:
		return slog.String(key, "<nil>")
	case time.Time:
		return slog.Time(key, v)
	case time.Duration:
		return slog.String(key, v.String())
	case error:
		return slog.String(key, errorString(v))
	default:
		// Handle complex types (structs, arrays, slices, maps)
		return handleComplexType(key, v)
	}
}

// handleComplexType processes structs, arrays, slices, and maps
func handleComplexType(key string, value any) slog.Attr {
	rv := reflect.ValueOf(value)

	switch rv.Kind() {
	case reflect.Struct:
		return handleStruct(key, value)
	case reflect.Slice, reflect.Array:
		return handleSliceOrArray(key, value)
	case reflect.Map:
		return handleMap(key, value)
	case reflect.Pointer:
		if rv.IsNil() {
			return slog.String(key, "<nil>")
		}
		// Dereference pointer and process the underlying value
		return convertToSlogAttr(key, rv.Elem().Interface())
	default:
		// For any other type, try JSON marshaling
		return marshalAsJSON(key, value)
	}
}
