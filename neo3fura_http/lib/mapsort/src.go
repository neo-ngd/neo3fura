package mapsort

import (
	"math/big"
	"sort"
)

// MapSort sorts by int64 key descending.
func MapSort(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(int64) > ms[j][key].(int64)
	})
	return ms
}

// MapSort2 sorts by int64 key ascending.
func MapSort2(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(int64) < ms[j][key].(int64)
	})
	return ms
}

// MapSort3 sorts by float64 key ascending.
func MapSort3(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(float64) < ms[j][key].(float64)
	})
	return ms
}

// MapSort4 sorts by *big.Int key descending.
func MapSort4(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(*big.Int).Cmp(ms[j][key].(*big.Int)) > 0
	})
	return ms
}

// MapSort5 sorts by int32 key ascending.
func MapSort5(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(int32) < ms[j][key].(int32)
	})
	return ms
}

// MapSort6 sorts by *big.Float key descending.
func MapSort6(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(*big.Float).Cmp(ms[j][key].(*big.Float)) > 0
	})
	return ms
}

// MapSort7 sorts by *big.Float key ascending.
func MapSort7(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(*big.Float).Cmp(ms[j][key].(*big.Float)) < 0
	})
	return ms
}

// MapSort8 sorts by string key ascending.
func MapSort8(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(string) < ms[j][key].(string)
	})
	return ms
}

// MapSort9 sorts by *big.Int key descending (same as MapSort4).
func MapSort9(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(*big.Int).Cmp(ms[j][key].(*big.Int)) > 0
	})
	return ms
}

// MapSort10 sorts by *big.Int key ascending.
func MapSort10(ms []map[string]interface{}, key string) []map[string]interface{} {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i][key].(*big.Int).Cmp(ms[j][key].(*big.Int)) < 0
	})
	return ms
}
