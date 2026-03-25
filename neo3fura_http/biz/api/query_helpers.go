package api

import (
	"encoding/json"
	"math/big"

	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (me *T) getAssetByContractHash(contractHash h160.T, ret *json.RawMessage) (map[string]interface{}, error) {
	return me.Client.QueryOne(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
	}{
		Collection: "Asset",
		Index:      "GetAssetInfoByContractHash",
		Sort:       bson.M{},
		Filter:     bson.M{"hash": contractHash.Val(), "totalsupply": bson.M{"$gt": 0}},
		Query:      []string{},
	}, ret)
}

func (me *T) getAssetTotalSupply(contractHash h160.T, ret *json.RawMessage) (*big.Int, error) {
	asset, err := me.getAssetByContractHash(contractHash, ret)
	if err != nil {
		return nil, err
	}
	totalSupply, ok := asBigInt(asset["totalsupply"])
	if !ok {
		return nil, stderr.ErrData
	}
	return totalSupply, nil
}

func (me *T) getAssetHolderCount(contractHash h160.T, assetType string, ret *json.RawMessage) (int64, error) {
	if assetType == "NEP11" {
		r3, err := me.Client.QueryAggregate(
			struct {
				Collection string
				Index      string
				Sort       bson.M
				Filter     bson.M
				Pipeline   []bson.M
				Query      []string
			}{
				Collection: "Address-Asset",
				Index:      "GetContractList",
				Sort:       bson.M{},
				Filter:     bson.M{},
				Pipeline: []bson.M{
					bson.M{"$match": bson.M{"asset": contractHash.Val(), "balance": bson.M{"$gt": 0}}},
					bson.M{"$group": bson.M{"_id": "$address"}},
					bson.M{"$count": "addressCounts"},
				},
				Query: []string{},
			}, ret)
		if err != nil {
			return 0, err
		}
		if len(r3) == 0 {
			return 0, nil
		}
		count, ok := asInt64(r3[0]["addressCounts"])
		if !ok {
			return 0, stderr.ErrData
		}
		return count, nil
	}

	count, err := me.Client.QueryDocument(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
	}{
		Collection: "Address-Asset",
		Index:      "GetAssetInfos",
		Sort:       bson.M{},
		Filter:     bson.M{"asset": contractHash.Val(), "balance": bson.M{"$gt": 0}},
	}, ret)
	if err != nil {
		return 0, err
	}
	holderCount, ok := asInt64(count["total counts"])
	if !ok {
		return 0, stderr.ErrData
	}
	return holderCount, nil
}

func (me *T) loadExecutionStates(txIDs []string) (map[string]string, error) {
	states := make(map[string]string, len(txIDs))
	if len(txIDs) == 0 {
		return states, nil
	}

	collection := me.Client.C_online.Database(me.Client.Db_online).Collection("Execution")
	cursor, err := collection.Find(
		me.Client.Ctx,
		bson.M{"txid": bson.M{"$in": txIDs}},
		options.Find().SetProjection(bson.M{"txid": 1, "vmstate": 1}),
	)
	if err != nil {
		return nil, stderr.ErrFind
	}
	defer cursor.Close(me.Client.Ctx)

	var docs []map[string]interface{}
	if err := cursor.All(me.Client.Ctx, &docs); err != nil {
		return nil, stderr.ErrFind
	}
	for _, doc := range docs {
		txid, _ := doc["txid"].(string)
		vmstate, _ := doc["vmstate"].(string)
		if txid != "" {
			states[txid] = vmstate
		}
	}
	return states, nil
}

func normalizeTransferEvent(doc map[string]interface{}) map[string]interface{} {
	if len(doc) == 0 {
		return doc
	}

	arr, ok := doc["hexStringParams"].(primitive.A)
	if !ok || len(arr) == 0 {
		return doc
	}

	reversed := make([]string, 0, len(arr))
	for _, item := range arr {
		value, ok := item.(string)
		if !ok {
			continue
		}
		reversed = append(reversed, strval.T(value).Reverse())
	}
	doc["hexStringParams"] = reversed
	return doc
}

func (me *T) loadTransferEvents(txIDs []string) (map[string]map[string]interface{}, error) {
	events := make(map[string]map[string]interface{}, len(txIDs))
	if len(txIDs) == 0 {
		return events, nil
	}

	collection := me.Client.C_local.Database("job").Collection("TransferEvent")
	cursor, err := collection.Find(me.Client.Ctx, bson.M{"txid": bson.M{"$in": txIDs}})
	if err != nil {
		return nil, stderr.ErrFind
	}
	defer cursor.Close(me.Client.Ctx)

	var docs []map[string]interface{}
	if err := cursor.All(me.Client.Ctx, &docs); err != nil {
		return nil, stderr.ErrFind
	}
	for _, doc := range docs {
		txid, _ := doc["txid"].(string)
		if txid == "" {
			continue
		}
		events[txid] = normalizeTransferEvent(doc)
	}
	return events, nil
}
