package ptr

import "strconv"

// Deref returns the pointer value or def when p is nil.
func Deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// Ref returns a pointer to v.
func Ref[T any](v T) *T {
	return &v
}

// StringFromMap returns a string value by key or empty string.
func StringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// IntFromMap returns an int value by key or zero.
func IntFromMap(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		if i, err := strconv.Atoi(t); err == nil {
			return i
		}
		return 0
	default:
		return 0
	}
}

// StringSliceFromMap returns a string slice by key or empty slice.
func StringSliceFromMap(m map[string]interface{}, key string) []string {
	if m == nil {
		return []string{}
	}
	v, ok := m[key]
	if !ok || v == nil {
		return []string{}
	}
	arr, ok := v.([]interface{})
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
