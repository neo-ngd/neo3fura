package api

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetContractList(args struct {
	Filter map[string]interface{}
	Limit  int64
	Skip   int64
	Cursor string
}, ret *json.RawMessage) error {
	if args.Limit == 0 {
		args.Limit = 512
	}

	sortKeys := []string{"createtime", "hash"}
	sortDirs := map[string]int{"createtime": -1, "hash": 1}

	pipeline := []bson.M{
		bson.M{"$addFields": bson.M{
			"createtimeLong": bson.M{
				"$convert": bson.M{
					"input":   "$createtime",
					"to":      "long",
					"onError": int64(0),
					"onNull":  int64(0),
				},
			},
		}},
		bson.M{"$sort": bson.M{"hash": 1, "updatecounter": -1, "_id": -1}},
		bson.M{"$group": bson.M{"_id": "$hash",
			"hash":          bson.M{"$first": "$hash"},
			"updatecounter": bson.M{"$first": "$updatecounter"},
			"createtime":    bson.M{"$min": "$createtimeLong"},
			"name":          bson.M{"$first": "$name"},
			"id":            bson.M{"$first": "$id"},
			"createTxid":    bson.M{"$first": "$createTxid"},
		},
		},
		bson.M{"$sort": bson.M{"createtime": -1, "hash": 1}},
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
			"from":         "Transaction",
			"localField":   "createTxid",
			"foreignField": "hash",
			"as":           "Transaction"}},
		bson.M{"$project": bson.M{"_id": 0, "Transaction.sender": 1, "hash": 1, "createtime": 1, "name": 1, "id": 1, "updatecounter": 1}},
	)

	var r1, err = me.Client.QueryAggregate(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Pipeline   []bson.M
			Query      []string
		}{
			Collection: "Contract",
			Index:      "GetContractList",
			Sort:       bson.M{},
			Filter:     bson.M{},
			Pipeline:   pipeline,
			Query:      []string{},
		}, ret)
	if err != nil {
		return err
	}
	r2, err := me.Client.QueryAggregate(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Pipeline   []bson.M
			Query      []string
		}{
			Collection: "Contract",
			Index:      "GetContractList",
			Sort:       bson.M{},
			Filter:     bson.M{},
			Pipeline: []bson.M{
				bson.M{"$group": bson.M{"_id": "$hash"}},
				bson.M{"$count": "total counts"},
			},
			Query: []string{},
		}, ret)
	if err != nil {
		return err
	}
	var count interface{}
	if len(r2) != 0 {
		count = r2[0]["total counts"]
	} else {
		count = 0
	}
	//r1 = append(r1, r3)
	r3, err := me.FilterAggragateAndAppendCountWithCursor(r1, count, args.Filter, sortKeys)
	if err != nil {
		return err
	}
	r, err := json.Marshal(r3)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
