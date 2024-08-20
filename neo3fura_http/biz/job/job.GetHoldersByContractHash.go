package job

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

func (me T) GetHoldersByContractHash() error {
	message := make(json.RawMessage, 0)
	ret := &message
	data := make([]bson.M, 0)

	r0, err := me.Client.QueryAggregate(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Pipeline   []bson.M
		Query      []string
	}{
		Collection: "Address-Asset",
		Index:      "GetHoldersByContractHash",
		Sort:       bson.M{},
		Filter:     bson.M{},
		Pipeline: []bson.M{
			bson.M{"$match": bson.M{"balance": bson.M{"$gt": 0}}},
			bson.M{"$group": bson.M{"_id": "$asset", "count": bson.M{"$sum": 1}}},
		},
		Query: []string{},
	}, ret)

	for _, item := range r0 {
		data = append(data, bson.M{item["_id"].(string): item["count"]})
	}
	_, err = me.Client.SaveJob(struct {
		Collection string
		Data       bson.M
	}{Collection: "Holders", Data: bson.M{"Holders": data}})
	if err != nil {
		return err
	}
	return nil
}
