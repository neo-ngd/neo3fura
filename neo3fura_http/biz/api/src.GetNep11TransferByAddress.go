package api

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
)

func (me *T) GetNep11TransferByAddress(args struct {
	Address h160.T
	Limit   int64
	Skip    int64
	Start   int64
	End     int64
	Filter  map[string]interface{}
	Raw     *[]map[string]interface{}
}, ret *json.RawMessage) error {
	if args.Address.Valid() == false {
		return stderr.ErrInvalidArgs
	}
	filter := bson.M{"$or": []interface{}{
		bson.M{"from": args.Address.TransferredVal()},
		bson.M{"to": args.Address.TransferredVal()},
	}}

	if args.Start > 0 && args.End > 0 {
		if args.Start >= args.End {
			return stderr.ErrArgsInner
		}
		filter["$and"] = []interface{}{
			bson.M{"timestamp": bson.M{"$gte": args.Start}},
			bson.M{"timestamp": bson.M{"$lte": args.End}},
		}

	} else if args.Start > 0 && args.End == 0 {
		filter["timestamp"] = bson.M{"$gte": args.Start}
	} else if args.Start == 0 && args.End > 0 {
		filter["timestamp"] = bson.M{"$lte": args.Start}

	}

	r1, err := me.Client.QueryAggregate(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Pipeline   []bson.M
		Query      []string
	}{
		Collection: "Nep11TransferNotification",
		Index:      "GetNep17TransferByContractHash",
		Sort:       bson.M{},
		Filter:     bson.M{},
		Pipeline: []bson.M{
			bson.M{"$match": filter},
			bson.M{"$sort": bson.M{"timestamp": -1, "_id": -1}},
			bson.M{"$lookup": bson.M{
				"from": "Execution",
				"let":  bson.M{"txid": "$txid", "blockhash": "$blockhash"},
				"pipeline": []bson.M{
					bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
						bson.M{"$eq": []interface{}{"$txid", "$$txid"}},
						bson.M{"$eq": []interface{}{"$blockhash", "$$blockhash"}},
					}}}},
					bson.M{"$project": bson.M{"vmstate": 1}},
				},
				"as": "execution"},
			},

			bson.M{"$lookup": bson.M{
				"from": "Transaction",
				"let":  bson.M{"hash": "$txid", "blockhash": "$blockhash"},
				"pipeline": []bson.M{
					bson.M{"$match": bson.M{"hash": bson.M{"$ne": "0x0000000000000000000000000000000000000000000000000000000000000000"}}},
					bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
						bson.M{"$eq": []interface{}{"$hash", "$$hash"}},
						bson.M{"$eq": []interface{}{"$blockhash", "$$blockhash"}},
					}}}},
					bson.M{"$project": bson.M{"netfee": 1, "sysfee": 1}},
				},
				"as": "transaction"},
			},

			bson.M{"$skip": args.Skip},
			bson.M{"$limit": args.Limit},
		},
		Query: []string{},
	}, ret)

	if err != nil {
		return err
	}

	count, err := me.Client.QueryDocument(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
	}{
		Collection: "Nep11TransferNotification",
		Index:      "GetNep11TransferByAddress",
		Sort:       bson.M{},
		Filter:     filter}, ret)
	if err != nil {
		return err
	}

	for _, item := range r1 {
		execution := item["execution"].(primitive.A)
		if len(execution) > 0 {
			item["vmstate"] = execution[0].(map[string]interface{})["vmstate"]

		} else {
			item["vmstate"] = "FAULT"
		}
		transaction := item["transaction"].(primitive.A)
		if len(transaction) > 0 {
			transaction_map := transaction[0].(map[string]interface{})
			item["sysfee"] = transaction_map["sysfee"]
			item["netfee"] = transaction_map["netfee"]
		}
		delete(item, "execution")
		delete(item, "transaction")
	}

	r2, err := me.FilterArrayAndAppendCount(r1, count["total counts"].(int64), args.Filter)
	if err != nil {
		return err
	}
	r, err := json.Marshal(r2)
	if err != nil {
		return err

	}
	if args.Raw != nil {
		*args.Raw = r1
	}
	*ret = json.RawMessage(r)
	return nil
}
