package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/h256"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetScCallByTransactionHash(args struct {
	TransactionHash h256.T
	Filter          map[string]interface{}
	Skip            int64
	Limit           int64
	Cursor          string
}, ret *json.RawMessage) error {
	if args.TransactionHash.Valid() == false {
		return stderr.ErrInvalidArgs
	}
	sortKeys := []string{"_id"}
	sortDirs := map[string]int{"_id": 1}

	var cursorFilter bson.M
	if args.Cursor != "" {
		cursorValues, err := DecodeCursor(args.Cursor)
		if err != nil {
			return err
		}
		cursorFilter = BuildCursorFilter(sortKeys, sortDirs, cursorValues)
	}

	r1, count, err := me.Client.QueryAllWithCursor(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
		Limit      int64
		Skip       int64
		CursorFilter bson.M
	}{
		Collection: "ScCall",
		Index:      "GetScCallByTransactionHash",
		Sort:       bson.M{"_id": 1},
		Filter:     bson.M{"txid": args.TransactionHash.Val()},
		Query:      []string{},
		Limit:      args.Limit,
		Skip:       args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
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
