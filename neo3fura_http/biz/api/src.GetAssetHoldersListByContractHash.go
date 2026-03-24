package api

import (
	"encoding/json"
	"math/big"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetAssetHoldersListByContractHash(args struct {
	ContractHash h160.T
	Limit        int64
	Skip         int64
	Cursor       string
	Filter       map[string]interface{}
	Raw          *[]map[string]interface{}
}, ret *json.RawMessage) error {
	if args.ContractHash.Valid() == false {
		return stderr.ErrInvalidArgs
	}
	if args.Limit <= 0 {
		args.Limit = consts.DefaultLimit
	}
	if args.Limit > consts.MaxLimit {
		args.Limit = consts.MaxLimit
	}
	if args.Skip < 0 {
		args.Skip = 0
	}
	sortKeys := []string{"_id"}
	sortDirs := map[string]int{"_id": -1}
	filter := bson.M{"asset": args.ContractHash.Val(), "balance": bson.M{"$gt": 0}}
	if args.Cursor != "" {
		cursorValues, err := DecodeCursor(args.Cursor)
		if err != nil {
			return err
		}
		cursorFilter := BuildCursorFilter(sortKeys, sortDirs, cursorValues)
		filter = MergeCursorFilter(filter, cursorFilter)
		args.Skip = 0
	}
	queryLimit := args.Limit + 1
	r1, count, err := me.Client.QueryAll(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
		Limit      int64
		Skip       int64
	}{
		Collection: "Address-Asset",
		Index:      "GetAssetHoldersListByContractHash",
		Sort:       bson.M{"_id": -1},
		Filter:     filter,
		Query:      []string{},
		Limit:      queryLimit,
		Skip:       args.Skip,
	}, ret)
	if err != nil {
		return err
	}
	hasNext := int64(len(r1)) > args.Limit
	page := r1
	if hasNext {
		page = r1[:args.Limit]
	}

	// 获取资产的totaluspply
	var raw1 map[string]interface{}
	err = me.GetAssetInfoByContractHash(struct {
		ContractHash h160.T
		Filter       map[string]interface{}
		Raw          *map[string]interface{}
	}{ContractHash: args.ContractHash, Raw: &raw1}, ret)
	if err != nil {
		return err
	}

	//it, _, err := raw1["totalsupply"].(primitive.Decimal128).BigInt()
	//if err != nil {
	//	return err
	//}

	it, ok := asBigInt(raw1["totalsupply"])
	if !ok {
		return stderr.ErrData
	}
	itf := new(big.Float).SetInt(it)

	for _, item := range page {

		ib, ok := asBigInt(item["balance"])
		if !ok {
			return stderr.ErrData
		}
		ibf := new(big.Float).SetInt(ib)
		dv := new(big.Float).Quo(ibf, itf)
		item["percentage"] = dv
	}

	if args.Raw != nil {
		*args.Raw = page
	}

	r2, err := me.FilterArrayAndAppendCount(page, count, args.Filter)
	if err != nil {
		return err
	}
	if hasNext {
		last := page[len(page)-1]
		nextCursor := EncodeCursor(last, sortKeys)
		if nextCursor == "" {
			return stderr.ErrInvalidArgs
		}
		r2["nextCursor"] = nextCursor
	}
	r, err := json.Marshal(r2)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
