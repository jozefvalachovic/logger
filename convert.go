package logger

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
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

// handleStruct converts struct to JSON-like representation
func handleStruct(key string, value any) slog.Attr {
	// Try JSON marshaling first (respects json tags)
	if jsonData, err := json.Marshal(value); err == nil {
		var result map[string]any
		if json.Unmarshal(jsonData, &result) == nil {
			return slog.Any(key, result)
		}
	}

	// Fallback to reflection
	rv := reflect.ValueOf(value)
	rt := reflect.TypeOf(value)

	fields := make(map[string]any)
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

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
	for _, mapKey := range rv.MapKeys() {
		keyStr := mapKey.String()
		mapValue := rv.MapIndex(mapKey)
		result[keyStr] = mapValue.Interface()
	}

	return slog.Any(key, result)
}

// marshalAsJSON fallback to JSON marshaling
func marshalAsJSON(key string, value any) slog.Attr {
	if jsonData, err := json.Marshal(value); err == nil {
		return slog.String(key, string(jsonData))
	}
	// Final fallback
	return slog.String(key, reflect.TypeOf(value).String())
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
	case reflect.Ptr:
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
