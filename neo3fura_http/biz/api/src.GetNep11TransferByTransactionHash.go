package api

import (
	"encoding/json"
	"errors"
	"neo3fura_http/lib/type/h256"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetNep11TransferByTransactionHash(args struct {
	TransactionHash h256.T
	Limit           int64
	Skip            int64
	Cursor          string
	Filter          map[string]interface{}
}, ret *json.RawMessage) error {
	if args.TransactionHash.Valid() == false {
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
		Collection:   "Nep11TransferNotification",
		Index:        "GetNep11TransferByTransactionHash",
		Sort:         bson.M{},
		Filter:       bson.M{"txid": args.TransactionHash.Val()},
		Query:        []string{},
		Limit:        args.Limit,
		Skip:         args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
	}

	for _, item := range r1 {
		r, err := me.Client.QueryOne(struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Query      []string
		}{
			Collection: "Asset",
			Index:      "GetNep11TransferByTransactionHash",
			Sort:       bson.M{},
			Filter:     bson.M{"hash": item["contract"]},
			Query:      []string{"tokenname", "decimals", "symbol"},
		}, ret)
		if err == nil {
			item["tokenname"] = r["tokenname"]
			item["decimals"] = r["decimals"]
			item["symbol"] = r["symbol"]

		} else if errors.Is(err, stderr.ErrNotFound) {
			item["tokenname"] = ""
			item["decimals"] = ""
			item["symbol"] = ""
		} else {
			return err
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
