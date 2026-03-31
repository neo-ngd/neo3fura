package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetNotificationByContractHash(args struct {
	ContractHash h160.T
	Limit        int64
	Skip         int64
	Cursor       string
	Filter       map[string]interface{}
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

	sortKeys := []string{"_id"}
	sortDirs := map[string]int{"_id": -1}
	baseFilter := bson.M{"contract": args.ContractHash.Val()}

	// Cursor page: skip CountDocuments, use QueryFind + hasNext pattern
	if args.Cursor != "" {
		cursorValues, err := DecodeCursor(args.Cursor)
		if err != nil {
			return err
		}
		cf := BuildCursorFilter(sortKeys, sortDirs, cursorValues)
		queryFilter := MergeCursorFilter(baseFilter, cf)

		queryLimit := args.Limit + 1
		r1, err := me.Client.QueryFind(struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Query      []string
			Limit      int64
		}{
			Collection: "Notification",
			Index:      "GetNotificationByContractHash",
			Sort:       bson.M{"_id": -1},
			Filter:     queryFilter,
			Query:      []string{},
			Limit:      queryLimit,
		}, ret)
		if err != nil {
			return err
		}

		hasNext := int64(len(r1)) > args.Limit
		page := r1
		if hasNext {
			page = r1[:args.Limit]
		}

		r2, err := me.FilterArrayAndAppendCount(page, 0, args.Filter)
		if err != nil {
			return err
		}
		if hasNext {
			r2["nextCursor"] = EncodeCursor(page[len(page)-1], sortKeys)
		}
		r, err := json.Marshal(r2)
		if err != nil {
			return err
		}
		*ret = json.RawMessage(r)
		return nil
	}

	// First page: use QueryAllWithCursor to get total count + data
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
		Collection:   "Notification",
		Index:        "GetNotificationByContractHash",
		Sort:         bson.M{"_id": -1},
		Filter:       baseFilter,
		Query:        []string{},
		Limit:        args.Limit + 1,
		Skip:         args.Skip,
		CursorFilter: nil,
	}, ret)
	if err != nil {
		return err
	}

	hasNext := int64(len(r1)) > args.Limit
	page := r1
	if hasNext {
		page = r1[:args.Limit]
	}

	r2, err := me.FilterArrayAndAppendCount(page, count, args.Filter)
	if err != nil {
		return err
	}
	if hasNext {
		r2["nextCursor"] = EncodeCursor(page[len(page)-1], sortKeys)
	}
	r, err := json.Marshal(r2)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
