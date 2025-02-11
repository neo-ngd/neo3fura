package api

import (
	"encoding/json"
	"fmt"

	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (me *T) GetAssetInfos(args struct {
	Filter    map[string]interface{}
	Addresses []h160.T
	Limit     int64
	Skip      int64
	Standard  strval.T
}, ret *json.RawMessage) error {
	var f bson.M
	if args.Limit == 0 {
		args.Limit = 512
	}
	if args.Addresses == nil {
		f = bson.M{}
	} else {
		addresses := make([]interface{}, 0)
		for _, address := range args.Addresses {
			if address.Valid() == false {
				return stderr.ErrInvalidArgs
			} else {
				addresses = append(addresses, bson.M{"hash": address.TransferredVal()})
			}
		}
		if len(addresses) == 0 {
			f = bson.M{}
		} else {
			f = bson.M{"$or": addresses}
		}
	}

	if args.Standard == "NEP17" || args.Standard == "NEP11" {
		f["type"] = args.Standard.Val()
	}

	r1, count, err := me.Client.QueryAll(struct {
		Collection string
		Index      string
		Sort       bson.M
		Filter     bson.M
		Query      []string
		Limit      int64
		Skip       int64
	}{
		Collection: "Asset",
		Index:      "GetAssetInfos",
		Sort:       bson.M{},
		Filter:     f,
		Query:      []string{},
		Skip:       args.Skip,
		Limit:      args.Limit,
	}, ret)
	if err != nil {
		return err
	}

	fmt.Println("count:", count)
	// retrieve all tokens
	r2, err := me.Client.QueryLastJob(struct{ Collection string }{Collection: "PopularTokens"})
	if err != nil {
		return err
	}
	r3, err := me.Client.QueryLastJob(struct{ Collection string }{Collection: "Holders"})
	if err != nil {
		return err
	}
	for _, item := range r1 {
		populars := r2["Populars"].(primitive.A)
		item["ispopular"] = false
		for _, v := range populars {
			if item["hash"] == v {
				item["ispopular"] = true
			}
		}
		holders := r3["Holders"].(primitive.A)
		for _, h := range holders {
			m := h.(map[string]interface{})
			for k, v := range m {
				if item["hash"] == k {
					item["holders"] = v
				}
			}
		}

		raw1 := make(map[string]interface{})
		if item["type"] == "Unknown" {
			err := me.GetContractByContractHash(struct {
				ContractHash h160.T
				Filter       map[string]interface{}
				Raw          *map[string]interface{}
			}{ContractHash: h160.T(fmt.Sprint(item["hash"])), Filter: nil, Raw: &raw1}, ret)
			if err != nil {
				return nil
			}
			m := make(map[string]interface{})
			json.Unmarshal([]byte(raw1["manifest"].(string)), &m)
			methods := m["abi"].(map[string]interface{})["methods"].([]interface{})
			i := 0
			for _, method := range methods {
				if method.(map[string]interface{})["name"].(string) == "transfer" {
					i = i + 1
				}
				if (method.(map[string]interface{})["name"].(string) == "transfer") && len(method.(map[string]interface{})["parameters"].([]interface{})) == 4 {
					i = i + 1
				}
				if (method.(map[string]interface{})["name"].(string) == "transfer") && len(method.(map[string]interface{})["parameters"].([]interface{})) == 3 {
					i = i + 2
				}
				if method.(map[string]interface{})["name"].(string) == "balanceOf" {
					i = i + 1
				}
				if method.(map[string]interface{})["name"].(string) == "totalSupply" {
					i = i + 1
				}
				if method.(map[string]interface{})["name"].(string) == "decimals" {
					i = i + 1
				}
			}
			if i == 5 {
				item["type"] = "NEP17"
			}
			if i == 6 {
				item["type"] = "NEP11"
			}
		}
	}

	//r5 := make([]map[string]interface{}, 0)
	//r6 := make([]map[string]interface{}, 0)
	//for _, item := range r1 {
	//	if args.Standard == "" || (args.Standard == "NEP11" && item["type"] == "NEP11") || (args.Standard == "NEP17" && item["type"] == "NEP17") {
	//		r5 = append(r5, item)
	//	}
	//}
	//for i, item := range r5 {
	//	if int64(i) < args.Skip {
	//		continue
	//	} else if int64(i) > args.Skip+args.Limit-1 {
	//		continue
	//	} else {
	//		r6 = append(r6, item)
	//	}
	//}
	r4, err := me.FilterArrayAndAppendCount(r1, count, args.Filter)
	if err != nil {
		return err
	}
	r, err := json.Marshal(r4)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
