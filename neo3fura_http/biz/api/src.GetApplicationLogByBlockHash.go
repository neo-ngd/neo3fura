package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/h256"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetApplicationLogByBlockHash(args struct {
	BlockHash h256.T
	Limit     int64
	Skip      int64
	Cursor    string
	Filter    map[string]interface{}
}, ret *json.RawMessage) error {
	if args.BlockHash.Valid() == false {
		return stderr.ErrInvalidArgs
	}
	if args.BlockHash.IsZero() == true {
		return stderr.ErrZero
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
		Collection:   "Execution",
		Index:        "GetApplicationLogByBlockHash",
		Sort:         bson.M{},
		Filter:       bson.M{"blockhash": args.BlockHash.Val()},
		Query:        []string{},
		Limit:        args.Limit,
		Skip:         args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
	}
	for _, item := range r1 {
		r2, _, err := me.Client.QueryAllWithCursor(struct {
			Collection   string
			Index        string
			Sort         bson.M
			Filter       bson.M
			Query        []string
			Limit        int64
			Skip         int64
			CursorFilter bson.M
		}{
			Collection:   "Notification",
			Index:        "GetApplicationLogByBlockHash",
			Sort:         bson.M{},
			Filter:       bson.M{"txid": item["txid"].(string), "blockhash": item["blockhash"].(string)},
			CursorFilter: nil,
		}, ret)
		if err != nil {
			return err
		}
		item["notifications"] = r2
	}
	r3, err := me.FilterArrayAndAppendCountWithCursor(r1, count, args.Filter, sortKeys)
	if err != nil {
		return nil
	}
	r, err := json.Marshal(r3)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
