package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type intDescCursor struct {
	SortValue int64  `json:"sv"`
	ObjectID  string `json:"id"`
}

type oidCursor struct {
	ObjectID string `json:"id"`
}

func encodeIntDescCursor(sortValue int64, id primitive.ObjectID) (string, error) {
	cursor := intDescCursor{
		SortValue: sortValue,
		ObjectID:  id.Hex(),
	}
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeIntDescCursor(cursor string) (intDescCursor, primitive.ObjectID, error) {
	if cursor == "" {
		return intDescCursor{}, primitive.NilObjectID, errors.New("cursor is empty")
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return intDescCursor{}, primitive.NilObjectID, err
	}
	var decoded intDescCursor
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return intDescCursor{}, primitive.NilObjectID, err
	}
	oid, err := primitive.ObjectIDFromHex(decoded.ObjectID)
	if err != nil {
		return intDescCursor{}, primitive.NilObjectID, err
	}
	return decoded, oid, nil
}

func buildIntDescCursorFilter(sortField string, cursor string) (bson.M, error) {
	decoded, oid, err := decodeIntDescCursor(cursor)
	if err != nil {
		return nil, err
	}
	return bson.M{
		"$or": []bson.M{
			{sortField: bson.M{"$lt": decoded.SortValue}},
			{sortField: decoded.SortValue, "_id": bson.M{"$lt": oid}},
		},
	}, nil
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

func encodeOIDCursor(id primitive.ObjectID) (string, error) {
	cursor := oidCursor{ObjectID: id.Hex()}
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeOIDCursor(cursor string) (primitive.ObjectID, error) {
	if cursor == "" {
		return primitive.NilObjectID, errors.New("cursor is empty")
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return primitive.NilObjectID, err
	}
	var decoded oidCursor
	if err = json.Unmarshal(raw, &decoded); err != nil {
		return primitive.NilObjectID, err
	}
	return primitive.ObjectIDFromHex(decoded.ObjectID)
}

func buildOIDDescCursorFilter(cursor string) (bson.M, error) {
	oid, err := decodeOIDCursor(cursor)
	if err != nil {
		return nil, err
	}
	return bson.M{"_id": bson.M{"$lt": oid}}, nil
}
