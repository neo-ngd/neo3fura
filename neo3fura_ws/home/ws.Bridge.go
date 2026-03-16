package home

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	log2 "neo3fura_ws/lib/log"
)

// bridge
func (me *T) GetBridge(contract string, nonce int32, ch *chan map[string]interface{}) error {
	c, err := me.Client.GetCollection(struct{ Collection string }{Collection: "Notification"})
	if err != nil {
		return err
	}

	//matchStage := bson.D{
	//	{"$match", bson.D{
	//		{"operationType", "insert"},
	//		//{"fullDocument.index", bson.D{
	//		//	{"index", 1},
	//		//}},
	//	}},
	//}
	//cs, err := c.Watch(context.TODO(), mongo.Pipeline{matchStage})   //需要开启副本集模式

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
			log2.Errorf("GetBridge decode change event failed: %v", err)
			continue
		}
		fullDocument, ok := asMap(changeEvent["fullDocument"])
		if !ok {
			continue
		}
		contractAdd, _ := asString(fullDocument["contract"])
		eventName, _ := asString(fullDocument["eventname"])
		if contractAdd == contract {
			if eventName == "Withdrawal" || eventName == "Claimable" {
				state, ok := asMap(fullDocument["state"])
				if !ok {
					continue
				}
				stateValue, ok := asPrimitiveA(state["value"])
				if !ok || len(stateValue) == 0 {
					continue
				}
				event, ok := asMap(stateValue[0])
				if !ok {
					continue
				}
				eventNonce, ok := asInt32(event["value"])
				if !ok {
					continue
				}
				if nonce == eventNonce {
					*ch <- fullDocument
				}

			}
		}

	}
	return cs.Err()
}
