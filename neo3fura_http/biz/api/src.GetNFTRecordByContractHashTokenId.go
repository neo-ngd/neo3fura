package api

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"
	"strconv"
)

func (me *T) GetNFTRecordByContractHashTokenId(args struct {
	ContractHash h160.T
	MarketHash   h160.T
	TokenId      strval.T
	Filter       map[string]interface{}
}, ret *json.RawMessage) error {

	if args.ContractHash.Valid() == false {
		return stderr.ErrInvalidArgs
	}
	if len(args.TokenId) <= 0 {
		return stderr.ErrInvalidArgs
	}
	f := bson.M{"eventname": bson.M{"$in": []interface{}{"Claim", "CompleteOffer", "CompleteOfferCollection"}}, "asset": args.ContractHash.Val(), "tokenid": args.TokenId.Val()}
	if len(args.MarketHash) > 0 {
		f["market"] = args.MarketHash.Val()
	}
	result := make([]map[string]interface{}, 0)

	r1, _, err := me.Client.QueryAllWithCursor(struct {
		Collection   string
		Index        string
		Sort         bson.M
		Filter       bson.M
		Query        []string
		Limit        int64
		Skip         int64
		CursorFilter bson.M
	}{
		Collection:   "MarketNotification",
		Index:        "GetNFTRecordByContractHashTokenId",
		Sort:         bson.M{"timestamp": -1},
		Filter:       f,
		Query:        []string{},
		CursorFilter: nil,
	}, ret)
	if err != nil {
		return err
	}

	// 获取NFT的Transfer
	var raw2 []map[string]interface{}
	err3 := me.GetNep11TransferByContractHashTokenId(struct {
		ContractHash h160.T
		Limit        int64
		Skip         int64
		Cursor       string
		TokenId      strval.T
		Filter       map[string]interface{}
		Raw          *[]map[string]interface{}
	}{ContractHash: args.ContractHash, TokenId: args.TokenId, Raw: &raw2}, ret)
	if err3 != nil {
		return err3
	}

	asset := args.ContractHash.Val()
	tokenid := args.TokenId.Val()

	var raw3 map[string]interface{}
	err2 := getNFTProperties(strval.T(tokenid), h160.T(asset), me, ret, args.Filter, &raw3)
	for _, item := range raw2 {
		tobanlance := item["tobalance"].(primitive.Decimal128).String()

		rr := make(map[string]interface{})
		rr["asset"] = item["contract"]
		rr["tokenid"] = item["tokenId"]
		rr["from"] = item["from"]
		rr["to"] = item["to"]
		rr["auctionAsset"] = "" //普通账户之间转账  无价格
		rr["auctionAmount"] = ""
		rr["offerAsset"] = ""
		rr["offerAmount"] = ""
		rr["timestamp"] = item["timestamp"]
		rr["eventname"] = "transfer"

		//筛选出从市场交易的nft 会有交易价格
		for _, i := range r1 {
			if item["txid"] == i["txid"] { //为了获取nft的交易价格
				extendData := i["extendData"].(string)

				var dat map[string]interface{}
				if err2 := json.Unmarshal([]byte(extendData), &dat); err2 == nil {
					eventname := i["eventname"].(string)
					rr["timestamp"] = i["timestamp"]
					rr["eventname"] = i["eventname"]
					if eventname == "Claim" {
						bidAmount, err1 := strconv.ParseInt(dat["bidAmount"].(string), 10, 64)
						if err1 != nil {
							return err1
						}
						rr["from"] = item["from"]
						rr["to"] = item["to"]
						auctionAsset := dat["auctionAsset"]
						rr["auctionAsset"] = auctionAsset
						rr["auctionAmount"] = bidAmount

					} else if eventname == "CompleteOffer" || eventname == "CompleteOfferCollection" {
						offerAmount, err1 := strconv.ParseInt(dat["offerAmount"].(string), 10, 64)
						if err1 != nil {
							return err1
						}
						rr["offerAsset"] = dat["offerAsset"]
						rr["offerAmount"] = offerAmount
						rr["from"] = i["user"]
						rr["to"] = dat["offerer"]

					}

				} else {
					return err2
				}

			}
		}
		rr["thumbnail"] = raw3["thumbnail"]
		rr["name"] = raw3["name"]
		rr["number"] = raw3["number"]
		rr["properties"] = raw3["properties"]
		if raw3["image"] != nil && raw3["image"] != "" {
			rr["image"] = raw3["image"]
		}
		if raw3["video"] != nil && raw3["video"] != "" {
			rr["video"] = raw3["video"]
		}
		if err2 != nil {
			rr["image"] = ""
			rr["name"] = ""
			rr["number"] = int64(-1)
			rr["properties"] = ""

		}

		if tobanlance != "0" {
			result = append(result, rr)
		}
	}

	// Batch fetch NNS data for all from/to addresses concurrently
	allAddrs := make([]string, 0, len(result)*2)
	for _, rr := range result {
		if from, ok := rr["from"].(string); ok && from != "" {
			allAddrs = append(allAddrs, from)
		}
		if to, ok := rr["to"].(string); ok && to != "" {
			allAddrs = append(allAddrs, to)
		}
	}
	nnsResults := GetNNSByAddresses(allAddrs)
	for _, rr := range result {
		fromAddr, _ := rr["from"].(string)
		toAddr, _ := rr["to"].(string)
		if res, ok := nnsResults[fromAddr]; ok && res.Err == nil {
			rr["from_nns"] = res.NNS
			rr["from_userName"] = res.UserName
		} else {
			rr["from_nns"] = ""
			rr["from_userName"] = ""
		}
		if res, ok := nnsResults[toAddr]; ok && res.Err == nil {
			rr["to_nns"] = res.NNS
			rr["to_userName"] = res.UserName
		} else {
			rr["to_nns"] = ""
			rr["to_userName"] = ""
		}
	}

	num, err := strconv.ParseInt(strconv.Itoa(len(result)), 10, 64)
	if err != nil {
		return err
	}
	r2, err := me.FilterArrayAndAppendCountWithCursor(result, num, args.Filter, []string{"_id"})
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
