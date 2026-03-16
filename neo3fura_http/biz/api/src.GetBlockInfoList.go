package api

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetBlockInfoList(args struct {
	Filter map[string]interface{}
	Limit  int64
	Skip   int64
	Cursor string
}, ret *json.RawMessage) error {

	if args.Limit == 0 {
		args.Limit = 20
	}

	sortKeys := []string{"_id"}
	sortDirs := map[string]int{"_id": -1}

	pipeline := []bson.M{
		bson.M{"$sort": bson.M{"_id": -1}},
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
			"from": "Transaction",
			"let":  bson.M{"blockhash": "$hash"},
			"pipeline": []bson.M{
				bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
					bson.M{"$eq": []interface{}{"$blockhash", "$$blockhash"}},
				}}}},
			},
			"as": "info"},
		},
		bson.M{"$project": bson.M{"_id": 1, "index": 1, "size": 1, "timestamp": 1, "hash": 1, "transactioncount": bson.M{"$size": "$info"}}},
	)

	r1, err := me.Client.QueryAggregate(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Pipeline   []bson.M
			Query      []string
		}{Collection: "Block",
			Index:    "GetBlockInfoList",
			Sort:     bson.M{},
			Filter:   bson.M{},
			Pipeline: pipeline,
			Query:    []string{}}, ret)

	count, err := me.Client.QueryDocument(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
	}{
		Collection: "Block",
		Index:      "GetBlockInfoList",
		Sort:       bson.M{},
		Filter:     bson.M{}}, ret)
	if err != nil {
		return err
	}

	r4, err := me.FilterArrayAndAppendCountWithCursor(r1, count["total counts"].(int64), args.Filter, sortKeys)
	r, err := json.Marshal(r4)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
