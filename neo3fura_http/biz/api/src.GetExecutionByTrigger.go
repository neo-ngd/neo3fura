package api

import (
	"encoding/json"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetExecutionByTrigger(args struct {
	Trigger strval.T
	Limit   int64
	Skip    int64
		Cursor      string
	Filter  map[string]interface{}
}, ret *json.RawMessage) error {
	if args.Limit == 0 {
		args.Limit = 512
	}
	in := args.Trigger.In([]string{"OnPersist", "PostPersist", "Application", "Verification", "System", "All"})
	if in == false {
		return stderr.ErrInvalidArgs
	}
	var filter bson.M
	if args.Trigger.Val() == "All" {
		filter = bson.M{"$or": []interface{}{
			bson.M{"trigger": "OnPersist"},
			bson.M{"trigger": "PostPersist"},
			bson.M{"trigger": "Application"},
			bson.M{"trigger": "Verification"},
		}}
	} else if args.Trigger.Val() == "System" {
		filter = bson.M{"$or": []interface{}{
			bson.M{"trigger": "OnPersist"},
			bson.M{"trigger": "PostPersist"},
			bson.M{"trigger": "Application"},
			bson.M{"trigger": "Verification"},
		}}
	} else {
		filter = bson.M{
			"trigger": args.Trigger.Val(),
		}
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
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
		Limit      int64
		Skip       int64
		CursorFilter bson.M
	}{
		Collection: "Execution",
		Index:      "GetExecutionByTrigger",
		Sort:       bson.M{},
		Filter:     filter,
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
