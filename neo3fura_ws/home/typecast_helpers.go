package home

import (
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func asMap(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func asString(v interface{}) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func asInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int32:
		return int64(x), true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	case float32:
		return int64(x), true
	case string:
		if x == "" {
			return 0, false
		}
		n, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func asInt32(v interface{}) (int32, bool) {
	n, ok := asInt64(v)
	if !ok {
		return 0, false
	}
	return int32(n), true
}

func asPrimitiveA(v interface{}) (primitive.A, bool) {
	switch x := v.(type) {
	case primitive.A:
		return x, true
	case []interface{}:
		return primitive.A(x), true
	default:
		return nil, false
	}
}

func extractTotalCounts(v interface{}) (int64, bool) {
	m, ok := asMap(v)
	if !ok {
		return 0, false
	}
	return asInt64(m["total counts"])
}

func extractFirstHash(v interface{}) (string, bool) {
	switch x := v.(type) {
	case []map[string]interface{}:
		if len(x) == 0 {
			return "", false
		}
		return asString(x[0]["hash"])
	case []interface{}:
		if len(x) == 0 {
			return "", false
		}
		first, ok := asMap(x[0])
		if !ok {
			return "", false
		}
		return asString(first["hash"])
	default:
		return "", false
	}
}
