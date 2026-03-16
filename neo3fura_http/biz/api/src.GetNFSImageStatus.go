package api

import (
	"encoding/json"
	"neo3fura_http/lib/httpx"
	"neo3fura_http/lib/type/strval"
)

func (me *T) GetNFSImgStatus(args struct {
	Url    strval.T
	Filter map[string]interface{}
}, ret *json.RawMessage) error {

	result := make(map[string]interface{})

	resp, err := httpx.Get(args.Url.Val())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		result["ImageStatus"] = true
	} else {
		result["ImageStatus"] = false
	}
	r, err := json.Marshal(result)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
