package api

import (
	"encoding/json"
	"io/ioutil"
	"neo3fura_http/lib/httpx"
	"net/http"
)

func (me *T) GetUserInfoTwitter(args struct {
	AccessToken string

	Filter map[string]interface{}
	Raw    *map[string]interface{}
}, ret *json.RawMessage) error {
	req, err := http.NewRequest(http.MethodGet, "https://api.twitter.com/2/users/me", nil)
	if err != nil {
		//log.Errorf("make request error:%v", err)
		return err
	}
	var bearer = "Bearer " + args.AccessToken
	req.Header.Add("Authorization", bearer)

	resp, err := httpx.Do(req)
	if err != nil {
		//log.Errorf("send request error:%v", err)
		return err
	}
	defer resp.Body.Close()
	reader := resp.Body
	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	var data map[string]interface{}
	if err1 := json.Unmarshal(body, &data); err1 != nil {
		return err
	}
	r2, err := me.Filter(data, args.Filter)
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
