package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CursorValue holds the sort field values for keyset pagination
type CursorValue struct {
	Fields map[string]interface{} `json:"f"`
}

// EncodeCursor encodes the last document's sort field values into a base64 cursor string.
// sortKeys: the field names used for sorting (e.g., ["timestamp", "_id"])
func EncodeCursor(lastDoc map[string]interface{}, sortKeys []string) string {
	if lastDoc == nil || len(sortKeys) == 0 {
		return ""
	}
	cv := CursorValue{Fields: make(map[string]interface{})}
	for _, key := range sortKeys {
		val := lastDoc[key]
		if val == nil {
			continue
		}
		// Convert MongoDB types to JSON-safe types
		switch v := val.(type) {
		case primitive.ObjectID:
			cv.Fields[key] = map[string]string{"$oid": v.Hex()}
		case primitive.Decimal128:
			cv.Fields[key] = map[string]string{"$decimal": v.String()}
		default:
			cv.Fields[key] = v
		}
	}
	data, err := json.Marshal(cv)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(data)
}

// DecodeCursor decodes a base64 cursor string back to sort field values.
func DecodeCursor(cursor string) (map[string]interface{}, error) {
	if cursor == "" {
		return nil, nil
	}
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	var cv CursorValue
	if err := json.Unmarshal(data, &cv); err != nil {
		return nil, fmt.Errorf("invalid cursor format: %w", err)
	}
	// Restore MongoDB types
	for key, val := range cv.Fields {
		if m, ok := val.(map[string]interface{}); ok {
			if oid, exists := m["$oid"]; exists {
				objID, err := primitive.ObjectIDFromHex(oid.(string))
				if err == nil {
					cv.Fields[key] = objID
				}
			} else if dec, exists := m["$decimal"]; exists {
				d128, err := primitive.ParseDecimal128(dec.(string))
				if err == nil {
					cv.Fields[key] = d128
				}
			}
		}
		// JSON numbers are float64, convert to int64 for timestamp-like fields
		if f, ok := val.(float64); ok {
			if f == float64(int64(f)) {
				cv.Fields[key] = int64(f)
			}
		}
	}
	return cv.Fields, nil
}

// BuildCursorFilter builds a keyset pagination filter for MongoDB.
// sortKeys: ordered list of sort field names (e.g., ["timestamp", "_id"])
// sortDirs: map of field name to sort direction (-1 for desc, 1 for asc)
// cursorValues: decoded cursor values
//
// For descending sort on (timestamp, _id), generates:
//
//	{$or: [
//	  {timestamp: {$lt: cursorTimestamp}},
//	  {timestamp: cursorTimestamp, _id: {$lt: cursorID}}
//	]}
func BuildCursorFilter(sortKeys []string, sortDirs map[string]int, cursorValues map[string]interface{}) bson.M {
	if len(sortKeys) == 0 || len(cursorValues) == 0 {
		return nil
	}

	// Single sort key: simple comparison
	if len(sortKeys) == 1 {
		key := sortKeys[0]
		val := cursorValues[key]
		if val == nil {
			return nil
		}
		op := "$lt"
		if sortDirs[key] == 1 {
			op = "$gt"
		}
		return bson.M{key: bson.M{op: val}}
	}

	// Multiple sort keys: compound $or filter
	orClauses := make([]interface{}, 0, len(sortKeys))
	for i := 0; i < len(sortKeys); i++ {
		clause := bson.M{}
		// All previous keys must be equal
		for j := 0; j < i; j++ {
			k := sortKeys[j]
			clause[k] = cursorValues[k]
		}
		// Current key uses comparison
		k := sortKeys[i]
		val := cursorValues[k]
		if val == nil {
			continue
		}
		op := "$lt"
		if sortDirs[k] == 1 {
			op = "$gt"
		}
		clause[k] = bson.M{op: val}
		orClauses = append(orClauses, clause)
	}

	if len(orClauses) == 0 {
		return nil
	}
	return bson.M{"$or": orClauses}
}

// BuildCursorMatchStage builds a $match stage for cursor-based pagination in aggregate pipelines.
// Returns nil if no cursor filter is needed.
func BuildCursorMatchStage(sortKeys []string, sortDirs map[string]int, cursor string) (bson.M, error) {
	if cursor == "" {
		return nil, nil
	}
	cursorValues, err := DecodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	if cursorValues == nil {
		return nil, nil
	}
	f := BuildCursorFilter(sortKeys, sortDirs, cursorValues)
	if f == nil {
		return nil, nil
	}
	return bson.M{"$match": f}, nil
}

// MergeCursorFilter merges a cursor filter into an existing filter using $and.
// If the existing filter already has $and, the cursor conditions are appended.
func MergeCursorFilter(existingFilter bson.M, cursorFilter bson.M) bson.M {
	if cursorFilter == nil {
		return existingFilter
	}
	if existingFilter == nil {
		return cursorFilter
	}
	// Merge using $and
	andClauses := []interface{}{existingFilter, cursorFilter}
	return bson.M{"$and": andClauses}
}

func int64FromAny(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}
