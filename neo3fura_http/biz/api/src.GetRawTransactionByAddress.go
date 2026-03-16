package api

import (
	"encoding/json"
	"fmt"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/h256"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (me *T) GetRawTransactionByAddress(args struct {
	Address h160.T
	Limit   int64
	Skip    int64
	Cursor  string
	Filter  map[string]interface{}
}, ret *json.RawMessage) error {
	if args.Address.Valid() == false {
		return stderr.ErrInvalidArgs
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

	r1, count, err := me.Client.QueryAllWithCursor(struct {
		Collection   string
		Index        string
		Sort         bson.M
		Filter       bson.M
		Query        []string
		Limit        int64
		Skip         int64
		CursorFilter bson.M
	}{
		Collection:   "Transaction",
		Index:        "GetRawTransactionByAddress",
		Sort:         bson.M{"_id": -1},
		Filter:       bson.M{"sender": args.Address.TransferAddress()},
		Query:        []string{},
		Limit:        args.Limit,
		Skip:         args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
	}
	var raw1 map[string]interface{}
	for _, item := range r1 {
		err = me.GetVmStateByTransactionHash(struct {
			TransactionHash h256.T
			Filter          map[string]interface{}
			Raw             *map[string]interface{}
		}{
			TransactionHash: h256.T(fmt.Sprint(item["hash"])),
			Filter:          nil,
			Raw:             &raw1,
		}, ret)
		if err != nil {
			return err
		}
		item["vmstate"] = raw1["vmstate"].(string)
		if raw1["vmstate"].(string) == "FAULT" {
			var raw2 map[string]interface{}
			err = me.GetTransferEventByTransactionHash(struct {
				TransactionHash h256.T
				Filter          map[string]interface{}
				Raw             *map[string]interface{}
			}{TransactionHash: h256.T(fmt.Sprint(item["hash"])), Filter: nil, Raw: &raw2}, ret)
			if err == mongo.ErrNoDocuments {
				item["faultdetail"] = nil
			}
			if err != nil && err != mongo.ErrNoDocuments {
				return err
			}
			item["faultdetail"] = raw2
		}
	}

	r2, err := me.FilterArrayAndAppendCountWithCursor(r1, count, args.Filter, sortKeys)
	if err != nil {
		return err
	}
	r, err := json.Marshal(r2)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
