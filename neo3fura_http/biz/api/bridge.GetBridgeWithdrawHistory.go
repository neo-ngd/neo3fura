package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"neo3fura_http/lib/neox"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"
)

func (me *T) GetBridgeWithdrawHistory(args struct {
	ContractHash h160.T
	Limit        int64
	Skip         int64
	Filter       map[string]interface{}
}, ret *json.RawMessage) error {
	if args.ContractHash.Valid() == false {
		return stderr.ErrInvalidArgs
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
		Collection: "Notification",
		Index:      "GetBridgeWithdrawHistory",
		Sort:       bson.M{"_id": -1},
		Filter: bson.M{
			"contract": args.ContractHash.Val(),
			"$or":      []interface{}{bson.M{"eventname": "GasDeposit"}, bson.M{"eventname": "TokenDeposit"}},
		},
		Query: []string{},
		Limit: args.Limit,
		Skip:  args.Skip,
	}, ret)

	//get status of target chain
	for _, item := range r1 {
		eventname := item["eventname"].(string)
		if eventname == "GasDeposit" {
			value := item["state"].(map[string]interface{})["value"].(primitive.A)
			nonce := value[0].(map[string]interface{})["value"]
			to := value[1].(map[string]interface{})["value"].(string)
			toDecode, err := base64.StdEncoding.DecodeString(to)
			if err != nil {
				return fmt.Errorf("fail to decode to toAddress: %w", err)
			}
			toAddress, err := util.Uint160DecodeBytesLE(toDecode)
			if err != nil {
				return fmt.Errorf("fail to Uint160DecodeBytesLE for toAddress: %w", err)
			}

			amount := value[2].(map[string]interface{})["value"]
			from := value[3].(map[string]interface{})["value"].(string)
			fromDecode, err := base64.StdEncoding.DecodeString(from)
			if err != nil {
				return fmt.Errorf("fail to decode to fromAddress: %w", err)
			}
			fromAddress, err := util.Uint160DecodeBytesLE(fromDecode)
			if err != nil {
				return fmt.Errorf("fail to Uint160DecodeBytesLE for fromAddress: %w", err)
			}

			item["from"] = "0x" + fromAddress.String()
			item["to"] = "0x" + toAddress.String()
			item["amount"] = amount
			item["nonce"] = nonce

			tx, status, err := getDepositTxFromNeox("", nonce.(string))
			if err != nil {
				return err
			}
			item["neoxTx"] = tx
			item["status"] = status

		} else {
			value := item["state"].(map[string]interface{})["value"].(primitive.A)
			neoxToken := value[1].(map[string]interface{})["value"].(string)
			neoxTokenStr, err := base64.StdEncoding.DecodeString(neoxToken)
			if err != nil {
				return fmt.Errorf("fail to decode to neoxToken: %w", err)
			}
			neoxTokenDecode, err := util.Uint160DecodeBytesLE(neoxTokenStr)
			if err != nil {
				return fmt.Errorf("fail to Uint160DecodeBytesLE for neoxToken: %w", err)
			}

			nonce := value[2].(map[string]interface{})["value"]
			to := value[3].(map[string]interface{})["value"].(string)
			toDecode, err := base64.StdEncoding.DecodeString(to)
			if err != nil {
				return fmt.Errorf("fail to decode to toAddress: %w", err)
			}
			toAddress, err := util.Uint160DecodeBytesLE(toDecode)
			if err != nil {
				return fmt.Errorf("fail to Uint160DecodeBytesLE for toAddress: %w", err)
			}

			amount := value[4].(map[string]interface{})["value"]
			from := value[5].(map[string]interface{})["value"].(string)
			fromDecode, err := base64.StdEncoding.DecodeString(from)
			if err != nil {
				return fmt.Errorf("fail to decode to fromAddress: %w", err)
			}
			fromAddress, err := util.Uint160DecodeBytesLE(fromDecode)
			if err != nil {
				return fmt.Errorf("fail to Uint160DecodeBytesLE for fromAddress: %w", err)
			}

			item["from"] = "0x" + fromAddress.String()
			item["to"] = "0x" + toAddress.String()
			item["amount"] = amount
			item["nonce"] = nonce
			item["neoxToken"] = "0x" + neoxTokenDecode.String()

			tx, status, err := getDepositTxFromNeox("0x"+neoxTokenDecode.String(), nonce.(string))
			if err != nil {
				return err
			}
			item["neoxTx"] = tx

			item["status"] = status
		}

		delete(item, "state")
		delete(item, "Vmstate")
		delete(item, "index")
		delete(item, "_id")

	}

	r2, err := me.FilterArrayAndAppendCount(r1, count, args.Filter)
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

func getDepositTxFromNeox(tokenHash string, nonceStr string) (string, string, error) {
	rt := os.ExpandEnv("${RUNTIME}")
	var url string
	switch rt {
	case "staging":
		url = neox.MainBridgeDepositTxUrl
	case "test":
		url = neox.TestNetBridgeDepositTxUrl
	default:
		url = neox.TestNetBridgeDepositTxUrl
	}
	var urlStr string
	if tokenHash == "" {
		urlStr = url + nonceStr
	} else {
		urlStr = url + tokenHash + "/" + nonceStr
	}

	resp, err := http.Get(urlStr)
	if err != nil {
		return "", "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	result := make(map[string]interface{})
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", "", nil
	}
	if result["txid"] == nil {
		return "", "pending", nil
	}
	txid := result["txid"].(string)
	status := "success"
	return txid, status, err
}

func getTxStatusFromNeox(txid string) (string, error) {
	rt := os.ExpandEnv("${RUNTIME}")
	var url string
	switch rt {
	case "staging":
		url = neox.MainNeoXRPC + txid
	case "test":
		url = neox.TestNetNeoXRPC + txid
	default:
		url = neox.TestNetNeoXRPC + txid
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	result := make(map[string]interface{})
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", nil
	}

	return result["status"].(string), err
}
