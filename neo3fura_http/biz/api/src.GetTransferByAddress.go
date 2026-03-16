package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (me *T) GetTransferByAddress(args struct {
	Address h160.T
	Limit   int64
	Skip    int64
	Cursor  string
	Filter  map[string]interface{}
}, ret *json.RawMessage) error {
	if args.Address.Valid() == false {
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

	baseFilter := bson.M{"$or": []interface{}{
		bson.M{"from": args.Address.TransferredVal()},
		bson.M{"to": args.Address.TransferredVal()},
	}}
	queryLimit := args.Limit + 1
	fetchLimit := args.Skip + queryLimit
	if fetchLimit > consts.MaxLimit {
		fetchLimit = consts.MaxLimit
	}

	var cursorFilter bson.M
	if args.Cursor != "" {
		decodedFilter, err := buildIntDescCursorFilter("timestamp", args.Cursor)
		if err != nil {
			return err
		}
		cursorFilter = decodedFilter
		args.Skip = 0
		fetchLimit = queryLimit
	}

	r1, _, err := me.Client.QueryAllWithCursor(struct {
		Collection   string
		Index        string
		Sort         bson.M
		Filter       bson.M
		Query        []string
		Limit        int64
		Skip         int64
		CursorFilter bson.M
	}{
		Collection:   "Nep11TransferNotification",
		Index:        "GetTransferByAddress",
		Sort:         bson.M{"timestamp": -1, "_id": -1},
		Filter:       baseFilter,
		Query:        []string{},
		Limit:        fetchLimit,
		Skip:         0,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
	}

	r2, _, err := me.Client.QueryAllWithCursor(struct {
		Collection   string
		Index        string
		Sort         bson.M
		Filter       bson.M
		Query        []string
		Limit        int64
		Skip         int64
		CursorFilter bson.M
	}{
		Collection:   "TransferNotification",
		Index:        "GetTransferByAddress",
		Sort:         bson.M{"timestamp": -1, "_id": -1},
		Filter:       baseFilter,
		Query:        []string{},
		Limit:        fetchLimit,
		Skip:         0,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
	}

	merged := append(r1, r2...)
	sort.Slice(merged, func(i, j int) bool {
		ti, _ := int64FromAny(merged[i]["timestamp"])
		tj, _ := int64FromAny(merged[j]["timestamp"])
		if ti != tj {
			return ti > tj
		}
		oi, ok1 := merged[i]["_id"].(primitive.ObjectID)
		oj, ok2 := merged[j]["_id"].(primitive.ObjectID)
		if ok1 && ok2 {
			return oi.Hex() > oj.Hex()
		}
		return false
	})

	start := args.Skip
	if start > int64(len(merged)) {
		start = int64(len(merged))
	}
	end := start + queryLimit
	if end > int64(len(merged)) {
		end = int64(len(merged))
	}
	window := merged[start:end]

	hasNext := int64(len(window)) > args.Limit
	page := window
	if hasNext {
		page = window[:args.Limit]
	}

	nep11CountRow, err := me.Client.QueryDocument(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
	}{
		Collection: "Nep11TransferNotification",
		Index:      "GetTransferByAddressCount",
		Sort:       bson.M{},
		Filter:     baseFilter,
	}, ret)
	if err != nil {
		return err
	}
	nep17CountRow, err := me.Client.QueryDocument(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
	}{
		Collection: "TransferNotification",
		Index:      "GetTransferByAddressCount",
		Sort:       bson.M{},
		Filter:     baseFilter,
	}, ret)
	if err != nil {
		return err
	}

	totalCount := nep11CountRow["total counts"].(int64) + nep17CountRow["total counts"].(int64)
	r5, err := me.FilterArrayAndAppendCount(page, totalCount, args.Filter)
	if err != nil {
		return err
	}
	if hasNext {
		last := page[len(page)-1]
		sortValue, ok := int64FromAny(last["timestamp"])
		if !ok {
			return stderr.ErrInvalidArgs
		}
		oid, ok := last["_id"].(primitive.ObjectID)
		if !ok {
			return stderr.ErrInvalidArgs
		}
		nextCursor, err := encodeIntDescCursor(sortValue, oid)
		if err != nil {
			return err
		}
		r5["nextCursor"] = nextCursor
	}
	r, err := json.Marshal(r5)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
