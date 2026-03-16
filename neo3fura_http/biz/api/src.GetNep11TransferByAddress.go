package api

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetNep11TransferByAddress(args struct {
	Address h160.T
	Limit   int64
	Skip    int64
	Cursor  string
	Start   int64
	End     int64
	Cursor  string
	Filter  map[string]interface{}
	Raw     *[]map[string]interface{}
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

	if args.Start > 0 && args.End > 0 {
		if args.Start >= args.End {
			return stderr.ErrArgsInner
		}
		baseFilter["$and"] = []interface{}{
			bson.M{"timestamp": bson.M{"$gte": args.Start}},
			bson.M{"timestamp": bson.M{"$lte": args.End}},
		}

	} else if args.Start > 0 && args.End == 0 {
		baseFilter["timestamp"] = bson.M{"$gte": args.Start}
	} else if args.Start == 0 && args.End > 0 {
		baseFilter["timestamp"] = bson.M{"$lte": args.Start}

	}
	filter := baseFilter
	if args.Cursor != "" {
		cursorFilter, err := buildIntDescCursorFilter("timestamp", args.Cursor)
		if err != nil {
			return err
		}
		filter = bson.M{
			"$and": []interface{}{
				filter,
				cursorFilter,
			},
		}
		args.Skip = 0
	}
	queryLimit := args.Limit + 1

	sortKeys := []string{"timestamp", "_id"}
	sortDirs := map[string]int{"timestamp": -1, "_id": -1}

	pipeline := []bson.M{
		bson.M{"$match": filter},
		bson.M{"$sort": bson.M{"timestamp": -1, "_id": -1}},
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

	pipeline = append(pipeline,
		bson.M{"$limit": args.Limit},
		bson.M{"$lookup": bson.M{
			"from": "Execution",
			"let":  bson.M{"txid": "$txid", "blockhash": "$blockhash"},
			"pipeline": []bson.M{
				bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
					bson.M{"$eq": []interface{}{"$txid", "$$txid"}},
					bson.M{"$eq": []interface{}{"$blockhash", "$$blockhash"}},
				}}}},
				bson.M{"$project": bson.M{"vmstate": 1}},
			},
			"as": "execution"},
		},

		bson.M{"$lookup": bson.M{
			"from": "Transaction",
			"let":  bson.M{"hash": "$txid", "blockhash": "$blockhash"},
			"pipeline": []bson.M{
				bson.M{"$match": bson.M{"hash": bson.M{"$ne": "0x0000000000000000000000000000000000000000000000000000000000000000"}}},
				bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
					bson.M{"$eq": []interface{}{"$hash", "$$hash"}},
					bson.M{"$eq": []interface{}{"$blockhash", "$$blockhash"}},
				}}}},
				bson.M{"$project": bson.M{"netfee": 1, "sysfee": 1}},
			},
			"as": "transaction"},
		},
	)

	r1, err := me.Client.QueryAggregate(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Pipeline   []bson.M
		Query      []string
	}{
		Collection: "Nep11TransferNotification",
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

	count, err := me.Client.QueryDocument(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
	}{
		Collection: "Nep11TransferNotification",
		Index:      "GetNep11TransferByAddress",
		Sort:       bson.M{},
		Filter:     baseFilter}, ret)
	if err != nil {
		return err
	}

	for _, item := range page {
		execution := item["execution"].(primitive.A)
		if len(execution) > 0 {
			item["vmstate"] = execution[0].(map[string]interface{})["vmstate"]

		} else {
			item["vmstate"] = "FAULT"
		}
		transaction := item["transaction"].(primitive.A)
		if len(transaction) > 0 {
			transaction_map := transaction[0].(map[string]interface{})
			item["sysfee"] = transaction_map["sysfee"]
			item["netfee"] = transaction_map["netfee"]
		}
		delete(item, "execution")
		delete(item, "transaction")
	}

	r2, err := me.FilterArrayAndAppendCountWithCursor(r1, count["total counts"].(int64), args.Filter, sortKeys)
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
		r2["nextCursor"] = nextCursor
	}
	r, err := json.Marshal(r2)
	if err != nil {
		return err

	}
	if args.Raw != nil {
		*args.Raw = page
	}
	*ret = json.RawMessage(r)
	return nil
}
