package api

import (
	"math/big"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestAsBigInt_String(t *testing.T) {
	got, ok := asBigInt("12345678901234567890")
	if !ok {
		t.Fatal("expected string to convert to big.Int")
	}
	if got.String() != "12345678901234567890" {
		t.Fatalf("unexpected value: %s", got.String())
	}
}

func TestAsBigInt_Decimal128(t *testing.T) {
	d128, err := primitive.ParseDecimal128("12345678901234567890")
	if err != nil {
		t.Fatalf("ParseDecimal128 error: %v", err)
	}

	got, ok := asBigInt(d128)
	if !ok {
		t.Fatal("expected Decimal128 to convert to big.Int")
	}
	if got.String() != "12345678901234567890" {
		t.Fatalf("unexpected value: %s", got.String())
	}
}

func TestAsBigInt_BigInt(t *testing.T) {
	want := big.NewInt(42)

	got, ok := asBigInt(want)
	if !ok {
		t.Fatal("expected big.Int to pass through")
	}
	if got.Cmp(want) != 0 {
		t.Fatalf("unexpected value: %s", got.String())
	}
}
