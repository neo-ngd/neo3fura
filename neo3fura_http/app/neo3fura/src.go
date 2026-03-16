package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"neo3fura_http/biz/api"
	"neo3fura_http/biz/job"
	"neo3fura_http/biz/watch"
	"neo3fura_http/config"
	"neo3fura_http/lib/cli"
	"neo3fura_http/lib/joh"
	log2 "neo3fura_http/lib/log"
	"neo3fura_http/lib/monitor"
	"neo3fura_http/lib/verify"
	"net/http"
	"net/rpc"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v2"

	"github.com/go-redis/redis/v8"
	neoRpc "github.com/joeqian10/neo3-gogogo/rpc"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron"
	"github.com/rs/cors"
)

func OpenConfigFile() (Config, error) {
	absPath, _ := filepath.Abs("./config.yml")
	f, err := os.Open(absPath)
	if err != nil {
		return Config{}, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log2.Fatalf("Closing file error: %v", err)
		}
	}(f)
	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return Config{}, err
	}
	return cfg, err
}

type Config struct {
	Database_Dev struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Pass     string `yaml:"pass"`
		Database string `yaml:"database"`
		DBName   string `yaml:"dbname"`
	} `yaml:"database_dev"`
	Database_Test struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Pass     string `yaml:"pass"`
		Database string `yaml:"database"`
		DBName   string `yaml:"dbname"`
	} `yaml:"database_test"`
	Database_Staging struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Pass     string `yaml:"pass"`
		Database string `yaml:"database"`
		DBName   string `yaml:"dbname"`
	} `yaml:"database_staging"`
	Database_Local struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Pass     string `yaml:"pass"`
		Database string `yaml:"database"`
		DBName   string `yaml:"dbname"`
	} `yaml:"database_local"`
	Redis struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"redis"`
	Proxy struct {
		Uri []string `yaml:"uri"`
	} `yaml:"proxy"`
	Replica    string `yaml:"replica"`
	NeoFs_Main struct {
		Host        string `yaml:"host"`
		Port        string `yaml:"port"`
		ContainerId string `yaml:"containerid"`
	} `yaml:"neofs_main"`
	NeoFs_Test struct {
		Host        string `yaml:"host"`
		Port        string `yaml:"port"`
		ContainerId string `yaml:"containerid"`
	} `yaml:"neofs_test"`
}

type docParam struct {
	Name     string
	Type     string
	Required bool
}

type docMethod struct {
	Params []docParam
}

type sourceMethod struct {
	Params []docParam
}

var (
	docMethodsOnce    sync.Once
	docMethodsMap     map[string]docMethod
	sourceMethodsOnce sync.Once
	sourceMethodsMap  map[string]sourceMethod
)

func main() {
	log2.InitLog(log2.DebugLog, "./Logs/", os.Stdout)
	log2.Infof("YOUR ENV IS %s", os.ExpandEnv("${RUNTIME}"))
	cfg, err := OpenConfigFile()
	if err != nil {
		log2.Fatalf("open file error:%s", err)
	}
	ctx := context.TODO()
	co, dbOnline := initializeMongoOnlineClient(cfg, ctx)
	cl := initializeMongoLocalClient(cfg, ctx)
	rds := initializeRedisLocalClient(cfg, ctx)
	fs := initializeNeoFsHost(cfg)

	client := &cli.T{
		Redis:     rds,
		Db_online: dbOnline,
		C_online:  co,
		C_local:   cl,
		Ctx:       ctx,
		RpcCli:    neoRpc.NewClient(""), // placeholder
		RpcPorts:  cfg.Proxy.Uri,
		NeoFs:     fs,
	}

	rpc.Register(&api.T{
		Client: client,
	})

	j := &job.T{
		Client: client,
	}

	w := &watch.T{
		Client: client,
	}

	h := &joh.T{}
	v := &verify.T{
		Client: client,
	}
	//go j.UpdateMarketNFTLatestTransaction()
	// reset qps
	go func() {
		for {
			monitor.Http_request_qps.Set(0)
			time.Sleep(1 * time.Second)
		}
	}()

	if cfg.Replica == "master" {
		go func() {
			err := w.GetFirstEventByTransactionHash()
			if err != nil {
				log2.Fatalf("run watching error:%v", err)
			}
		}()

		c1 := cron.New()
		c2 := cron.New()
		c3 := cron.New()

		err = c1.AddFunc("@daily", func() {
			log2.Infof("Start daily job")
			go j.GetPopularTokens()
			go j.GetDailyTransactions()
			go j.GetNewAddresses()
			go j.GetActiveAddresses()
			go j.GetMarketDailyVolume() //获取market 前一天的交易数据
		})
		err = c2.AddFunc("@hourly", func() { //@hourly
			log2.Infof("Start hourly job")
			go j.GetHoldersByContractHash()
			go j.GetTransactionList()
			go j.GetBlockInfoList()
			go j.GetHourlyTransactions()
			go j.GetMarketHourlyVolume() //获取market当天的交易数据
		})

		err = c3.AddFunc("@every 10m", func() {
			log2.Infof("Start mintnue job")
			go j.GetMarketSupply()
			go j.GetMarketTxAmount()
			go j.GetMarketOwnerCount()
			go j.GetNFTFloorPrice()
			//go j.GetNFTIndex()
		})
		if err != nil {
			log2.Fatal("add job function error:%s", err)
		}
		c1.Start()
		c2.Start()
		c3.Start()
	}

	listen := os.ExpandEnv("0.0.0.0:1926")
	log2.Infof("NOW LISTEN ON: %s", listen)
	mux := http.NewServeMux()
	mux.HandleFunc("/swagger", func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/swagger/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/swagger/", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, "./swagger/ui.html")
	})
	mux.HandleFunc("/swagger/rpc/", func(writer http.ResponseWriter, request *http.Request) {
		swaggerRPCProxy(h, writer, request)
	})
	mux.HandleFunc("/swagger/openapi.yaml", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/yaml")
		doc, err := buildSwaggerOpenAPI()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = writer.Write(doc)
	})
	mux.HandleFunc("/swagger/openapi.json", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		doc, err := buildSwaggerOpenAPIJSON()
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = writer.Write(doc)
	})
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		monitor.Http_request_qps.Inc()
		monitor.Http_request_total.Inc()
		monitor.Http_request_in_flight.Inc()
		defer monitor.Http_request_in_flight.Dec()
		monitor.Http_request_duration_seconds.Observe(time.Since(time.Now()).Seconds())
		h.ServeHTTP(writer, request)
	})
	mux.HandleFunc("/upload", func(writer http.ResponseWriter, request *http.Request) {
		v.MultipleFile(writer, request)
	})
	mux.Handle("/metrics", promhttp.Handler())
	handler := cors.Default().Handler(mux)
	err = http.ListenAndServe(listen, handler)
	if err != nil {
		log2.Fatalf("listen and server error:%s", err)
	}
}

func swaggerRPCProxy(h *joh.T, writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	method := strings.TrimPrefix(request.URL.Path, "/swagger/rpc/")
	if method == "" {
		http.Error(writer, "empty method", http.StatusBadRequest)
		return
	}
	var params interface{} = map[string]interface{}{}
	jsonrpc := "2.0"
	idVal := interface{}(1)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(bytes.TrimSpace(body)) > 0 {
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(writer, "request body must be valid JSON", http.StatusBadRequest)
			return
		}
		// Support both:
		// 1) full JSON-RPC body {jsonrpc, method, params, id}
		// 2) params-only body {...}
		if _, hasMethod := payload["method"]; hasMethod || payload["params"] != nil || payload["jsonrpc"] != nil || payload["id"] != nil {
			if v, ok := payload["jsonrpc"].(string); ok && v != "" {
				jsonrpc = v
			}
			if v, ok := payload["id"]; ok {
				idVal = v
			}
			if v, ok := payload["params"]; ok {
				params = v
			}
		} else {
			params = payload
		}
	}
	rpcReq := map[string]interface{}{
		"jsonrpc": jsonrpc,
		"method":  method,
		"params":  params,
		"id":      idVal,
	}
	payload, err := json.Marshal(rpcReq)
	if err != nil {
		http.Error(writer, "build rpc request failed", http.StatusInternalServerError)
		return
	}
	proxyReq := request.Clone(request.Context())
	proxyReq.Method = http.MethodPost
	proxyReq.URL.Path = "/"
	proxyReq.Body = io.NopCloser(bytes.NewReader(payload))
	proxyReq.ContentLength = int64(len(payload))
	proxyReq.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(writer, proxyReq)
}

func buildSwaggerOpenAPI() ([]byte, error) {
	doc := buildSwaggerOpenAPIDoc()
	return yaml.Marshal(doc)
}

func buildSwaggerOpenAPIDoc() map[string]interface{} {
	paths := map[string]interface{}{
		"/": map[string]interface{}{
			"post": map[string]interface{}{
				"tags":        []string{"JSON-RPC"},
				"summary":     "JSON-RPC entrypoint",
				"description": "Raw JSON-RPC 2.0 endpoint.",
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/JsonRpcRequest",
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "JSON-RPC response",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/JsonRpcResponse",
								},
							},
						},
					},
				},
			},
		},
		"/metrics": map[string]interface{}{
			"get": map[string]interface{}{
				"tags":    []string{"Ops"},
				"summary": "Prometheus metrics",
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Metrics text",
					},
				},
			},
		},
	}

	methods := append([]string{}, config.Apis...)
	sort.Strings(methods)
	for _, method := range methods {
		path := "/swagger/rpc/" + method
		paramsExample, paramsRequired, paramSource := defaultParamsForMethod(method)
		// Hide optional Filter from Swagger UI request params.
		delete(paramsExample, "Filter")
		delete(paramsExample, "filter")
		paramsRequired = removeParamIgnoreCase(paramsRequired, "Filter")
		paramsProperties := map[string]interface{}{}
		for k, v := range paramsExample {
			if strings.EqualFold(k, "Filter") {
				continue
			}
			paramsProperties[k] = inferredSchemaForValue(v)
		}
		desc := "Try this RPC method directly from Swagger. Request body uses standard JSON-RPC fields."
		if paramSource != "doc" && paramSource != "override" {
			desc += " Parameter template is pending confirmation (generated from code signature)."
		}
		paths[path] = map[string]interface{}{
			"post": map[string]interface{}{
				"tags":        []string{"RPC"},
				"summary":     method,
				"description": desc,
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"jsonrpc": map[string]interface{}{"type": "string", "example": "2.0"},
									"method":  map[string]interface{}{"type": "string", "example": method},
									"params": map[string]interface{}{
										"type":                 "object",
										"additionalProperties": true,
										"properties":           paramsProperties,
										"required":             paramsRequired,
									},
									"id": map[string]interface{}{
										"oneOf": []map[string]interface{}{
											{"type": "integer"},
											{"type": "string"},
										},
									},
								},
								"required": []string{"jsonrpc", "method", "params", "id"},
							},
							"example": map[string]interface{}{
								"jsonrpc": "2.0",
								"method":  method,
								"params":  paramsExample,
								"id":      1,
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "JSON-RPC wrapped response",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/JsonRpcResponse",
								},
							},
						},
					},
				},
			},
		}
	}

	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "Neo3Fura RPC Tryout API",
			"version":     "1.0.0",
			"description": "Swagger exposes each RPC method at /swagger/rpc/{Method}.",
		},
		"servers": []map[string]interface{}{
			{"url": "http://127.0.0.1:1926"},
		},
		"paths": paths,
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"JsonRpcRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"jsonrpc": map[string]interface{}{"type": "string", "example": "2.0"},
						"method":  map[string]interface{}{"type": "string", "example": "GetTransactionList"},
						"params":  map[string]interface{}{"type": "object", "additionalProperties": true},
						"id": map[string]interface{}{
							"oneOf": []map[string]interface{}{
								{"type": "integer"},
								{"type": "string"},
							},
						},
					},
					"required": []string{"jsonrpc", "method", "params", "id"},
				},
				"JsonRpcResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"oneOf": []map[string]interface{}{
								{"type": "integer"},
								{"type": "string"},
							},
						},
						"result": map[string]interface{}{"type": "object", "nullable": true, "additionalProperties": true},
						"error": map[string]interface{}{
							"nullable": true,
							"oneOf": []map[string]interface{}{
								{"type": "string"},
								{"type": "object"},
							},
						},
					},
					"required": []string{"id"},
				},
			},
		},
	}
}

func buildSwaggerOpenAPIJSON() ([]byte, error) {
	doc := buildSwaggerOpenAPIDoc()
	return json.MarshalIndent(doc, "", "  ")
}

func inferredSchemaForValue(v interface{}) map[string]interface{} {
	switch v.(type) {
	case int, int32, int64:
		return map[string]interface{}{"type": "integer"}
	case bool:
		return map[string]interface{}{"type": "boolean"}
	case map[string]interface{}:
		return map[string]interface{}{"type": "object", "additionalProperties": true}
	case []interface{}:
		return map[string]interface{}{"type": "array"}
	default:
		return map[string]interface{}{"type": "string"}
	}
}

func defaultParamsForMethod(method string) (map[string]interface{}, []string, string) {
	// Explicit method overrides take highest priority.
	// GetAddressCount has no params by design.
	switch method {
	case "GetAddressCount":
		return map[string]interface{}{}, []string{}, "override"
	case "GetAddressByAddress", "GetAddressInfoByAddress":
		return map[string]interface{}{
			"Address": "0x0bf916d727c75f2e51e1ab2c476304513da59701",
		}, []string{"Address"}, "override"
	}

	if m, ok := getDocMethods()[normalizeMethodKey(method)]; ok {
		params := map[string]interface{}{}
		required := []string{}
		for _, p := range m.Params {
			params[p.Name] = sampleValueForParam(method, p.Name, p.Type)
			if p.Required {
				required = append(required, p.Name)
			}
		}
		return params, required, "doc"
	}

	if m, ok := getSourceMethods()[normalizeMethodKey(method)]; ok {
		params := map[string]interface{}{}
		required := []string{}
		for _, p := range m.Params {
			params[p.Name] = sampleValueForParam(method, p.Name, p.Type)
			if isLikelyRequiredSourceParam(p.Name, p.Type) {
				required = append(required, p.Name)
			}
		}
		if len(params) == 0 {
			params = map[string]interface{}{}
		}
		return params, required, "source"
	}

	// Fallback if docs are missing for a method.
	params := map[string]interface{}{}
	required := []string{}
	addRequired := func(key string, val interface{}) {
		if _, ok := params[key]; !ok {
			params[key] = val
		}
		for _, it := range required {
			if it == key {
				return
			}
		}
		required = append(required, key)
	}
	addOptional := func(key string, val interface{}) {
		if _, ok := params[key]; !ok {
			params[key] = val
		}
	}

	lm := strings.ToLower(method)
	if strings.Contains(lm, "address") {
		switch {
		case strings.Contains(lm, "candidatebyaddress"), strings.Contains(lm, "candidateaddress"):
			addRequired("CandidateAddress", "0x0bf916d727c75f2e51e1ab2c476304513da59701")
		case strings.Contains(lm, "voteraddress"):
			addRequired("VoterAddress", "0x0bf916d727c75f2e51e1ab2c476304513da59701")
		default:
			addRequired("Address", "0x0bf916d727c75f2e51e1ab2c476304513da59701")
		}
	}
	if strings.Contains(lm, "transactionhash") {
		addRequired("TransactionHash", "0x872fec7575dbbedb99d72055efcb96517d3165c098129211aeeeea2e6cacff8c")
	}
	if strings.Contains(lm, "blockhash") {
		addRequired("BlockHash", "0x3a5fdece2a3a2d05667a0a35fba09572517bb46dd57b1b99d4121463b16ca6a0")
	}
	if strings.Contains(lm, "blockheight") {
		addRequired("BlockHeight", 1)
	}
	if strings.Contains(lm, "contracthash") {
		addRequired("ContractHash", "0xd2a4cff31913016155e38e474a2c06d08be276cf")
	}
	if strings.Contains(lm, "tokenid") {
		addRequired("TokenId", "1")
	}
	if strings.Contains(lm, "markethash") {
		addRequired("MarketHash", "0x1f594c26a50d25d22d8afc3f1843b4ddb17cf180")
	}

	if strings.Contains(lm, "list") ||
		strings.Contains(lm, "transfer") ||
		strings.Contains(lm, "owned") ||
		strings.Contains(lm, "market") ||
		strings.Contains(lm, "history") {
		addOptional("Limit", 20)
		addOptional("Cursor", "")
	}

	if len(params) == 0 {
		params = map[string]interface{}{}
	}
	return params, required, "heuristic"
}

func getDocMethods() map[string]docMethod {
	docMethodsOnce.Do(func() {
		docMethodsMap = map[string]docMethod{}
		loadDocMethodsFromDir("./docs/api")
		loadDocMethodsFromDir("./docs/market")
	})
	return docMethodsMap
}

func getSourceMethods() map[string]sourceMethod {
	sourceMethodsOnce.Do(func() {
		sourceMethodsMap = map[string]sourceMethod{}
		loadSourceMethodsFromDir("./biz/api")
	})
	return sourceMethodsMap
}

func loadSourceMethodsFromDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			loadSourceMethodsFromDir(path)
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".go") {
			continue
		}
		loadSourceMethodsFromFile(path)
	}
}

func loadSourceMethodsFromFile(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	txt := string(raw)
	reMethod := regexp.MustCompile(`func\s+\(me\s+\*T\)\s+([A-Za-z0-9_]+)\s*\(\s*args\s+struct\s*\{([\s\S]*?)\}\s*,\s*ret\s+\*json\.RawMessage\s*\)\s*error`)
	matches := reMethod.FindAllStringSubmatch(txt, -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		methodName := m[1]
		fieldsBlock := m[2]
		params := parseStructFields(fieldsBlock)
		sourceMethodsMap[normalizeMethodKey(methodName)] = sourceMethod{Params: params}
	}
}

func parseStructFields(block string) []docParam {
	out := []docParam{}
	lines := strings.Split(block, "\n")
	reField := regexp.MustCompile(`^\s*([A-Za-z0-9_]+)\s+([^\s` + "`" + `]+)`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		sm := reField.FindStringSubmatch(line)
		if len(sm) < 3 {
			continue
		}
		name := strings.TrimSpace(sm[1])
		typ := strings.TrimSpace(sm[2])
		if name == "" {
			continue
		}
		out = append(out, docParam{
			Name:     name,
			Type:     typ,
			Required: false,
		})
	}
	return out
}

func loadDocMethodsFromDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		methodName, methodSpec, ok := parseDocMethodFile(path)
		if !ok || methodName == "" {
			continue
		}
		docMethodsMap[normalizeMethodKey(methodName)] = methodSpec
	}
}

func parseDocMethodFile(path string) (string, docMethod, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", docMethod{}, false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	methodName := ""
	inParams := false
	seenTableHeader := false
	params := []docParam{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") && methodName == "" {
			methodName = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			continue
		}
		if strings.EqualFold(line, "### Parameters") || strings.EqualFold(line, "#### Body Parameters") {
			inParams = true
			seenTableHeader = false
			continue
		}
		if inParams {
			lc := strings.ToLower(line)
			if strings.HasPrefix(lc, "### ") || strings.HasPrefix(lc, "#### example") || strings.HasPrefix(lc, "### example") {
				break
			}
		}
		if !inParams {
			continue
		}
		if strings.EqualFold(line, "None") {
			return methodName, docMethod{Params: []docParam{}}, true
		}
		if !strings.HasPrefix(line, "|") {
			continue
		}
		parts := splitMarkdownTableLine(line)
		if len(parts) < 4 {
			continue
		}
		// Skip table header and separator row.
		if !seenTableHeader {
			seenTableHeader = true
			continue
		}
		if strings.Contains(parts[0], "---") {
			continue
		}
		name := strings.TrimSpace(parts[0])
		typ := strings.TrimSpace(parts[1])
		reqCell := strings.TrimSpace(parts[len(parts)-1])
		required := isRequiredCell(reqCell)
		if name == "" {
			continue
		}
		params = append(params, docParam{
			Name:     name,
			Type:     typ,
			Required: required,
		})
	}
	if methodName == "" {
		return "", docMethod{}, false
	}
	return methodName, docMethod{Params: params}, true
}

func splitMarkdownTableLine(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	raw := strings.Split(trimmed, "|")
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		out = append(out, strings.TrimSpace(it))
	}
	return out
}

func isRequiredCell(cell string) bool {
	lc := strings.ToLower(cell)
	if strings.Contains(lc, "optional") {
		return false
	}
	return strings.Contains(lc, "required")
}

func normalizeMethodKey(method string) string {
	return strings.ToLower(strings.TrimSpace(method))
}

func sampleValueForParam(method, name, typ string) interface{} {
	ln := strings.ToLower(name)
	lt := strings.ToLower(typ)
	if ln == "cursor" {
		return ""
	}
	if ln == "filter" || ln == "raw" {
		return map[string]interface{}{}
	}
	if ln == "order" {
		return -1
	}
	if ln == "limit" {
		return 20
	}
	if ln == "skip" {
		return 0
	}
	if ln == "start" || ln == "end" {
		return 0
	}
	if strings.Contains(ln, "exclude") {
		return false
	}
	if strings.Contains(ln, "address") {
		return "0x0bf916d727c75f2e51e1ab2c476304513da59701"
	}
	if strings.Contains(ln, "hash") {
		return "0xd2a4cff31913016155e38e474a2c06d08be276cf"
	}
	if strings.Contains(ln, "tokenid") || strings.Contains(ln, "tokenid") {
		return "1"
	}
	if strings.Contains(lt, "int") {
		return 0
	}
	if strings.Contains(lt, "bool") {
		return false
	}
	return ""
}

func isLikelyRequiredSourceParam(name, typ string) bool {
	ln := strings.ToLower(strings.TrimSpace(name))
	lt := strings.ToLower(strings.TrimSpace(typ))
	if ln == "" {
		return false
	}
	// Common optional control fields across query APIs.
	switch ln {
	case "filter", "limit", "skip", "cursor", "order", "raw", "excludeupdated":
		return false
	}
	// Map-like fields are usually optional payload extensions.
	if strings.Contains(lt, "map[") {
		return false
	}
	return true
}

func removeParamIgnoreCase(items []string, target string) []string {
	if len(items) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if strings.EqualFold(strings.TrimSpace(it), target) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func initializeMongoOnlineClient(cfg Config, ctx context.Context) (*mongo.Client, string) {
	rt := os.ExpandEnv("${RUNTIME}")
	var clientOptions *options.ClientOptions
	var dbOnline string
	switch rt {
	case "dev":
		clientOptions = options.Client().ApplyURI("mongodb://" + cfg.Database_Dev.User + ":" + cfg.Database_Dev.Pass + "@" + cfg.Database_Dev.Host + ":" + cfg.Database_Dev.Port + "/" + cfg.Database_Dev.Database)
		dbOnline = cfg.Database_Dev.Database
	case "test":
		clientOptions = options.Client().ApplyURI("mongodb://" + cfg.Database_Test.User + ":" + cfg.Database_Test.Pass + "@" + cfg.Database_Test.Host + ":" + cfg.Database_Test.Port + "/" + cfg.Database_Test.Database)
		dbOnline = cfg.Database_Test.Database
	case "test2":
		clientOptions = options.Client().ApplyURI("mongodb://" + cfg.Database_Test.User + ":" + cfg.Database_Test.Pass + "@" + cfg.Database_Test.Host + ":" + cfg.Database_Test.Port + "/" + cfg.Database_Test.Database)
		dbOnline = cfg.Database_Test.Database
	case "staging":
		clientOptions = options.Client().ApplyURI("mongodb://" + cfg.Database_Staging.User + ":" + cfg.Database_Staging.Pass + "@" + cfg.Database_Staging.Host + ":" + cfg.Database_Staging.Port + "/" + cfg.Database_Staging.Database)
		dbOnline = cfg.Database_Staging.Database
	default:
		log2.Fatalf("runtime environment mismatch: RUNTIME=%s (expected: dev/test/test2/staging)", rt)
		os.Exit(1)
	}
	if clientOptions == nil {
		log2.Fatalf("mongo client options is nil, RUNTIME=%s", rt)
		os.Exit(1)
	}

	clientOptions.SetMaxPoolSize(50)
	co, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log2.Fatalf("mongo connect error:%s", err)
	}
	err = co.Ping(ctx, nil)
	if err != nil {
		log2.Fatalf("ping mongo error:%s", err)
	}
	return co, dbOnline
}
func initializeNeoFsHost(cfg Config) string {
	rt := os.ExpandEnv("${RUNTIME}")
	var neoFsHost string
	switch rt {
	case "test":
		neoFsHost = cfg.NeoFs_Test.Host + ":" + cfg.NeoFs_Test.Port + "/gate" + "/get/" + cfg.NeoFs_Test.ContainerId + "/"
	case "staging":
		neoFsHost = cfg.NeoFs_Main.Host + ":" + cfg.NeoFs_Main.Port + "/gate" + "/get/" + cfg.NeoFs_Main.ContainerId + "/"
	default:
		log2.Fatalf("runtime environment mismatch: RUNTIME=%s (expected: test/staging)", rt)
		os.Exit(1)
	}
	return neoFsHost
}
func initializeMongoLocalClient(cfg Config, ctx context.Context) *mongo.Client {
	var clientOptions *options.ClientOptions
	clientOptions = options.Client().ApplyURI("mongodb://" + cfg.Database_Local.Host + ":" + cfg.Database_Local.Port + "/" + cfg.Database_Local.Database)
	cl, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log2.Fatalf("connect to mongo error:%s", err)
	}
	err = cl.Ping(ctx, nil)
	if err != nil {
		log2.Fatalf("ping mongo error:%s", err)
	}
	return cl
}

func initializeRedisLocalClient(cfg Config, ctx context.Context) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return rdb
}
