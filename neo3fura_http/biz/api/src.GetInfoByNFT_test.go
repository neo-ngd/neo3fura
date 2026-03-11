package api

import (
	"encoding/json"
	"testing"

	"neo3fura_http/lib/type/h160"
	"neo3fura_http/lib/type/strval"
	"neo3fura_http/var/stderr"
)

func TestGetInfoByNFTRecoversPanic(t *testing.T) {
	var ret json.RawMessage
	err := (&T{}).GetInfoByNFT(struct {
		Asset   h160.T
		Tokenid []string
		Filter  map[string]interface{}
		Raw     *map[string]interface{}
	}{
		Asset:   h160.T("0xc198d687cc67e244662c3b9c1325f095f8e663b1"),
		Tokenid: []string{"AQ=="},
	}, &ret)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != stderr.ErrData.Error() {
		t.Fatalf("expected ErrData, got: %v", err)
	}
}

func TestGetInfoByNFTListRecoversPanic(t *testing.T) {
	var ret json.RawMessage
	err := (&T{}).GetInfoByNFTList(struct {
		NFT []struct {
			Asset   h160.T
			TokenId strval.T
		}
		Filter map[string]interface{}
		Raw    *map[string]interface{}
	}{
		NFT: []struct {
			Asset   h160.T
			TokenId strval.T
		}{
			{
				Asset:   h160.T("0xc198d687cc67e244662c3b9c1325f095f8e663b1"),
				TokenId: strval.T("AQ=="),
			},
		},
	}, &ret)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != stderr.ErrData.Error() {
		t.Fatalf("expected ErrData, got: %v", err)
	}
}
