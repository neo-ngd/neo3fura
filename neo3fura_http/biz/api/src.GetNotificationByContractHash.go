package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	if args.Skip < 0 {
		args.Skip = 0
	}
	filter := bson.M{"contract": args.ContractHash.Val()}
	if args.Cursor != "" {
		cursorFilter, err := buildOIDDescCursorFilter(args.Cursor)
		if err != nil {
			return err
		}
		filter = bson.M{
			"$and": []interface{}{
				filter,
				cursorFilter,
			},
		}
		args.Skip = 0
	}
	queryLimit := args.Limit + 1
	r1, count, err := me.Client.QueryAll(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
		Limit      int64
		Skip       int64
	}{
		Collection: "Notification",
		Index:      "GetNotificationByContractHash",
		Sort:       bson.M{"_id": -1},
		Filter:     filter,
		Query:      []string{},
		Limit:      queryLimit,
		Skip:       args.Skip,
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
