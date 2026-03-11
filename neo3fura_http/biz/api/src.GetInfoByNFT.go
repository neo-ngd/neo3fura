package api

import (
	"encoding/json"
	"io/ioutil"
	"math/big"
	"neo3fura_http/lib/httpx"
	log2 "neo3fura_http/lib/log"
	"os"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"neo3fura_http/lib/type/Contract"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"
)

func (me *T) GetInfoByNFT(args struct {
	Asset   h160.T
	Tokenid []string
	Filter  map[string]interface{}
	Raw     *map[string]interface{}
}, ret *json.RawMessage) (errRet error) {
	defer func() {
		if r := recover(); r != nil {
			log2.Errorf("GetInfoByNFT panic recovered: %v", r)
			errRet = stderr.ErrData
		}
	}()

	if args.Asset.Valid() == false {
		return stderr.ErrInvalidArgs
	}
	rt := os.ExpandEnv("${RUNTIME}")
	primaryMarket := Contract.Main_PrimaryMarket
	if rt == "staging" {
		primaryMarket = Contract.Main_PrimaryMarket

	} else if rt == "test2" {
		primaryMarket = Contract.Test_PrimaryMarket
	} else {
		primaryMarket = Contract.Test_PrimaryMarket
	}

	//获取上架以及Owner信息
	r1, err := me.Client.QueryAggregate(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Pipeline   []bson.M
			Query      []string
		}{
			Collection: "Market",
			Index:      "GetAssetInfo",
			Sort:       bson.M{},
			Filter:     bson.M{},
			Pipeline: []bson.M{
				bson.M{"$match": bson.M{"amount": 1, "asset": args.Asset.Val(), "tokenid": bson.M{"$in": args.Tokenid}}},
				bson.M{"$lookup": bson.M{
					"from": "MarketNotification",
					"let":  bson.M{"asset": "$asset", "tokenid": "$tokenid"},
					"pipeline": []bson.M{ //
						bson.M{"$match": bson.M{"$expr": bson.M{"$or": []interface{}{
							bson.M{"$and": []interface{}{
								bson.M{"$in": []interface{}{"$eventname", []interface{}{"CompleteOfferCollection", "Offer", "CompleteOffer", "Claim"}}},
								bson.M{"$eq": []interface{}{"$asset", "$$asset"}},
								bson.M{"$eq": []interface{}{"$tokenid", "$$tokenid"}},
							}},
							bson.M{"$and": []interface{}{
								bson.M{"$eq": []interface{}{"$eventname", "OfferCollection"}},
								bson.M{"$eq": []interface{}{"$asset", "$$asset"}},
							}},
						}}}},
						bson.M{"$sort": bson.M{"timestamp": 1}},
						bson.M{"$group": bson.M{"_id": "$eventname", "eventArr": bson.M{"$push": "$$ROOT"}, "eventname": bson.M{"$last": "$eventname"}, "market": bson.M{"$last": "$market"}, "timestamp": bson.M{"$last": "$timestamp"}, "extendData": bson.M{"$last": "$extendData"}}},
						//bson.M{"$project": bson.M{"eventname": 1,"eventArr" :1"market": 1, "extendData": 1, "timestamp": 1}},
					},
					"as": "eventlist"}},
			},
			Query: []string{},
		}, ret)
	if err != nil {
		return err
	}

	currentTime := time.Now().UnixNano() / 1e6

	for _, item := range r1 {
		//NFT状态   上架 （售卖中  成交未领取）  未上架
		asset, ok := toString(item["asset"])
		if !ok || asset == "" {
			continue
		}
		tokenid, ok := toString(item["tokenid"])
		if !ok || tokenid == "" {
			continue
		}
		ddl, ok := asInt64(item["deadline"])
		if !ok {
			continue
		}

		bidAmount, ok := asDecimalString(item["bidAmount"])
		if !ok {
			bidAmount = "0"
		}
		if item["market"] != item["owner"] || ddl < currentTime {
			item["state"] = "notlist"
		} else {
			item["state"] = "list"
		}

		item["buyNowAsset"] = ""
		item["buyNowAmount"] = "0"
		item["lastSoldAsset"] = ""
		item["lastSoldAmount"] = "0"
		item["currentBidAsset"] = ""
		item["currentBidAmount"] = "0"
		item["offerAsset"] = ""
		item["offerAmount"] = "0"
		item["nonce"] = 0
		item["eventname"] = ""

		auctionType, ok := asInt32(item["auctionType"])
		if !ok {
			auctionType = 0
		}
		if ddl > currentTime {
			if auctionType == 1 {
				item["buyNowAsset"] = item["auctionAsset"]
				item["buyNowAmount"] = item["auctionAmount"]
			} else if auctionType == 2 {
				if bidAmount != "0" {
					item["currentBidAsset"] = item["auctionAsset"]
					item["currentBidAmount"] = item["bidAmount"]
				} else {
					item["currentBidAsset"] = item["auctionAsset"]
					item["currentBidAmount"] = item["auctionAmount"]
				}
			}
		} else {
			if auctionType == 2 && bidAmount != "0" {
				item["lastSoldAsset"] = item["auctionAsset"]
				item["lastSoldAmount"] = item["bidAmount"]
			}
			market, _ := toString(item["market"])
			if item["owner"] == item["market"] && market == primaryMarket.Val() { //一级市场过期
				if bidAmount == "0" {
					item["lastSoldAsset"] = item["auctionAsset"]
					item["lastSoldAmount"] = item["auctionAmount"]
				}

			}
		}
		var finishTime int64
		eventlist, ok := toPrimitiveA(item["eventlist"])
		if item["eventlist"] != nil && ok && len(eventlist) > 0 {
			for _, it := range eventlist {
				eventItem, ok := toMap(it)
				if !ok {
					continue
				}
				eventname, _ := toString(eventItem["eventname"])
				extendData, _ := toString(eventItem["extendData"])
				market, _ := toString(eventItem["market"])

				data := make(map[string]interface{})
				if extendData == "" {
					continue
				}
				if err := json.Unmarshal([]byte(extendData), &data); err == nil {
					if eventname == "Claim" {
						time, ok := asInt64(eventItem["timestamp"])
						if !ok {
							continue
						}
						if time > finishTime {
							finishTime = time
							item["lastSoldAsset"] = data["auctionAsset"]
							item["lastSoldAmount"] = data["bidAmount"]
						}

					} else if eventname == "Offer" || eventname == "OfferCollection" {
						//判断offer 有效期以及是否有足够的保证金
						deadline, ok := toString(data["deadline"])
						if !ok || deadline == "" {
							continue
						}
						offerddl, _ := strconv.ParseInt(deadline, 10, 64)

						highestOffer := make(map[string]interface{})
						if offerddl > currentTime {
							err := me.GetHighestOfferByNFT(struct {
								Asset      h160.T
								TokenId    strval.T
								MarketHash h160.T
								Limit      int64
								Skip       int64
								Filter     map[string]interface{}
								Raw        *map[string]interface{}
							}{Asset: h160.T(asset), TokenId: strval.T(tokenid), MarketHash: h160.T(market), Raw: &highestOffer}, ret)
							if err != nil {
								return stderr.ErrGetHighestOffer
							}
							if len(highestOffer) > 0 {
								offerAmount, ok := asInt64(highestOffer["offerAmount"])
								if !ok {
									continue
								}
								guarantee, ok := asBigInt(highestOffer["guarantee"])
								if !ok {
									continue
								}
								amount := big.NewInt(offerAmount)
								if guarantee.Cmp(amount) == 1 {
									item["offerAsset"] = highestOffer["offerAsset"]
									item["offerAmount"] = amount.String()
									item["nonce"] = highestOffer["nonce"]
									item["eventname"] = highestOffer["eventname"]
								}
							}
						}
					} else if eventname == "CompleteOffer" || eventname == "CompleteOfferCollection" {
						time, ok := asInt64(eventItem["timestamp"])
						if !ok {
							continue
						}
						if time > finishTime {
							finishTime = time
							item["lastSoldAsset"] = data["offerAsset"]
							item["lastSoldAmount"] = data["offerAmount"]

						}
					}
				}

			}
		}

		if (item["market"] == item["owner"] && ddl > currentTime) || (item["market"] == item["owner"] && ddl < currentTime && bidAmount == "0") { //上架
			item["owner"] = item["auctor"]
		}
		if item["market"] == item["owner"] && ddl < currentTime && bidAmount != "0" { // 未领取
			item["owner"] = item["bidder"]
		}
		delete(item, "eventlist")
	}

	// Batch fetch NNS data for all owners concurrently
	ownerAddrs := make([]string, 0, len(r1))
	for _, item := range r1 {
		if owner, ok := item["owner"].(string); ok && owner != "" {
			ownerAddrs = append(ownerAddrs, owner)
		}
	}
	nnsResults := GetNNSByAddresses(ownerAddrs)
	for _, item := range r1 {
		owner, _ := item["owner"].(string)
		if res, ok := nnsResults[owner]; ok && res.Err == nil {
			item["nns"] = res.NNS
			item["userName"] = res.UserName
		} else {
			item["nns"] = ""
			item["userName"] = ""
		}
	}

	count := len(r1)
	r3, err := me.FilterAggragateAndAppendCount(r1, count, args.Filter)

	if err != nil {
		return err
	}
	r, err := json.Marshal(r3)
	if err != nil {
		return err
	}
	if args.Raw != nil {
		*args.Raw = r3
	}
	*ret = json.RawMessage(r)
	return nil
}

func GetNNSByAddress(address string) (string, string, error) {
	rt := os.ExpandEnv("${RUNTIME}")
	url := "https://megaoasis.ngd.network:8893/profile/get?address="
	if rt == "test2" {
		url = "https://megaoasis.ngd.network:8889/profile/get?address=" //staging
	} else if rt == "test" {
		url = "https://megaoasis.ngd.network:8889/profile/get?address=" //test
	}
	resp, err := httpx.Get(url + address)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var nns, userName string
	if string(body) != "" && string(body) != "null" {
		var data map[string]interface{}
		err = json.Unmarshal(body, &data)
		if err != nil {
			return "", "", err
		}
		nns, _ = toString(data["nns"])
		userName, _ = toString(data["username"])
	} else {
		nns = ""
		userName = ""
	}

	return nns, userName, nil
}

// NNSResult holds the NNS and UserName for an address.
type NNSResult struct {
	NNS      string
	UserName string
	Err      error
}

// GetNNSByAddresses concurrently fetches NNS data for multiple addresses.
// Returns a map[address]NNSResult. Limits concurrency to 10 goroutines.
func GetNNSByAddresses(addresses []string) map[string]NNSResult {
	results := make(map[string]NNSResult, len(addresses))
	if len(addresses) == 0 {
		return results
	}

	// Deduplicate addresses
	unique := make(map[string]struct{}, len(addresses))
	for _, addr := range addresses {
		if addr != "" {
			unique[addr] = struct{}{}
		}
	}

	type indexedResult struct {
		Address string
		NNSResult
	}

	ch := make(chan indexedResult, len(unique))
	sem := make(chan struct{}, 10) // concurrency limit

	for addr := range unique {
		sem <- struct{}{}
		go func(a string) {
			defer func() { <-sem }()
			nns, userName, err := GetNNSByAddress(a)
			ch <- indexedResult{Address: a, NNSResult: NNSResult{NNS: nns, UserName: userName, Err: err}}
		}(addr)
	}

	for i := 0; i < len(unique); i++ {
		r := <-ch
		results[r.Address] = r.NNSResult
	}

	return results
}
