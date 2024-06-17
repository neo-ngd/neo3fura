package job

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"

	"neo3fura_http/lib/type/consts"
)

func (me T) GetActiveAddresses() error {
	message := make(json.RawMessage, 0)
	ret := &message

	r0, err := me.Client.QueryOne(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Query      []string
		}{Collection: "Transaction", Index: "GetActiveAddresses", Sort: bson.M{"_id": -1}}, ret)
	if err != nil {
		return err
	}
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
		Index:      "GetActiveAddresses",
		Sort:       bson.M{},
		Filter:     bson.M{"blocktime": bson.M{"$gt": r0["blocktime"].(int64) - 3600*24*1000}},
		Query:      []string{},
		Limit:      consts.MaxLimit,
	}, ret)
	if err != nil {
		return err
	}
	r2 := make(map[string]interface{})
	for _, item := range r1 {
		r2[item["sender"].(string)] = true
	}

	data := bson.M{"ActiveAddresses": len(r2), "insertTime": r0["blocktime"]}
	_, err = me.Client.SaveJob(struct {
		Collection string
		Data       bson.M
	}{Collection: "ActiveAddresses", Data: data})
	if err != nil {
		return err
	}
	return nil
}
