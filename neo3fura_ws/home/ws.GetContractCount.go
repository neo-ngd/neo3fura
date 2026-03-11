package home

import (
	"context"
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	log2 "neo3fura_ws/lib/log"
)

// Address
func (me *T) GetContractCount(ch *chan map[string]interface{}) error {
	contractCount, err := me.getContractCount()
	if err != nil {
		return err
	}
	*ch <- contractCount

	c, err := me.Client.GetCollection(struct{ Collection string }{Collection: "Contract"})
	if err != nil {
		return err
	}
	cs, err := c.Watch(context.TODO(), mongo.Pipeline{})
	if err != nil {
		return err
	}
	defer cs.Close(context.TODO())
	// Whenever there is a new change event, decode the change event and print some information about it
	for cs.Next(context.TODO()) {
		var changeEvent map[string]interface{}
		err := cs.Decode(&changeEvent)
		if err != nil {
			log2.Errorf("GetContractCount decode change event failed: %v", err)
			continue
		}
		newContractCount, err := me.getContractCount()
		if err != nil {
			return err
		}
		oldTotal, okOld := extractTotalCounts(contractCount["ContractCount"])
		newTotal, okNew := extractTotalCounts(newContractCount["ContractCount"])
		if !okOld || !okNew || oldTotal != newTotal {
			*ch <- newContractCount
			contractCount = newContractCount
		}
	}
	return cs.Err()
}

func (me T) getContractCount() (map[string]interface{}, error) {
	message := make(json.RawMessage, 0)
	ret := &message
	res := make(map[string]interface{})
	r1, err := me.Client.GetDistinctCount(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Pipeline   []bson.M
	}{
		Collection: "Contract",
		Index:      "GetContractCount",
		Sort:       bson.M{},
		Filter:     bson.M{},
	}, ret)
	if err != nil {
		return nil, err
	}
	res["ContractCount"] = r1
	return res, nil
}
