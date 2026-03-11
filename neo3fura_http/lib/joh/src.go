package joh

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/rpc"
	"path/filepath"
	"sync/atomic"

	"github.com/thinkeridea/go-extend/exnet"
	"gopkg.in/yaml.v2"

	"neo3fura_http/config"
	"neo3fura_http/lib/httpx"
	log2 "neo3fura_http/lib/log"
	"neo3fura_http/lib/rwio"
	"neo3fura_http/lib/scex"
)

// T ...
type T struct{}

type Config struct {
	Methods struct {
		Realized []string `yaml:"realized"`
	} `yaml:"methods"`
	Proxy struct {
		URI []string `yaml:"uri"`
	} `yaml:"proxy"`
}

// cachedConfig holds the config loaded once at first use.
var cachedConfig *Config

// apiSet is a map for O(1) method lookup, initialized once.
var apiSet map[string]struct{}

// repostMode uses atomic to avoid data race on concurrent writes.
var repostMode int64 = 0

func init() {
	apiSet = make(map[string]struct{}, len(config.Apis))
	for _, name := range config.Apis {
		apiSet[name] = struct{}{}
	}
}

func (me *T) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	// Limit request body to 1MB to prevent OOM from oversized payloads
	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		log2.Infof("Error in reading body: %v", err)
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}
	r := req.Clone(req.Context())
	req.Body = io.NopCloser(bytes.NewReader(body))
	r.Body = io.NopCloser(bytes.NewReader(body))

	ip := exnet.ClientPublicIP(r)
	if ip == "" {
		ip = exnet.ClientIP(r)
	}
	log2.Infof("Request from:%v", ip)

	request := make(map[string]interface{})
	err = json.Unmarshal(body, &request)
	if err != nil {
		log2.Infof("Error decoding in JSON: %v", err)
		http.Error(w, "Can't decoding in JSON", http.StatusBadRequest)
		return
	}

	log2.Infof("Request is: %v", request["method"])
	c, err := me.getConfig()
	if err != nil {
		log2.Errorf("Open config file error:%s", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	method, _ := request["method"].(string)
	if _, ok := apiSet[method]; ok {
		// can find
		w.Header().Set("Content-Type", "application/json")
		log2.Infof("Serving %v", method)
		conn := &rwio.T{R: req.Body, W: w}
		codec := &scex.T{}
		codec.Init(conn)
		rpc.ServeCodec(codec)
	} else {
		// can't find — repost to proxy
		log2.Infof("Repost %v", method)
		if len(c.Proxy.URI) == 0 {
			log2.Errorf("Repost skipped: no proxy URI configured")
			http.Error(w, "proxy unavailable", http.StatusBadGateway)
			return
		}
		responseBody := bytes.NewBuffer(body)
		w.Header().Set("Content-Type", "application/json")
		idx := (atomic.AddInt64(&repostMode, 1) - 1) % int64(len(c.Proxy.URI))
		resp, err := httpx.Post(c.Proxy.URI[idx], "application/json", responseBody)
		if err != nil {
			log2.Errorf("Repost error%v", err)
			http.Error(w, "proxy request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log2.Errorf("Read err%v", err)
			http.Error(w, "proxy response read failed", http.StatusBadGateway)
			return
		}
		if _, err = w.Write(respBody); err != nil {
			log2.Errorf("Write response error: %v", err)
		}
	}
}

// getConfig returns the cached config, loading it once on first call.
func (me *T) getConfig() (Config, error) {
	if cachedConfig != nil {
		return *cachedConfig, nil
	}
	cfg, err := me.OpenConfigFile()
	if err != nil {
		return Config{}, err
	}
	cachedConfig = &cfg
	return cfg, nil
}

func (me *T) OpenConfigFile() (Config, error) {
	absPath, _ := filepath.Abs("./config.yml")
	f, err := ioutil.ReadFile(absPath)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	err = yaml.Unmarshal(f, &cfg)
	if err != nil {
		return Config{}, err
	}
	return cfg, err
}
