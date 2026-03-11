package api

import (
	"encoding/json"
	"io/ioutil"
	"neo3fura_http/lib/httpx"
	"net/http"
)

func (me *T) GetOpenseaSingleCollection(args struct {
	CollectionSlug string
	ApiKey         string
	Filter         map[string]interface{}
}, ret *json.RawMessage) error {

	var requestGetURLNoParams = "https://api.opensea.io/api/v1/collection/" + args.CollectionSlug

	req, err := http.NewRequest("GET", requestGetURLNoParams, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-KEY", args.ApiKey)
	resp, err := httpx.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	resbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	re := make(map[string]interface{})
	err = json.Unmarshal(resbody, &re)
	if err != nil {
		return err
	}

	r, err := json.Marshal(re)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
