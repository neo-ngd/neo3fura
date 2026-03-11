package api

import (
	"encoding/json"
	"neo3fura_http/lib/httpx"
)

func (me *T) GetNeoFsImage(args struct{ ImageId string }, ret *json.RawMessage) error {
	result := make(map[string]interface{})
	resp, err := httpx.Get("http://" + me.Client.NeoFs + args.ImageId)
	if err != nil {
		result["ImageUrl"] = "Image object not found"
		r, err := json.Marshal(result)
		if err != nil {
			return err
		}
		*ret = json.RawMessage(r)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		result["ImageUrl"] = "Image object not found"
	} else {
		result["ImageUrl"] = me.Client.NeoFs + args.ImageId
	}
	r, err := json.Marshal(result)
	if err != nil {
		return err
	}
	*ret = json.RawMessage(r)
	return nil
}
