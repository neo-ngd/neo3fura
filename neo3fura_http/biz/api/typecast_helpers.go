package api

import (
	"math/big"
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

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

func asDecimalString(v interface{}) (string, bool) {
	switch x := v.(type) {
	case primitive.Decimal128:
		return x.String(), true
	case string:
		return x, true
	case *big.Int:
		return x.String(), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case int:
		return strconv.Itoa(x), true
	default:
		return "", false
	}
}

func asBigInt(v interface{}) (*big.Int, bool) {
	switch x := v.(type) {
	case *big.Int:
		return x, true
	case int64:
		return big.NewInt(x), true
	case int32:
		return big.NewInt(int64(x)), true
	case int:
		return big.NewInt(int64(x)), true
	case string:
		n, ok := new(big.Int).SetString(x, 10)
		return n, ok
	default:
		return nil, false
	}
}
