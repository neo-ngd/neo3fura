package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetNep17TransferByContractHash(args struct {
	ContractHash h160.T
	Limit        int64
	Skip         int64
	Cursor       string
	Filter       map[string]interface{}
}, ret *json.RawMessage) error {
	if args.ContractHash.Valid() == false {
		return stderr.ErrInvalidArgs
	}

	sortKeys := []string{"_id"}
	sortDirs := map[string]int{"_id": -1}
	queryLimit := args.Limit + 1

	var cursorFilter bson.M
	if args.Cursor != "" {
		cursorValues, err := DecodeCursor(args.Cursor)
		if err != nil {
			return err
		}
		cursorFilter = BuildCursorFilter(sortKeys, sortDirs, cursorValues)
	}

	filter := bson.M{"contract": args.ContractHash.Val()}

	r1, err := me.Client.QueryAllWithCursorNoCount(struct {
		Collection   string
		Index        string
		Sort         bson.M
		Filter       bson.M
		Query        []string
		Limit        int64
		Skip         int64
		CursorFilter bson.M
	}{
		Collection:   "TransferNotification",
		Index:        "GetNep17TransferByContractHash",
		Sort:         bson.M{"_id": -1},
		Filter:       filter,
		Query:        []string{},
		Limit:        queryLimit,
		Skip:         args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
	}
	count, ok := me.Client.CachedDocumentCount(struct {
		Collection string
		Index      string
		Filter     bson.M
	}{
		Collection: "TransferNotification",
		Index:      "GetNep17TransferByContractHash",
		Filter:     filter,
	})
	if !ok {
		count = -1
	}

	hasNext := int64(len(r1)) > args.Limit
	page := r1
	if hasNext {
		page = r1[:args.Limit]
	}

	txIDs := make([]string, 0, len(page))
	for _, item := range page {
		txid, ok := item["txid"].(string)
		if ok && txid != "" {
			txIDs = append(txIDs, txid)
		}
	}

	vmstates, err := me.loadExecutionStates(txIDs)
	if err != nil {
		return err
	}
	for _, item := range page {
		txid, _ := item["txid"].(string)
		vmstate := vmstates[txid]
		if vmstate == "" {
			vmstate = "FAULT"
		}
		item["vmstate"] = vmstate
	}

	r2, err := me.FilterArrayAndAppendCount(page, count, args.Filter)
	if err != nil {
		return err
	}
	if hasNext {
		last := page[len(page)-1]
		nextCursor := EncodeCursor(last, sortKeys)
		if nextCursor == "" {
			return stderr.ErrInvalidArgs
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
