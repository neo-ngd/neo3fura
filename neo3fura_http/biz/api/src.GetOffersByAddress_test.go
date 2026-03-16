package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"
)

func TestGetAssetTokenidSkipsInvalidEntries(t *testing.T) {
	input := []map[string]interface{}{
		{
			"asset": "asset-a",
			"marketAsset": []interface{}{
				map[string]interface{}{"tokenid": "t1"},
				map[string]interface{}{"tokenid": 100},
			},
		},
		{
			"asset":       42,
			"marketAsset": []interface{}{map[string]interface{}{"tokenid": "t2"}},
		},
		{
			"asset":       "asset-b",
			"marketAsset": "bad-type",
		},
	}

	got := GetAssetTokenid(input)
	tokenidArr, ok := got["tokenidArr"].([]interface{})
	if !ok {
		t.Fatalf("tokenidArr type mismatch: %T", got["tokenidArr"])
	}
	if len(tokenidArr) != 1 || tokenidArr[0] != "t1" {
		t.Fatalf("unexpected tokenidArr: %#v", tokenidArr)
	}
	if _, ok := got["asset-a"]; !ok {
		t.Fatalf("expected asset-a key in result")
	}
	if _, ok := got["asset-b"]; ok {
		t.Fatalf("did not expect invalid asset-b to be included")
	}
}

func TestGetImgFromTokenURLHandlesMalformedAttributes(t *testing.T) {
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err = os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	asset := "0xabc"
	tokenDir := filepath.Join(tmp, "tokenURI", asset)
	if err = os.MkdirAll(tokenDir, 0o755); err != nil {
		t.Fatalf("mkdir token dir: %v", err)
	}
	body := `{"name":"demo","attributes":{"bad":"shape"},"number":123}`
	if err = os.WriteFile(filepath.Join(tokenDir, "1.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	data, err := GetImgFromTokenURL("http://unused", asset, "AQ==")
	if err != nil {
		t.Fatalf("GetImgFromTokenURL error: %v", err)
	}
	if data["name"] != "demo" {
		t.Fatalf("unexpected name: %#v", data["name"])
	}
	if _, ok := data["attributes"]; ok {
		t.Fatalf("attributes key should be removed")
	}
	if _, ok := data["number"]; ok {
		t.Fatalf("number key should be removed")
	}
}

func TestGetOffersByAddressRecoversPanic(t *testing.T) {
	var ret json.RawMessage
	err := (&T{}).GetOffersByAddress(struct {
		Address    h160.T
		OfferState strval.T
		Limit      int64
		Skip       int64
		Filter     map[string]interface{}
	}{
		Address: h160.T("0xc198d687cc67e244662c3b9c1325f095f8e663b1"),
		Limit:   10,
		Skip:    0,
	}, &ret)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != stderr.ErrData.Error() {
		t.Fatalf("expected ErrData, got: %v", err)
	}
}
