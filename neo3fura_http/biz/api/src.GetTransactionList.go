package api

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetTransactionList(args struct {
	Limit  int64
	Skip   int64
	Cursor string
	Filter map[string]interface{}
}, ret *json.RawMessage) error {
	if args.Limit == 0 {
		args.Limit = 512
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
	r, err := json.Marshal(r2)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
