package api

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestEncodeDecode_EmptyInput(t *testing.T) {
	// nil doc
	if got := EncodeCursor(nil, []string{"_id"}); got != "" {
		t.Errorf("expected empty cursor for nil doc, got %q", got)
	}
	// empty sortKeys
	if got := EncodeCursor(map[string]interface{}{"_id": 1}, nil); got != "" {
		t.Errorf("expected empty cursor for nil sortKeys, got %q", got)
	}
	// empty cursor string
	vals, err := DecodeCursor("")
	if err != nil || vals != nil {
		t.Errorf("expected nil/nil for empty cursor, got %v, %v", vals, err)
	}
}

func TestEncodeDecode_Int64(t *testing.T) {
	doc := map[string]interface{}{
		"timestamp": int64(1700000000000),
	}
	cursor := EncodeCursor(doc, []string{"timestamp"})
	if cursor == "" {
		t.Fatal("expected non-empty cursor")
	}

	vals, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor error: %v", err)
	}
	ts, ok := vals["timestamp"].(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", vals["timestamp"])
	}
	if ts != 1700000000000 {
		t.Errorf("expected 1700000000000, got %d", ts)
	}
}

func TestEncodeDecode_ObjectID(t *testing.T) {
	oid := primitive.NewObjectID()
	doc := map[string]interface{}{
		"_id": oid,
	}
	cursor := EncodeCursor(doc, []string{"_id"})
	if cursor == "" {
		t.Fatal("expected non-empty cursor")
	}

	vals, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor error: %v", err)
	}
	decoded, ok := vals["_id"].(primitive.ObjectID)
	if !ok {
		t.Fatalf("expected ObjectID, got %T", vals["_id"])
	}
	if decoded != oid {
		t.Errorf("expected %s, got %s", oid.Hex(), decoded.Hex())
	}
}

func TestEncodeDecode_Decimal128(t *testing.T) {
	d128, _ := primitive.ParseDecimal128("12345.6789")
	doc := map[string]interface{}{
		"balance": d128,
	}
	cursor := EncodeCursor(doc, []string{"balance"})
	if cursor == "" {
		t.Fatal("expected non-empty cursor")
	}

	vals, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor error: %v", err)
	}
	decoded, ok := vals["balance"].(primitive.Decimal128)
	if !ok {
		t.Fatalf("expected Decimal128, got %T", vals["balance"])
	}
	if decoded.String() != "12345.6789" {
		t.Errorf("expected 12345.6789, got %s", decoded.String())
	}
}

func TestEncodeDecode_MultipleFields(t *testing.T) {
	oid := primitive.NewObjectID()
	doc := map[string]interface{}{
		"timestamp": int64(1700000000000),
		"_id":       oid,
	}
	sortKeys := []string{"timestamp", "_id"}
	cursor := EncodeCursor(doc, sortKeys)

	vals, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor error: %v", err)
	}
	if vals["timestamp"].(int64) != 1700000000000 {
		t.Error("timestamp mismatch")
	}
	if vals["_id"].(primitive.ObjectID) != oid {
		t.Error("_id mismatch")
	}
}

func TestEncodeDecode_String(t *testing.T) {
	doc := map[string]interface{}{
		"hash": "0xabc123",
	}
	cursor := EncodeCursor(doc, []string{"hash"})
	vals, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor error: %v", err)
	}
	if vals["hash"].(string) != "0xabc123" {
		t.Errorf("expected 0xabc123, got %v", vals["hash"])
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := DecodeCursor("!!!invalid!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	// Valid base64 but invalid JSON
	_, err := DecodeCursor("bm90LWpzb24=") // "not-json"
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- BuildCursorFilter tests ---

func TestBuildCursorFilter_SingleDesc(t *testing.T) {
	filter := BuildCursorFilter(
		[]string{"_id"},
		map[string]int{"_id": -1},
		map[string]interface{}{"_id": int64(100)},
	)
	expected := bson.M{"_id": bson.M{"$lt": int64(100)}}
	assertBsonEqual(t, expected, filter)
}

func TestBuildCursorFilter_SingleAsc(t *testing.T) {
	filter := BuildCursorFilter(
		[]string{"tokenid"},
		map[string]int{"tokenid": 1},
		map[string]interface{}{"tokenid": "abc"},
	)
	expected := bson.M{"tokenid": bson.M{"$gt": "abc"}}
	assertBsonEqual(t, expected, filter)
}

func TestBuildCursorFilter_CompoundDescDesc(t *testing.T) {
	oid := primitive.NewObjectID()
	filter := BuildCursorFilter(
		[]string{"timestamp", "_id"},
		map[string]int{"timestamp": -1, "_id": -1},
		map[string]interface{}{"timestamp": int64(1700000000000), "_id": oid},
	)
	// Expected: {$or: [{timestamp: {$lt: ...}}, {timestamp: ..., _id: {$lt: ...}}]}
	orClauses := filter["$or"].([]interface{})
	if len(orClauses) != 2 {
		t.Fatalf("expected 2 $or clauses, got %d", len(orClauses))
	}
	// First clause: timestamp < cursor
	c0 := orClauses[0].(bson.M)
	if c0["timestamp"].(bson.M)["$lt"].(int64) != 1700000000000 {
		t.Error("first clause timestamp mismatch")
	}
	// Second clause: timestamp == cursor AND _id < cursor
	c1 := orClauses[1].(bson.M)
	if c1["timestamp"].(int64) != 1700000000000 {
		t.Error("second clause timestamp should be equal")
	}
	if c1["_id"].(bson.M)["$lt"].(primitive.ObjectID) != oid {
		t.Error("second clause _id mismatch")
	}
}

func TestBuildCursorFilter_EmptyInput(t *testing.T) {
	if f := BuildCursorFilter(nil, nil, nil); f != nil {
		t.Error("expected nil for empty input")
	}
	if f := BuildCursorFilter([]string{"_id"}, map[string]int{"_id": -1}, nil); f != nil {
		t.Error("expected nil for nil cursorValues")
	}
	if f := BuildCursorFilter([]string{"_id"}, map[string]int{"_id": -1}, map[string]interface{}{}); f != nil {
		t.Error("expected nil for empty cursorValues")
	}
}

// --- BuildCursorMatchStage tests ---

func TestBuildCursorMatchStage_EmptyCursor(t *testing.T) {
	stage, err := BuildCursorMatchStage([]string{"_id"}, map[string]int{"_id": -1}, "")
	if err != nil || stage != nil {
		t.Error("expected nil for empty cursor")
	}
}

func TestBuildCursorMatchStage_RoundTrip(t *testing.T) {
	oid := primitive.NewObjectID()
	doc := map[string]interface{}{
		"timestamp": int64(1700000000000),
		"_id":       oid,
	}
	sortKeys := []string{"timestamp", "_id"}
	sortDirs := map[string]int{"timestamp": -1, "_id": -1}

	cursor := EncodeCursor(doc, sortKeys)
	stage, err := BuildCursorMatchStage(sortKeys, sortDirs, cursor)
	if err != nil {
		t.Fatalf("BuildCursorMatchStage error: %v", err)
	}
	if stage == nil {
		t.Fatal("expected non-nil stage")
	}
	matchFilter := stage["$match"].(bson.M)
	orClauses := matchFilter["$or"].([]interface{})
	if len(orClauses) != 2 {
		t.Fatalf("expected 2 $or clauses, got %d", len(orClauses))
	}
}

// --- MergeCursorFilter tests ---

func TestMergeCursorFilter_NilCursor(t *testing.T) {
	existing := bson.M{"asset": "0xabc"}
	result := MergeCursorFilter(existing, nil)
	if result["asset"] != "0xabc" {
		t.Error("expected existing filter unchanged")
	}
}

func TestMergeCursorFilter_NilExisting(t *testing.T) {
	cursor := bson.M{"_id": bson.M{"$lt": int64(100)}}
	result := MergeCursorFilter(nil, cursor)
	if result["_id"] == nil {
		t.Error("expected cursor filter returned")
	}
}

func TestMergeCursorFilter_BothPresent(t *testing.T) {
	existing := bson.M{"asset": "0xabc"}
	cursor := bson.M{"_id": bson.M{"$lt": int64(100)}}
	result := MergeCursorFilter(existing, cursor)
	andClauses := result["$and"].([]interface{})
	if len(andClauses) != 2 {
		t.Fatalf("expected 2 $and clauses, got %d", len(andClauses))
	}
}

// --- Helpers ---

func assertBsonEqual(t *testing.T, expected, actual bson.M) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("bson length mismatch: expected %d, got %d\nexpected: %v\nactual: %v", len(expected), len(actual), expected, actual)
	}
}
