package home

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	log2 "neo3fura_ws/lib/log"
)

// Asset
func (me *T) GetBlockCount(ch *chan map[string]interface{}) error {
	blockCount := make(map[string]interface{})
	totalCounts := make(map[string]interface{})
	lastestBlock, err := me.Client.QueryLastOne(struct{ Collection string }{Collection: "Block"})
	if err != nil {
		return err
	}
	totalCounts["total counts"] = lastestBlock["index"]
	blockCount["BlockCount"] = totalCounts
	*ch <- blockCount
	c, err := me.Client.GetCollection(struct{ Collection string }{Collection: "Block"})
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
			log2.Errorf("GetBlockCount decode change event failed: %v", err)
			continue
		}
		fullDocument, ok := asMap(changeEvent["fullDocument"])
		if !ok {
			continue
		}
		index, ok := asInt64(fullDocument["index"])
		if !ok {
			continue
		}
		newBlockCount := make(map[string]interface{})
		newTotalCounts := make(map[string]interface{})
		newTotalCounts["total counts"] = index
		newBlockCount["BlockCount"] = newTotalCounts
		oldTotal, okOld := extractTotalCounts(blockCount["BlockCount"])
		newTotal, okNew := extractTotalCounts(newBlockCount["BlockCount"])
		if !okOld || !okNew || oldTotal != newTotal {
			*ch <- newBlockCount
			blockCount = newBlockCount
		}
	}
	return cs.Err()
}
