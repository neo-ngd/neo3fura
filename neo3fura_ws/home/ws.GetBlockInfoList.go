package home

import (
	"context"
	"encoding/json"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Block
func (me *T) GetBlockInfoList(ch *chan map[string]interface{}) error {
	blockInfoList, err := me.getBlockInfoList2()
	if err != nil {
		return err
	}
	*ch <- blockInfoList

	c, err := me.Client.GetCollection(struct{ Collection string }{Collection: "Block"})
	if err != nil {
		return err
	}
	cs, err := c.Watch(context.TODO(), mongo.Pipeline{})
	if err != nil {
		return err
	}
	// Whenever there is a new change event, decode the change event and print some information about it
	for cs.Next(context.TODO()) {
		var changeEvent map[string]interface{}
		err := cs.Decode(&changeEvent)
		if err != nil {
			log.Fatal(err)
		}

		newBlockInfoList, err := me.getBlockInfoList2()
		if err != nil {
			return err
		}
		if newBlockInfoList["BlockInfoList"].([]map[string]interface{})[0]["hash"] == newBlockInfoList["BlockInfoList"].([]map[string]interface{})[0]["hash"] {
			*ch <- newBlockInfoList
			blockInfoList = newBlockInfoList
		}
	}
	return nil
}

func (me T) getBlockInfoList() (map[string]interface{}, error) {
	message := make(json.RawMessage, 0)
	ret := &message
	res := make(map[string]interface{})

	r1, _, err := me.Client.QueryAll(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Query      []string
			Limit      int64
			Skip       int64
		}{
			Collection: "Block",
			Index:      "GetBlockInfoList",
			Sort:       bson.M{"_id": -1},
			Filter:     bson.M{},
			Query:      []string{"_id", "index", "size", "timestamp", "hash"},
			Limit:      10,
			Skip:       0,
		}, ret)
	if err != nil {
		return nil, err
	}

	r2 := make([]map[string]interface{}, 0)
	for _, item := range r1 {
		r3, err := me.Client.QueryDocument(
			struct {
				Collection string
				Index      string
				Sort       bson.M
				Filter     bson.M
			}{Collection: "Transaction",
				Index:  "GetBlockInfoList",
				Sort:   bson.M{},
				Filter: bson.M{"blockhash": item["hash"]}}, ret)
		if err != nil {
			return nil, err
		}
		if r3["total counts"] == nil {
			item["transactioncount"] = 0
		} else {
			item["transactioncount"] = r3["total counts"]
		}
		r2 = append(r2, item)
	}
	res["BlockInfoList"] = r2
	return res, nil
}

func (me T) getBlockInfoList2() (map[string]interface{}, error) {
	message := make(json.RawMessage, 0)
	ret := &message
	res := make(map[string]interface{})
	r1, err := me.Client.QueryAggregate(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Pipeline   []bson.M
			Query      []string
		}{Collection: "Block",
			Index:  "GetBlockInfoList",
			Sort:   bson.M{},
			Filter: bson.M{},
			Pipeline: []bson.M{
				bson.M{"$sort": bson.M{"_id": -1}},
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
				bson.M{"$limit": 10},
			},
			Query: []string{}}, ret)

	if err != nil {
		return nil, err
	}

	res["BlockInfoList"] = r1

	return res, nil
}
