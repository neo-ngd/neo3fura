package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (me *T) GetTransactionList(args struct {
	Limit  int64
	Skip   int64
	Cursor string
	Filter map[string]interface{}
}, ret *json.RawMessage) error {
	if args.Limit <= 0 {
		args.Limit = consts.DefaultLimit
	}

	sortKeys := []string{"blocktime"}
	sortDirs := map[string]int{"blocktime": -1}

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
		Index:        "GetTransactionList",
		Sort:         bson.M{"blocktime": -1},
		Filter:       bson.M{},
		Query:        []string{},
		Limit:        args.Limit,
		Skip:         args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
	}
	r2, err := me.FilterArrayAndAppendCountWithCursor(r1, count, args.Filter, sortKeys)
	if err != nil {
		return err
	}
	if hasNext {
		last := page[len(page)-1]
		oid, ok := last["_id"].(primitive.ObjectID)
		if !ok {
			return stderr.ErrInvalidArgs
		}
		nextCursor, err := encodeOIDCursor(oid)
		if err != nil {
			return err
		}
		r2["nextCursor"] = nextCursor
	}
	r, err := json.Marshal(r2)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
