package home

import (
	"context"
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	log2 "neo3fura_ws/lib/log"
)

// Asset
func (me *T) GetAssetCount(ch *chan map[string]interface{}) error {
	assetCount, err := me.getAssetCount()
	if err != nil {
		return err
	}
	*ch <- assetCount

	c, err := me.Client.GetCollection(struct{ Collection string }{Collection: "Asset"})
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
			log2.Errorf("GetAssetCount decode change event failed: %v", err)
			continue
		}
		newAssetCount, err := me.getAssetCount()
		if err != nil {
			return err
		}
		oldTotal, okOld := extractTotalCounts(assetCount["AssetCount"])
		newTotal, okNew := extractTotalCounts(newAssetCount["AssetCount"])
		if !okOld || !okNew || oldTotal != newTotal {
			*ch <- newAssetCount
			assetCount = newAssetCount
		}
	}
	return cs.Err()
}

func (me T) getAssetCount() (map[string]interface{}, error) {
	message := make(json.RawMessage, 0)
	ret := &message
	res := make(map[string]interface{})

	r1, err := me.Client.QueryDocument(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
	}{
		Collection: "Asset",
		Index:      "GetAssetCount",
		Sort:       bson.M{},
		Filter:     bson.M{},
	}, ret)
	if err != nil {
		return nil, err
	}
	res["AssetCount"] = r1
	return res, nil
}
