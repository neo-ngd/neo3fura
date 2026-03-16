package job

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
)

func (me T) GetBlockInfoList() error {
	message := make(json.RawMessage, 0)
	ret := &message

	// Use $lookup to get transaction count in a single aggregation query
	// instead of N+1 separate count queries per block.
	r1, err := me.Client.QueryAggregate(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Pipeline   []bson.M
			Query      []string
		}{
			Collection: "Block",
			Index:      "GetBlockInfoList",
			Sort:       bson.M{},
			Filter:     bson.M{},
			Pipeline: []bson.M{
				bson.M{"$sort": bson.M{"_id": -1}},
				bson.M{"$limit": int64(10)},
				bson.M{"$lookup": bson.M{
					"from": "Transaction",
					"let":  bson.M{"blockhash": "$hash"},
					"pipeline": []bson.M{
						bson.M{"$match": bson.M{"$expr": bson.M{"$eq": []interface{}{"$blockhash", "$$blockhash"}}}},
					},
					"as": "txs",
				}},
				bson.M{"$project": bson.M{
					"_id":              1,
					"index":            1,
					"size":             1,
					"timestamp":        1,
					"hash":             1,
					"transactioncount": bson.M{"$size": "$txs"},
				}},
			},
			Query: []string{},
		}, ret)
	if err != nil {
		return err
	}

	data := bson.M{"BlockInfoList": r1}
	_, err = me.Client.SaveJob(struct {
		Collection string
		Data       bson.M
	}{Collection: "BlockInfoList", Data: data})
	if err != nil {
		return err
	}
	return nil
}
