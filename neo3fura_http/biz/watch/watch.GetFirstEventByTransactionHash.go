package watch

import (
	"context"
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	log2 "neo3fura_http/lib/log"
)

func (me *T) GetFirstEventByTransactionHash() error {
	message := make(json.RawMessage, 0)
	ret := &message
	c := me.Client.C_online.Database(me.Client.Db_online).Collection("Transaction")
	cs, err := c.Watch(context.TODO(), mongo.Pipeline{})
	if err != nil {
		return err
	}
	defer cs.Close(context.TODO())
	// Whenever there is a new change event, decode the change event and print some information about it
	for cs.Next(context.TODO()) {
		log2.Infof("Detect New Transaction, Processing")
		var changeEvent map[string]interface{}
		err := cs.Decode(&changeEvent)
		if err != nil {
			log2.Errorf("Decode error:%v", err)
			continue
		}
		fullDocument, ok := changeEvent["fullDocument"].(map[string]interface{})
		if !ok {
			log2.Errorf("Invalid change event: fullDocument missing")
			continue
		}
		txid, ok := fullDocument["hash"]
		if !ok || txid == nil {
			log2.Errorf("Invalid change event: transaction hash missing")
			continue
		}

		r2, err := me.Client.QueryOne(struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Query      []string
		}{
			Collection: "ScCall",
			Index:      "GetFirstEventByTransactionHash",
			Sort:       bson.M{"_id": 1},
			Filter:     bson.M{"txid": txid},
			Query:      []string{},
		}, ret)
		if err != nil {
			log2.Errorf("Query ScCall error:%v", err)
			continue
		}

		r1, err := me.Client.QueryOne(struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Query      []string
		}{
			Collection: "Execution",
			Index:      "GetFirstEventByTransactionHash",
			Sort:       bson.M{},
			Filter:     bson.M{"txid": txid},
			Query:      []string{"vmstate"},
		}, ret)
		if err != nil {
			log2.Errorf("Query Execution error:%v", err)
			continue
		}
		if len(r1) > 0 && len(r2) > 0 && r1["vmstate"] != nil && r1["vmstate"] != "" {
			r2["vmstate"] = r1["vmstate"]
			_, err = me.Client.SaveJob(struct {
				Collection string
				Data       bson.M
			}{Collection: "TransferEvent", Data: r2})
			if err != nil {
				log2.Errorf("Save Job Error: %v", err)
			}
		}

	}
	return cs.Err()
}
