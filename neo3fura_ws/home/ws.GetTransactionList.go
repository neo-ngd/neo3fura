package home

import (
	"context"
	"encoding/json"
	log2 "neo3fura_ws/lib/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// TransactionList
func (me *T) GetTransactionList(ch *chan map[string]interface{}) error {
	transactionList, err := me.getTransactionList2()
	if err != nil {
		return err
	}
	*ch <- transactionList

	c, err := me.Client.GetCollection(struct{ Collection string }{Collection: "Transaction"})
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
			log2.Errorf("GetTransactionList decode change event failed: %v", err)
			continue
		}
		newTransactionList, err := me.getTransactionList2()
		if err != nil {
			return err
		}
		oldHash, okOld := extractFirstHash(transactionList["TransactionList"])
		newHash, okNew := extractFirstHash(newTransactionList["TransactionList"])
		if okOld && okNew && oldHash == newHash {
			*ch <- newTransactionList
			transactionList = newTransactionList
		}
	}
	return cs.Err()
}

func (me T) getTransactionList() (map[string]interface{}, error) {
	message := make(json.RawMessage, 0)
	ret := &message
	res := make(map[string]interface{})
	r1, _, err := me.Client.QueryAll(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
		Limit      int64
		Skip       int64
	}{
		Collection: "Transaction",
		Index:      "GetTransactionList",
		Sort:       bson.M{"blocktime": -1},
		Filter:     bson.M{},
		Query:      []string{},
		Limit:      10,
		Skip:       0,
	}, ret)
	if err != nil {
		return nil, err
	}
	res["TransactionList"] = r1
	return res, nil
}

func (me T) getTransactionList2() (map[string]interface{}, error) {
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
		}{Collection: "Transaction",
			Index:  "GetTransactionList",
			Sort:   bson.M{},
			Filter: bson.M{},
			Pipeline: []bson.M{
				bson.M{"$sort": bson.M{"blocktime": -1}},
				bson.M{"$project": bson.M{"_id": 1, "size": 1, "blocktime": 1, "hash": 1, "sysfee": 1, "netfee": 1}},
				bson.M{"$limit": 10},
			},
			Query: []string{}}, ret)

	if err != nil {
		return nil, err
	}
	res["TransactionList"] = r1
	return res, nil
}
