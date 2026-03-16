package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (me *T) GetNep11TransferByContractHashTokenId(args struct {
	ContractHash h160.T
	Limit        int64
	Skip         int64
	Cursor       string
	TokenId      strval.T
	Filter       map[string]interface{}
	Raw          *[]map[string]interface{}
}, ret *json.RawMessage) error {
	if args.ContractHash.Valid() == false {
		return stderr.ErrInvalidArgs
	}
	if args.Limit <= 0 {
		args.Limit = consts.DefaultLimit
	}
	if args.Limit > consts.MaxLimit {
		args.Limit = consts.MaxLimit
	}
	if args.Skip < 0 {
		args.Skip = 0
	}
	var f bson.M
	if args.TokenId == "" {
		f = bson.M{"contract": args.ContractHash.Val()}
	} else {
		f = bson.M{"contract": args.ContractHash.Val(), "tokenId": args.TokenId}
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
		Index:        "GetNep11TransferByAddress",
		Sort:         bson.M{"_id": -1},
		Filter:       f,
		Query:        []string{},
		Limit:        args.Limit,
		Skip:         args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	hasNext := int64(len(r1)) > args.Limit
	page := r1
	if hasNext {
		page = r1[:args.Limit]
	}

	if args.Raw != nil {
		*args.Raw = page
	}
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
