package api

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"

	"neo3fura_http/lib/type/h160"
)

func (me *T) GetTransferTxByAddressAsset(args struct {
	FromAddress h160.T
	ToAddress   h160.T
	Asset       h160.T
	StartTime   uint64
	EndTime     uint64
	Limit       int64
	Skip        int64
	Filter      map[string]interface{}
}, ret *json.RawMessage) error {
	f := bson.M{}
	if args.FromAddress.Valid() {
		f["from"] = args.FromAddress.Val()
	}
	if args.ToAddress.Valid() {
		f["to"] = args.ToAddress.Val()
	}

	if args.Asset.Valid() {
		f["contract"] = args.Asset.Val()
	}

	if args.StartTime <= args.EndTime && args.EndTime > 0 {
		f["timestamp"] = bson.M{"$gte": args.StartTime, "$lte": args.EndTime}
	}

	if args.Limit == 0 {
		args.Limit = 512
	}
	r1, count, err1 := me.Client.QueryAll(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
		Limit      int64
		Skip       int64
	}{
		Collection: "TransferNotification",
		Index:      "TransferNotification",
		Sort:       bson.M{},
		Filter:     f,
		Query:      []string{},
	}, ret)
	if err1 != nil {
		return err1
	}

	r5, err := me.FilterArrayAndAppendCount(r1, count, args.Filter)
	if err != nil {
		return err
	}
	r, err := json.Marshal(r5)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
