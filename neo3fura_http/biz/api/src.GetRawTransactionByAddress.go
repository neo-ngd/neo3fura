package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetRawTransactionByAddress(args struct {
	Address h160.T
	Limit   int64
	Skip    int64
	Cursor  string
	Filter  map[string]interface{}
}, ret *json.RawMessage) error {
	if args.Address.Valid() == false {
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
		Index:        "GetRawTransactionByAddress",
		Sort:         bson.M{"_id": -1},
		Filter:       bson.M{"sender": args.Address.TransferAddress()},
		Query:        []string{},
		Limit:        queryLimit,
		Skip:         args.Skip,
		CursorFilter: cursorFilter,
	}, ret)
	if err != nil {
		return err
	}
	hasNext := int64(len(r1)) > args.Limit
	page := r1
	if hasNext {
		page = r1[:args.Limit]
	}

	txIDs := make([]string, 0, len(page))
	for _, item := range page {
		hash, ok := item["hash"].(string)
		if ok && hash != "" {
			txIDs = append(txIDs, hash)
		}
	}

	vmstates, err := me.loadExecutionStates(txIDs)
	if err != nil {
		return err
	}

	faultTxIDs := make([]string, 0, len(page))
	for _, item := range page {
		hash, _ := item["hash"].(string)
		vmstate := vmstates[hash]
		if vmstate == "" {
			vmstate = "FAULT"
		}
		item["vmstate"] = vmstate
		if vmstate == "FAULT" && hash != "" {
			faultTxIDs = append(faultTxIDs, hash)
		}
	}

	faultDetails, err := me.loadTransferEvents(faultTxIDs)
	if err != nil {
		return err
	}
	for _, item := range page {
		hash, _ := item["hash"].(string)
		if item["vmstate"] == "FAULT" {
			item["faultdetail"] = faultDetails[hash]
		}
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
