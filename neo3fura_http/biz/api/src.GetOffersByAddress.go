package api

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"io/ioutil"
	"log"
	"neo3fura_http/lib/httpx"
	log2 "neo3fura_http/lib/log"
	"neo3fura_http/lib/mapsort"
	"neo3fura_http/lib/type/OfferState"
	_ "neo3fura_http/lib/type/OfferState"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (me *T) GetOffersByAddress(args struct {
	Address    h160.T
	OfferState strval.T //state:vaild  received
	Limit      int64
	Skip       int64
	Filter     map[string]interface{}
}, ret *json.RawMessage) (errRet error) {
	defer func() {
		if r := recover(); r != nil {
			log2.Errorf("GetOffersByAddress panic recovered: %v", r)
			errRet = stderr.ErrData
		}
	}()

	currentTime := time.Now().UnixNano() / 1e6
	if args.Address.Valid() == false {
		return stderr.ErrInvalidArgs
	}

	pipeline := []bson.M{}
	if args.OfferState.Val() == OfferState.Valid.Val() { //拍卖中  accont >0 && auctionType =2 &&  owner=market && runtime <deadline
		pipeline = []bson.M{
			bson.M{"$match": bson.M{"user": args.Address, "eventname": "Offer"}},
			bson.M{"$lookup": bson.M{
				"from": "Nep11Properties",
				"let":  bson.M{"asset": "$asset", "tokenid": "$tokenid"},
				"pipeline": []bson.M{
					bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
						bson.M{"$eq": []interface{}{"$tokenid", "$$tokenid"}},
						bson.M{"$eq": []interface{}{"$asset", "$$asset"}},
					}}}},
					bson.M{"$project": bson.M{"asset": 1, "tokenid": 1, "properties": 1}},
				},
				"as": "properties"},
			},
		}
	} else if args.OfferState.Val() == OfferState.Received.Val() {
		pipeline = []bson.M{
			bson.M{"$match": bson.M{"eventname": "Offer", "$or": []interface{}{
				bson.M{"extendData": bson.M{"$regex": "originOwner\":\"" + args.Address}},
				bson.M{"extendData": bson.M{"$regex": "originOwner\": \"" + args.Address}},
			}}},
			bson.M{"$lookup": bson.M{
				"from": "Nep11Properties",
				"let":  bson.M{"asset": "$asset", "tokenid": "$tokenid"},
				"pipeline": []bson.M{
					bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
						bson.M{"$eq": []interface{}{"$tokenid", "$$tokenid"}},
						bson.M{"$eq": []interface{}{"$asset", "$$asset"}},
					}}}},
					bson.M{"$project": bson.M{"asset": 1, "tokenid": 1, "properties": 1}},
				},
				"as": "properties"},
			},
		}
	} else {
		pipeline = []bson.M{
			bson.M{"$match": bson.M{"eventname": "Offer", "$or": []interface{}{
				bson.M{"extendData": bson.M{"$regex": "originOwner\":\"" + args.Address}},
				bson.M{"extendData": bson.M{"$regex": "originOwner\": \"" + args.Address}},
				bson.M{"user": args.Address},
			}}},
			bson.M{"$lookup": bson.M{
				"from": "Nep11Properties",
				"let":  bson.M{"asset": "$asset", "tokenid": "$tokenid"},
				"pipeline": []bson.M{
					bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
						bson.M{"$eq": []interface{}{"$tokenid", "$$tokenid"}},
						bson.M{"$eq": []interface{}{"$asset", "$$asset"}},
					}}}},
					bson.M{"$project": bson.M{"properties": 1}},
				},
				"as": "properties"},
			},
		}
	}

	var r1, err = me.Client.QueryAggregate(
		struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Pipeline   []bson.M
			Query      []string
		}{
			Collection: "MarketNotification",
			Index:      "GetOffersByAddress",
			Sort:       bson.M{"timestamp": -1},
			Filter:     bson.M{},
			Pipeline:   pipeline,
			Query:      []string{},
		}, ret)
	if err != nil {
		return err
	}

	result := make([]map[string]interface{}, 0)
	for _, item := range r1 {

		//查看offer 当前状态
		offer_nonce := item["nonce"]
		offer, _ := me.Client.QueryOne(struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Query      []string
		}{
			Collection: "MarketNotification",
			Index:      "getOfferSate",
			Sort:       bson.M{},
			Filter: bson.M{
				"nonce":   offer_nonce,
				"asset":   item["asset"],
				"tokenid": item["tokenid"],
				//"eventname":"CancelOffer",
				"$or": []interface{}{
					bson.M{"eventname": "CompleteOffer"},
					bson.M{"eventname": "CancelOffer"},
				},
			},
			Query: []string{},
		}, ret)

		if len(offer) > 0 {
			continue
		}

		if extendData, ok := toString(item["extendData"]); ok && extendData != "" {
			var data map[string]interface{}
			if err2 := json.Unmarshal([]byte(extendData), &data); err2 == nil {
				item["originOwner"] = data["originOwner"]
				item["offerAsset"] = data["offerAsset"]
				oa, ok := toString(data["offerAmount"])
				if !ok {
					continue
				}
				offerAmount, err := strconv.ParseInt(oa, 10, 64)
				if err != nil {
					return err
				}
				item["offerAmount"] = offerAmount
				dl, ok := toString(data["deadline"])
				if !ok {
					continue
				}
				deadline, err := strconv.ParseInt(dl, 10, 64)
				if err != nil {
					return err
				}
				if deadline < currentTime {
					continue
				}
				item["deadline"] = deadline
			} else {
				return err2
			}
		}

		nftproperties := item["properties"]
		if nftproperties != nil && nftproperties != "" {
			pp, ok := toPrimitiveA(nftproperties)
			if ok && len(pp) > 0 {
				it, ok := toMap(pp[0])
				if !ok {
					continue
				}
				extendData, _ := toString(it["properties"])
				asset, _ := toString(item["asset"])
				tokenid, _ := toString(item["tokenid"])
				if asset == "" || tokenid == "" {
					continue
				}
				if extendData != "" {
					properties := make(map[string]interface{})
					var data map[string]interface{}
					if err1 := json.Unmarshal([]byte(extendData), &data); err1 == nil {
						name, ok := data["name"]
						if ok {
							item["name"] = name
						} else {
							item["name"] = ""
						}
						image, ok := data["image"]
						if ok {
							properties["image"] = image
							//item["image"] = image
							if imageStr, ok := toString(image); ok {
								item["image"] = ImagUrl(asset, imageStr, "images")
							}
						} else {
							item["image"] = ""
						}
						thumbnail, ok := data["thumbnail"]
						if ok {
							thumbnailStr, ok := toString(thumbnail)
							if !ok {
								continue
							}
							tb, err22 := base64.URLEncoding.DecodeString(thumbnailStr)
							if err22 != nil {
								return err22
							}
							ss := string(tb[:])
							if ss == "" {
								itemAsset, ok1 := toString(item["asset"])
								itemImage, ok2 := toString(item["image"])
								if ok1 && ok2 {
									item["thumbnail"] = ImagUrl(itemAsset, itemImage, "thumbnail")
								}
							} else {
								item["thumbnail"] = ImagUrl(asset, string(tb[:]), "thumbnail")
							}

						} else {
							if item["thumbnail"] == nil {
								if image != nil && image != "" {
									if image == nil {
										item["thumbnail"] = item["image"]
									} else if imageStr, ok := toString(image); ok {
										item["thumbnail"] = ImagUrl(asset, imageStr, "thumbnail")
									}
								}
							}
						}
						tokenuri, ok := data["tokenURI"]
						if ok {
							tokenuriStr, ok := toString(tokenuri)
							if !ok {
								continue
							}
							ppjson, err := GetImgFromTokenURL(tokenurl(tokenuriStr), asset, tokenid)
							if err != nil {
								return err
							}
							for key, value := range ppjson {
								item[key] = value
								properties[key] = value
								if key == "image" {
									img, ok := toString(value)
									if !ok {
										continue
									}
									tb := ImagUrl(asset, img, "thumbnail")
									flag := strings.HasSuffix(tb, ".mp4")
									if flag {
										tb = strings.Replace(tb, ".mp4", "mp4", -1)
									}
									item["thumbnail"] = tb
									item["image"] = ImagUrl(asset, img, "images")
								}

								if key == "name" {
									item["name"] = value
								}
							}
						}
						if item["name"] == "Nuanced Floral Symphony" || item["name"] == "Virtual Visions #1" {
							item["video"] = item["image"]
							delete(item, "image")
							properties["video"] = properties["image"]
							delete(properties, "image")
						}

					} else {
						return err
					}

				} else {
					item["image"] = ""
				}

			}

		}

		deadline, ok := asInt64(item["deadline"])
		if !ok {
			continue
		}
		if deadline > currentTime {
			result = append(result, item)
		}
		delete(item, "extendData")
		delete(item, "properties")
		item["count"] = 1
	}

	//	//OfferCollection
	rt := os.ExpandEnv("${RUNTIME}")

	var secondMarketHash string
	if rt == "staging" {
		secondMarketHash = "0xd2e7cf18ee0d9b509fac02457f54b63e47b25e29"
	} else if rt == "test2" {
		secondMarketHash = "0xc198d687cc67e244662c3b9c1325f095f8e663b1"
	} else {
		secondMarketHash = "0xc198d687cc67e244662c3b9c1325f095f8e663b1"

	}

	raw := make(map[string]interface{}, 0)
	err = me.GetMarketAssetOwnedByAddress(struct {
		Address    h160.T
		MarketHash h160.T
		Limit      int64
		Skip       int64
		Filter     map[string]interface{}
		Raw        *map[string]interface{}
	}{Address: args.Address, MarketHash: h160.T(secondMarketHash), Raw: &raw}, ret)
	if err != nil {
		return err
	}
	//if raw != nil {
	//	asset := raw["assetlist"]
	pipeline1 := []bson.M{}
	if args.OfferState.Val() == OfferState.Valid.Val() { //拍卖中  accont >0 && auctionType =2 &&  owner=market && runtime <deadline
		pipeline1 = []bson.M{
			bson.M{"$match": bson.M{"user": args.Address, "eventname": "OfferCollection"}},
			bson.M{"$lookup": bson.M{
				"from": "Asset",
				"let":  bson.M{"asset": "$asset"},
				"pipeline": []bson.M{
					bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
						//bson.M{"$eq": []interface{}{"$tokenid", "$$tokenid"}},
						bson.M{"$eq": []interface{}{"$hash", "$$asset"}},
					}}}},
					//bson.M{"$project": bson.M{"properties": 1, "asset": 1}},
				},
				"as": "assetInfo"},
			},
		}
	} else if args.OfferState.Val() == OfferState.Received.Val() {
		if raw != nil {
			asset, ok := toInterfaceSlice(raw["assetlist"])
			if !ok {
				asset = []interface{}{}
			}
			assetList := asset
			marketAsset, ok := toMapSlice(raw["result"])
			if !ok {
				marketAsset = []map[string]interface{}{}
			}

			tokenlist := GetAssetTokenid(marketAsset)
			tokenidArr := tokenlist["tokenidArr"]

			if len(assetList) > 0 {
				pipeline1 = []bson.M{
					bson.M{"$match": bson.M{"user": bson.M{"$ne": args.Address.Val()}, "eventname": "OfferCollection", "asset": bson.M{"$in": asset}}},
					bson.M{"$lookup": bson.M{
						"from": "Nep11Properties",
						"let":  bson.M{"asset": "$asset"},
						"pipeline": []bson.M{
							bson.M{"$match": bson.M{"tokenid": bson.M{"$in": tokenidArr}}},
							bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
								//bson.M{"$eq": []interface{}{"$tokenid", "$$tokenid"}},
								bson.M{"$eq": []interface{}{"$asset", "$$asset"}},
							}}}},

							//bson.M{"$project": bson.M{"properties": 1, "asset": 1}},
						},
						"as": "properties"},
					},
				}

			}
		}

	} else {
		if raw != nil && raw["assetlist"] != nil {
			asset := raw["assetlist"]
			pipeline1 = []bson.M{
				bson.M{"$match": bson.M{"eventname": "OfferCollection", "$or": []interface{}{
					bson.M{"user": args.Address},
					bson.M{"asset": bson.M{"$in": asset}},
				}}},

				bson.M{"$lookup": bson.M{
					"from": "Nep11Properties",
					"let":  bson.M{"asset": "$asset", "tokenid": "$tokenid"},
					"pipeline": []bson.M{
						bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
							//bson.M{"$eq": []interface{}{"$tokenid", "$$tokenid"}},
							bson.M{"$eq": []interface{}{"$asset", "$$asset"}},
						}}}},
						bson.M{"$project": bson.M{"properties": 1, "asset": 1, "tokenid": 1}},
					},
					"as": "properties"},
				},
			}
		} else {
			pipeline1 = []bson.M{
				bson.M{"$match": bson.M{"eventname": "OfferCollection"}},
				bson.M{"$match": bson.M{"$expr": bson.M{"$or": []interface{}{
					bson.M{"user": args.Address},
					//bson.M{"asset": bson.M{"$in": asset}},
				}}}},
				bson.M{"$lookup": bson.M{
					"from": "Nep11Properties",
					"let":  bson.M{"asset": "$asset", "tokenid": "$tokenid"},
					"pipeline": []bson.M{
						bson.M{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
							bson.M{"$eq": []interface{}{"$tokenid", "$$tokenid"}},
							bson.M{"$eq": []interface{}{"$asset", "$$asset"}},
						}}}},
						bson.M{"$project": bson.M{"properties": 1, "asset": 1}},
					},
					"as": "properties"},
				},
			}
		}

	}

	if pipeline1 != nil {
		offerCollection, err := me.Client.QueryAggregate(struct {
			Collection string
			Index      string
			Sort       bson.M
			Filter     bson.M
			Pipeline   []bson.M
			Query      []string
		}{Collection: "MarketNotification",
			Index:    "getOfferCollection",
			Sort:     bson.M{},
			Filter:   bson.M{},
			Pipeline: pipeline1,
			Query:    []string{},
		}, ret)
		if err != nil {
			return err
		}

		if offerCollection != nil {
			for _, item := range offerCollection {
				//查看offer 当前状态
				offer_nonce := item["nonce"]
				offer, _ := me.Client.QueryOne(struct {
					Collection string
					Index      string
					Sort       bson.M
					Filter     bson.M
					Query      []string
				}{
					Collection: "MarketNotification",
					Index:      "getOfferSate",
					Sort:       bson.M{},
					Filter: bson.M{
						"nonce":   offer_nonce,
						"asset":   item["asset"],
						"tokenid": item["tokenid"],
						//"eventname":"CancelOffer",
						"$or": []interface{}{
							bson.M{"eventname": "CompleteOfferCollection"},
							bson.M{"eventname": "CancelOfferCollection"},
						},
					},
					Query: []string{},
				}, ret)

				if len(offer) > 0 {
					offereventname, _ := toString(offer["eventname"])
					if offereventname == "CancelOfferCollection" {
						continue
					} else {
						extendData, _ := toString(item["extendData"])
						var data map[string]interface{}
						if err1 := json.Unmarshal([]byte(extendData), &data); err1 == nil {
							count, ok := toString(data["count"])
							if !ok {
								continue
							}
							if count == "0" {
								continue
							}

						}
					}
				}

				if extendData, ok := toString(item["extendData"]); ok && extendData != "" {
					var data map[string]interface{}
					if err2 := json.Unmarshal([]byte(extendData), &data); err2 == nil {
						//item["originOwner"] = data["originOwner"]
						item["offerAsset"] = data["offerAsset"]
						oa, ok := toString(data["offerAmount"])
						if !ok {
							continue
						}
						offerAmount, err := strconv.ParseInt(oa, 10, 64)
						if err != nil {
							return err
						}
						if data["count"] != nil {
							c, ok := toString(data["count"])
							if !ok {
								continue
							}
							count, err := strconv.ParseInt(c, 10, 64)
							if err != nil {
								return err
							}
							item["count"] = count
						} else {
							item["count"] = 1
						}
						item["offerAmount"] = offerAmount
						dl, ok := toString(data["deadline"])
						if !ok {
							continue
						}
						deadline, err := strconv.ParseInt(dl, 10, 64)
						if err != nil {
							return err
						}
						item["deadline"] = deadline
						if deadline < currentTime {
							continue
						}
						//properties
						if args.OfferState.Val() == OfferState.Received.Val() {
							if item["properties"] != nil {
								properties, ok := toPrimitiveA(item["properties"])
								if !ok {
									continue
								}
								for _, ppit := range properties {
									copyItem := item
									ppitem, ok := toMap(ppit)
									if !ok {
										continue
									}
									ppinfo := ppitem["properties"]
									ppAsset, ok := toString(ppitem["asset"])
									if !ok || ppAsset == "" {
										continue
									}
									ppTokenid, ok := toString(ppitem["tokenid"])
									if !ok || ppTokenid == "" {
										continue
									}
									var ppdata map[string]interface{}
									copyItem["tokenid"] = ppTokenid
									//item["asset"] = ppAsset
									copyItem["properties"] = ppinfo

									//

									marketInfo, err := me.Client.QueryOne(struct {
										Collection string
										Index      string
										Sort       bson.M
										Filter     bson.M
										Query      []string
									}{Collection: "Market",
										Index:  "GetMarketInfo",
										Sort:   bson.M{},
										Filter: bson.M{"amount": bson.M{"$gt": 0}, "asset": item["asset"], "tokenid": ppTokenid},
										Query:  []string{},
									}, ret)

									if err != nil {
										return stderr.ErrGetNFTInfo
									}
									market := marketInfo["market"]
									owner := marketInfo["owner"]
									bidder := marketInfo["bidder"]
									bidAmount, ok := asDecimalString(marketInfo["bidAmount"])
									if !ok {
										bidAmount = "0"
									}
									ddl, ok := asInt64(marketInfo["deadline"])
									if !ok {
										continue
									}

									if market == owner && ddl > currentTime { // 上架未过期
										copyItem["originOwner"] = marketInfo["auctor"]
									} else if market == owner && ddl < currentTime { //上架过期
										if bidAmount == "0" {
											copyItem["originOwner"] = marketInfo["auctor"]
										} else {
											copyItem["originOwner"] = bidder
										}
									} else { //未上架
										copyItem["originOwner"] = owner
									}
									//筛选
									originOwner, ok := toString(copyItem["originOwner"])
									if !ok || originOwner != args.Address.Val() {
										continue
									}
									//properties
									if ppinfo != nil {
										ppinfoStr, ok := toString(ppinfo)
										if !ok {
											continue
										}
										if err1 := json.Unmarshal([]byte(ppinfoStr), &ppdata); err1 == nil {
											name, ok := ppdata["name"]
											if ok {
												copyItem["name"] = name
											} else {
												copyItem["name"] = ""
											}
											image, ok := ppdata["image"]
											if ok {
												if imageStr, ok := toString(image); ok {
													copyItem["image"] = ImagUrl(ppAsset, imageStr, "images")
												}
											} else {
												copyItem["image"] = ""
											}
											thumbnail, ok := ppdata["thumbnail"]
											if ok {
												thumbnailStr, ok := toString(thumbnail)
												if !ok {
													continue
												}
												tb, err22 := base64.URLEncoding.DecodeString(thumbnailStr)
												if err22 != nil {
													return err22
												}
												//item["image"] = string(tb[:])
												copyItem["thumbnail"] = ImagUrl(ppAsset, string(tb[:]), "thumbnail")
											} else {
												if copyItem["thumbnail"] == nil {
													if image != nil && image != "" {
														if image == nil {
															copyItem["thumbnail"] = item["image"]
														} else if imageStr, ok := toString(image); ok {
															copyItem["thumbnail"] = ImagUrl(ppAsset, imageStr, "thumbnail")
														}
													}
												}
											}
											tokenuri, ok := ppdata["tokenURI"]
											if ok {
												tokenuriStr, ok := toString(tokenuri)
												if !ok {
													continue
												}
												ppjson, err := GetImgFromTokenURL(tokenurl(tokenuriStr), ppAsset, ppTokenid)
												if err != nil {
													return err
												}
												for key, value := range ppjson {
													copyItem[key] = value

													if key == "image" {
														img, ok := toString(value)
														if !ok {
															continue
														}
														tb := ImagUrl(ppAsset, img, "thumbnail")
														flag := strings.HasSuffix(tb, ".mp4")
														if flag {
															tb = strings.Replace(tb, ".mp4", "mp4", -1)
														}
														copyItem["thumbnail"] = tb
														copyItem["image"] = ImagUrl(ppAsset, img, "images")
													}
													if key == "name" {
														copyItem["name"] = value
													}
												}
											}
											nameVal, ok := toString(copyItem["name"])
											if ok && nameVal == "Nuanced Floral Symphony" {
												copyItem["video"] = copyItem["image"]
												delete(copyItem, "image")
											}
											if ok && nameVal == "Virtual Visions #1" {
												copyItem["video"] = copyItem["image"]
												delete(copyItem, "image")
											}

										} else {
											return err
										}
									}

									delete(copyItem, "extendData")
									delete(copyItem, "properties")

									re := make(map[string]interface{})
									re = CopyMap(re, copyItem)
									result = append(result, re)

								}
							}

						} else if args.OfferState.Val() == OfferState.Valid.Val() {
							if item["assetInfo"] != nil {
								assetInfo, ok := toPrimitiveA(item["assetInfo"])
								if !ok || len(assetInfo) == 0 {
									continue
								}
								info, ok := toMap(assetInfo[0])
								if !ok {
									continue
								}
								item["name"] = info["tokenname"]
								item["image"] = ""
								item["thumbnail"] = ""
								item["originOwner"] = ""
								delete(item, "assetInfo")
								delete(item, "extendData")

								result = append(result, item)
							}
						}

					} else {
						return err2
					}
				}

			}

		}

		//append(result, )
	}
	//}

	if args.OfferState.Val() == OfferState.Received.Val() {
		result = mapsort.MapSort(result, "offerAmount")
	}
	pageResult := make([]map[string]interface{}, 0)
	for i, item := range result {
		if int64(i) < args.Skip {
			continue
		} else if int64(i) > args.Skip+args.Limit-1 {
			continue
		} else {
			pageResult = append(pageResult, item)
		}
	}

	var count = int64(len(pageResult))

	if err != nil {
		return err
	}
	r2, err := me.FilterArrayAndAppendCount(result, count, args.Filter)
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

func GetAssetTokenid(mapArr []map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	var tokenidArr []interface{}
	if len(mapArr) > 0 {
		for _, item := range mapArr {
			asset, ok := toString(item["asset"])
			if !ok || asset == "" {
				continue
			}
			marketArr, ok := toPrimitiveA(item["marketAsset"])
			if !ok {
				continue
			}
			tokenidList := []interface{}{}
			for _, it := range marketArr {
				i, ok := toMap(it)
				if !ok {
					continue
				}
				tokenid, ok := toString(i["tokenid"])
				if !ok || tokenid == "" {
					continue
				}
				tokenidList = append(tokenidList, tokenid)
				tokenidArr = append(tokenidArr, tokenid)
			}
			result[asset] = tokenidList
		}
	}
	result["tokenidArr"] = tokenidArr
	return result
}

func toString(v interface{}) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	default:
		return "", false
	}
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	m, ok := v.(map[string]interface{})
	return m, ok
}

func toPrimitiveA(v interface{}) (primitive.A, bool) {
	switch x := v.(type) {
	case primitive.A:
		return x, true
	case []interface{}:
		return primitive.A(x), true
	default:
		return nil, false
	}
}

func toInterfaceSlice(v interface{}) ([]interface{}, bool) {
	switch x := v.(type) {
	case []interface{}:
		return x, true
	case primitive.A:
		return []interface{}(x), true
	default:
		return nil, false
	}
}

func toMapSlice(v interface{}) ([]map[string]interface{}, bool) {
	switch x := v.(type) {
	case []map[string]interface{}:
		return x, true
	case []interface{}:
		result := make([]map[string]interface{}, 0, len(x))
		for _, it := range x {
			m, ok := toMap(it)
			if !ok {
				continue
			}
			result = append(result, m)
		}
		return result, true
	default:
		return nil, false
	}
}

func GetImgFromTokenURL(tokenurl string, asset string, tokenid string) (map[string]interface{}, error) {
	//检查该tokenurl 文件是否本地存在
	if tokenurl == "" {
		return nil, nil
	}
	currentPath, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	//
	num, err := base64.StdEncoding.DecodeString(tokenid)
	if err != nil {
		return nil, err
	}
	fname, err := bytesToIntS(num)
	if err != nil {
		return nil, err
	}
	filename := strconv.FormatInt(int64(fname), 10) + ".json"
	path := currentPath + "/tokenURI/" + asset + "/" + filename
	isExit, _ := PathExists(path)
	jsonData := make(map[string]interface{})
	if !isExit { //读取数据并保存到本地
		filepath := CreateDateDir(currentPath+"/tokenURI/", asset)
		response, err := httpx.Get(tokenurl)
		if err != nil {
			log.Println("http get error: ", err)
			return nil, err
		}

		raw := response.Body
		defer raw.Close()

		out, err := os.Create(filepath + "/" + filename)
		if err != nil {
			return nil, err
		}

		wt := bufio.NewWriter(out)
		defer out.Close()

		_, err = io.Copy(wt, response.Body)
		if err != nil {
			return nil, err
		}
		wt.Flush()

	}
	//从文件读数据
	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	body, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	if len(body) > 0 {
		err := json.Unmarshal([]byte(string(body)), &jsonData)
		if err != nil {
			log.Println("imag from json error :", err, tokenurl)
			return nil, err
		}

		attributes, ok := jsonData["attributes"]

		if ok {
			if attribute, ok := toInterfaceSlice(attributes); ok {
				for _, item := range attribute {
					it, ok := toMap(item)
					if !ok {
						continue
					}
					traitType, ok := toString(it["trait_type"])
					if !ok || traitType == "" {
						continue
					}
					jsonData[traitType] = it["value"]
				}
			}
			delete(jsonData, "attributes")
		}
		delete(jsonData, "number")

	}

	return jsonData, nil
}
func Imgname(asset string, url string) string {
	imgname := strings.ReplaceAll(url, "/", "")
	imgname = strings.ReplaceAll(imgname, "%20", "")
	rt := os.ExpandEnv("${RUNTIME}")
	const test = "0xaecbad96ccc77c8b147a52e45723a6b5886454e0"
	const main = "0x9f344fe24c963d70f5dcf0cfdeb536dc9c0acb3a"
	split := strings.Split(url, ".")
	suf := split[len(split)-1]
	pre := "ipfs://bafybeiapiufkjejfj2mdvjyigrga5vt3o2sd6xf35372tnptiah7kygm7m/1.gif"
	if rt == "staging" && asset == main && suf == "gif" {
		imgname = strings.ReplaceAll(pre, "/", "")
	} else if rt == "test2" && asset == test && suf == "gif" {
		imgname = strings.ReplaceAll(pre, "/", "")
	}

	return imgname
}
func ImagUrl(asset string, imgurl string, pre string) string {
	rt := os.ExpandEnv("${RUNTIME}")
	name := Imgname(asset, imgurl)
	url := ""
	switch rt {
	case "test":
		url = "https://testimg.megaoasis.io/" + pre + "/" + asset + "/" + name
	case "test2":
		url = "https://testimg.megaoasis.io/" + pre + "/" + asset + "/" + name
	case "staging":
		url = "https://img.megaoasis.io/" + pre + "/" + asset + "/" + name
	default:
		log2.Errorf("runtime environment mismatch")
	}
	return url
}

func CreateDateDir(basepath string, folderName string) string {

	folderPath := filepath.Join(basepath, folderName)
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		_ = os.MkdirAll(folderPath, 0777)
		_ = os.Chmod(folderPath, 0777)
	}
	return folderPath
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	//当为空文件或文件夹存在
	if err == nil {
		return true, nil
	}
	//os.IsNotExist(err)为true，文件或文件夹不存在
	if os.IsNotExist(err) {
		return false, nil
	}
	//其它类型，不确定是否存在
	return false, err
}

func bytesToIntS(b []byte) (int, error) {
	if len(b) == 3 {
		b = append([]byte{0}, b...)
	}
	bytesBuffer := bytes.NewBuffer(b)
	switch len(b) {
	case 1:
		var tmp int8
		err := binary.Read(bytesBuffer, binary.LittleEndian, &tmp)
		return int(tmp), err
	case 2:
		var tmp int16
		err := binary.Read(bytesBuffer, binary.LittleEndian, &tmp)
		return int(tmp), err
	case 4:
		var tmp int32
		err := binary.Read(bytesBuffer, binary.LittleEndian, &tmp)
		return int(tmp), err
	default:
		return 0, fmt.Errorf("%s", "BytesToInt bytes lenth is invaild!")
	}
}

func CopyMap(dst map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for key, it := range src {
		dst[key] = it
	}
	return dst
}
