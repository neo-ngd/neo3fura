package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

// this function may be not supported any more, we only support address in the formart of script hash
func (me *T) GetScCallByContractHashAddress(args struct {
	ContractHash h160.T
	Address      h160.T
	Limit        int64
	Skip         int64
	Cursor       string
	Filter       map[string]interface{}
}, ret *json.RawMessage) error {
	if args.ContractHash.Valid() == false {
		return stderr.ErrInvalidArgs
	}
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
		Collection:   "ScCall",
		Index:        "GetScCallByContractHashAddress",
		Sort:         bson.M{},
		Filter:       bson.M{"contractHash": args.ContractHash.Val(), "originSender": args.Address.TransferredVal()},
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
