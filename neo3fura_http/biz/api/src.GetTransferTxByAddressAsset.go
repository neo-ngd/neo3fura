package api

import (
	"encoding/json"
	"errors"

	"go.mongodb.org/mongo-driver/bson"

	"neo3fura_http/lib/type/h160"
)

func (me *T) GetTransferTxByAddressAsset(args struct {
	FromAddress h160.T
	//	ToAddress   h160.T
	//Asset       h160.T
	//StartTime uint64
	//EndTime   uint64
	Limit  int64
	Skip   int64
		Cursor      string
	Filter map[string]interface{}
}, ret *json.RawMessage) error {
	f := bson.M{}
	if !args.FromAddress.Valid() {
		return errors.New("invalid fromAddress")
	}
	f["from"] = args.FromAddress.Val()
	f["to"] = "0x472c36c9e51bc7d3906e48182c2213539a4728d5"
	f["contract"] = "0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5"

	f["$and"] = []interface{}{
		bson.M{"timestamp": bson.M{"$gte": 1743825600000}},
		bson.M{"timestamp": bson.M{"$lte": 1744430400000}},
	}

	sortKeys := []string{"_id"}
	sortDirs := map[string]int{"_id": -1}

	var cursorFilter bson.M
	if args.Cursor != "" {
		cursorValues, err := DecodeCursor(args.Cursor)
		if err != nil {
			return err
		}
		cursorFilter = BuildCursorFilter(sortKeys, sortDirs, cursorValues)
	}

	r1, count, err1 := me.Client.QueryAllWithCursor(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
		Limit      int64
		Skip       int64
		CursorFilter bson.M
	}{
		Collection: "TransferNotification",
		Index:      "TransferNotification",
		Sort:       bson.M{},
		Filter:     f,
		Query:      []string{"from", "to", "value", "timestamp", "txid"},
		Limit:      args.Limit,
		Skip:       args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	if err1 != nil {
		return err1
	}

	r5, err := me.FilterArrayAndAppendCountWithCursor(r1, count, args.Filter, sortKeys)
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
