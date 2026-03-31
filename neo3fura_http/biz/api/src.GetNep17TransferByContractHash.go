package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetNep17TransferByContractHash(args struct {
	ContractHash h160.T
	Limit        int64
	Skip         int64
	Cursor       string
	Filter       map[string]interface{}
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

	sortKeys := []string{"_id"}
	sortDirs := map[string]int{"_id": -1}

	// Phase 1: fetch transfers without $lookup (limit+1 for hasNext detection)
	queryLimit := args.Limit + 1
	pipeline := []bson.M{
		{"$match": bson.M{"contract": args.ContractHash.Val()}},
		{"$sort": bson.M{"_id": -1}},
	}
	if args.Cursor != "" {
		cursorMatch, err := BuildCursorMatchStage(sortKeys, sortDirs, args.Cursor)
		if err != nil {
			return err
		}
		if cursorMatch != nil {
			pipeline = append(pipeline, cursorMatch)
		}
	} else {
		pipeline = append(pipeline, bson.M{"$skip": args.Skip})
	}
	pipeline = append(pipeline, bson.M{"$limit": queryLimit})

	r1, err := me.Client.QueryAggregate(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Pipeline   []bson.M
		Query      []string
	}{
		Collection: "TransferNotification",
		Index:      "GetNep17TransferByContractHash",
		Sort:       bson.M{},
		Filter:     bson.M{},
		Pipeline:   pipeline,
		Query:      []string{},
	}, ret)
	if err != nil {
		return err
	}

	hasNext := int64(len(r1)) > args.Limit
	page := r1
	if hasNext {
		page = r1[:args.Limit]
	}

	// Phase 2: batch fetch vmstate from Execution for all transfers in page
	if len(page) > 0 {
		orConditions := make([]interface{}, 0, len(page))
		for _, item := range page {
			orConditions = append(orConditions, bson.M{
				"txid":      item["txid"],
				"blockhash": item["blockhash"],
			})
		}
		executions, err := me.Client.QueryFind(struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Query      []string
			Limit      int64
		}{
			Collection: "Execution",
			Index:      "GetNep17TransferByContractHash",
			Sort:       bson.M{},
			Filter:     bson.M{"$or": orConditions},
			Query:      []string{"txid", "blockhash", "vmstate"},
			Limit:      int64(len(page)),
		}, ret)
		if err != nil && err != stderr.ErrNotFound {
			return err
		}

		// Build lookup map: "txid|blockhash" -> vmstate
		execMap := make(map[string]string, len(executions))
		for _, ex := range executions {
			key := stringify(ex["txid"]) + "|" + stringify(ex["blockhash"])
			if vs, ok := ex["vmstate"].(string); ok {
				execMap[key] = vs
			}
		}
		for _, item := range page {
			key := stringify(item["txid"]) + "|" + stringify(item["blockhash"])
			if vs, ok := execMap[key]; ok {
				item["vmstate"] = vs
			} else {
				item["vmstate"] = "FAULT"
			}
		}
	}

	// Count total only on first page (no cursor)
	var totalCount int64
	if args.Cursor == "" {
		countDoc, err := me.Client.QueryDocument(struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
		}{
			Collection: "TransferNotification",
			Index:      "GetNep17TransferByContractHash",
			Sort:       bson.M{},
			Filter:     bson.M{"contract": args.ContractHash.Val()},
		}, ret)
		if err != nil {
			return err
		}
		totalCount = countDoc["total counts"].(int64)
	}

	r2, err := me.FilterArrayAndAppendCount(page, totalCount, args.Filter)
	if err != nil {
		return err
	}
	if hasNext {
		r2["nextCursor"] = EncodeCursor(page[len(page)-1], sortKeys)
	}
	r, err := json.Marshal(r2)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}

func stringify(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
