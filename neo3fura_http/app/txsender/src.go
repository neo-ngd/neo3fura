package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"neo3fura_http/lib/httpx"
	log2 "neo3fura_http/lib/log"
	"os"
	"strings"
	"time"
)

func main() {
	for {
		task()
	}
}

func init() {
	addressesNEORPCPOPPER = strings.Split(os.ExpandEnv("${NEORPC_POPPERADDRESSES}"), " ")
	addressesNEOCLI = strings.Split(os.ExpandEnv("${NEOCLI_ADDRESSES}"), " ")
}

var addressesNEORPCPOPPER []string
var addressesNEOCLI []string

func task() {
	defer func() {
		if r := recover(); r != nil {
			log2.Infof("[!!!!][ERROR]", r)
			time.Sleep(time.Second)
		}
	}()
	if len(addressesNEORPCPOPPER) == 0 || len(addressesNEOCLI) == 0 {
		log2.Errorf("[txsender] empty upstream addresses")
		time.Sleep(time.Second)
		return
	}
	addressPOPPER := addressesNEORPCPOPPER[rand.Intn(len(addressesNEORPCPOPPER))]
	resp, err := httpx.Get(addressPOPPER)
	if err != nil {
		log2.Errorf("[txsender] popper request failed: %v", err)
		time.Sleep(time.Second)
		return
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log2.Errorf("[txsender] read popper body failed: %v", err)
		time.Sleep(time.Second)
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      rand.Uint32(),
		"method":  "sendrawtransaction",
		"params": []interface{}{
			hex.EncodeToString(data),
		},
	})
	if err != nil {
		return
	}

	for i := time.Millisecond; i < time.Second; i = i * 2 {
		addressNEOCLI := addressesNEOCLI[rand.Intn(len(addressesNEOCLI))]
		resp, err := httpx.Post(addressNEOCLI, "application/json", bytes.NewReader(payload))
		if err != nil {
			log2.Infof("[????][REQ]", err)
			continue
		}
		resp.Body.Close()
	}
}
