package cli

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"sort"
	"strings"


	"github.com/go-redis/redis/v8"
	"github.com/joeqian10/neo3-gogogo/rpc"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	log2 "neo3fura_http/lib/log"
	"neo3fura_http/lib/type/consts"
	"neo3fura_http/lib/type/h160"
	"neo3fura_http/var/stderr"
)

// T ...
type T struct {
	Redis     *redis.Client
	Db_online string
	C_online  *mongo.Client
	C_local   *mongo.Client
	Ctx       context.Context
	RpcCli    *rpc.RpcClient
	RpcPorts  []string
	NeoFs     string
}

type Insert struct {
	Hash          h160.T
	Id            int32
	UpdateCounter int32
}

type SourceCode struct {
	Hash          h160.T
	Updatecounter int32
	Filename      string
	Code          string
}


func sortedBsonKeys(m bson.M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func normalizeLimit(limit int64) int64 {
	defaultLimit := configuredInt64Env("NEO3FURA_DEFAULT_LIMIT", consts.DefaultLimit)
	maxLimit := configuredInt64Env("NEO3FURA_MAX_LIMIT", consts.MaxLimit)
	if maxLimit < 1 {
		maxLimit = consts.MaxLimit
	}
	if defaultLimit < 1 {
		defaultLimit = consts.DefaultLimit
	}
	if defaultLimit > maxLimit {
		defaultLimit = maxLimit
	}
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func normalizeSkip(skip int64) int64 {
	if skip < 0 {
		return 0
	}
	return skip
}

func asInt64(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int8:
		return int64(t), true
	case int16:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case uint:
		return int64(t), true
	case uint8:
		return int64(t), true
	case uint16:
		return int64(t), true
	case uint32:
		return int64(t), true
	case uint64:
		if t > uint64(9223372036854775807) {
			return 0, false
		}
		return int64(t), true
	case float32:
		return int64(t), true
	case float64:
		return int64(t), true
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func normalizePipelineLimit(pipeline []bson.M) {
	for _, stage := range pipeline {
		rawLimit, ok := stage["$limit"]
		if !ok {
			continue
		}
		limit, ok := asInt64(rawLimit)
		if !ok {
			continue
		}
		stage["$limit"] = normalizeLimit(limit)
	}
}

func configuredInt64Env(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func queryTimeoutDuration() time.Duration {
	ms := configuredInt64Env("NEO3FURA_QUERY_TIMEOUT_MS", 15000)
	if ms <= 0 {
		ms = 15000
	}
	return time.Duration(ms) * time.Millisecond

}

func (me *T) GetCollection(args struct {
	Collection string
}) (*mongo.Collection, error) {
	collection := me.C_online.Database(me.Db_online).Collection(args.Collection)
	return collection, nil
}

func (me *T) QueryOne(args struct {
	Collection string
	Index      string
	Sort       bson.M
	Filter     bson.M
	Query      []string
}, ret *json.RawMessage) (map[string]interface{}, error) {
	var sb strings.Builder
	sb.WriteString(args.Collection)
	sb.WriteString(args.Index)
	for _, k := range sortedBsonKeys(args.Sort) {
		v := args.Sort[k]
		sb.WriteString(k)
		fmt.Fprintf(&sb, "%v", v)
	}
	for _, k := range sortedBsonKeys(args.Filter) {
		v := args.Filter[k]
		sb.WriteString(k)
		fmt.Fprintf(&sb, "%v", v)
	}
	for _, v := range args.Query {
		sb.WriteString(v)
	}
	h := sha1.New()
	h.Write([]byte(sb.String()))
	hash := hex.EncodeToString(h.Sum(nil))
	val, err := me.Redis.Get(me.Ctx, hash).Result()
	// if sort != nil, it may have several results, we have to pick the sorted one
	if err != nil || len(args.Sort) != 0 || args.Index == "GetCandidateByAddress" || args.Index == "GetAssetInfoByContractHash" || args.Index == "GetVerifiedContractByContractHash" || args.Index == "GetVotesByCandidateAddress" {
		var result map[string]interface{}
		convert := make(map[string]interface{})
		collection := me.C_online.Database(me.Db_online).Collection(args.Collection)
		opts := options.FindOne().SetSort(args.Sort)
		err = collection.FindOne(me.Ctx, args.Filter, opts).Decode(&result)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, stderr.ErrNotFound
		} else if err != nil {
			return nil, stderr.ErrFind
		}
		if len(args.Query) == 0 {
			convert = result
		} else {
			for _, v := range args.Query {
				convert[v] = result[v]
			}
		}
		r, err := json.Marshal(convert)
		if err != nil {
			return nil, stderr.ErrFind
		}
		err = me.Redis.Set(me.Ctx, hash, hex.EncodeToString(r), 0).Err()
		if err != nil {
			log2.Infof("Redis cache set error: %v", err)
		}
		*ret = json.RawMessage(r)
		return convert, nil
	} else {
		r, err := hex.DecodeString(val)
		if err != nil {
			return nil, stderr.ErrFind
		}

		*ret = json.RawMessage(r)
		convert := make(map[string]interface{})
		err = json.Unmarshal(r, &convert)
		if convert["_id"] != nil {
			convert["_id"], err = primitive.ObjectIDFromHex(convert["_id"].(string))
		}
		if err != nil {
			return nil, stderr.ErrFind
		}
		return convert, nil
	}
}

func (me *T) QueryAll(args struct {
	Collection string
	Index      string
	Sort       bson.M
	Filter     bson.M
	Query      []string
	Limit      int64
	Skip       int64
}, ret *json.RawMessage) ([]map[string]interface{}, int64, error) {
	args.Limit = normalizeLimit(args.Limit)
	args.Skip = normalizeSkip(args.Skip)

	var results []map[string]interface{}
	convert := make([]map[string]interface{}, 0)
	collection := me.C_online.Database(me.Db_online).Collection(args.Collection)
	queryCtx, cancel := context.WithTimeout(me.Ctx, queryTimeoutDuration())
	defer cancel()
	op := options.Find()
	op.SetSort(args.Sort)
	op.SetLimit(args.Limit)
	op.SetSkip(args.Skip)
	var (
		count int64
		err   error
	)
	if len(args.Filter) == 0 {
		count, err = collection.EstimatedDocumentCount(queryCtx)
	} else {
		co := options.CountOptions{}
		count, err = collection.CountDocuments(queryCtx, args.Filter, &co)
	}
	if err != nil && err != context.DeadlineExceeded {
		log2.Warnf("count fallback to CountDocuments for %s due to err=%v", args.Collection, err)
		co := options.CountOptions{}
		count, err = collection.CountDocuments(queryCtx, args.Filter, &co)
	}
	if err != nil {
		return nil, 0, stderr.ErrFind
	}

	cursor, err := collection.Find(me.Ctx, args.Filter, op)
	if err == mongo.ErrNoDocuments {

		return nil, 0, stderr.ErrNotFound
	}
	if err != nil {
		return nil, 0, stderr.ErrFind
	}
	defer func() {

		if err := cursor.Close(me.Ctx); err != nil {
			log2.Errorf("Closing cursor error %v", err)
		}
	}()
	if err = cursor.All(me.Ctx, &results); err != nil {

		return nil, 0, stderr.ErrFind
	}
	for _, item := range results {
		if len(args.Query) == 0 {
			convert = append(convert, item)
		} else {
			temp := make(map[string]interface{})
			for _, v := range args.Query {
				temp[v] = item[v]
			}
			convert = append(convert, temp)
		}
	}
	r, err := json.Marshal(convert)
	if err != nil {
		return nil, 0, stderr.ErrFind
	}
	*ret = json.RawMessage(r)
	return convert, count, nil
}

func (me *T) SaveJob(args struct {
	Collection string
	Data       bson.M
}) (bool, error) {
	collection := me.C_local.Database("job").Collection(args.Collection)
	_, err := collection.InsertOne(me.Ctx, args.Data)
	if err != nil {
		return false, stderr.ErrInsert
	}
	return true, nil
}

func (me *T) SaveManyJob(args struct {
	Collection string
	Data       []interface{}
}) (bool, error) {
	collection := me.C_local.Database("job").Collection(args.Collection)
	//opts := options.InsertMany().SetOrdered(false)
	_, err := collection.InsertMany(me.Ctx, args.Data)
	if err != nil {
		return false, stderr.ErrInsert
	}
	return true, nil
}

func (me *T) UpdateJob(args struct {
	Collection string
	Data       bson.M
	Filter     bson.M
}) (bool, error) {
	collection := me.C_local.Database("job").Collection(args.Collection)
	var result map[string]interface{}
	err := collection.FindOne(me.Ctx, args.Filter).Decode(&result)
	if err != nil && err.Error() != "mongo: no documents in result" {
		return false, stderr.ErrInsert
	}
	var filter bson.M
	if len(result) > 0 {
		id := result["_id"].(primitive.ObjectID)
		filter = bson.M{"_id": id}
		update := bson.M{"$set": args.Data}
		opts := options.Update().SetUpsert(true)
		_, err = collection.UpdateOne(me.Ctx, filter, update, opts)
		if err != nil {
			return false, err
		}
	} else {
		args.Data["asset"] = args.Filter["asset"]
		_, err := collection.InsertOne(me.Ctx, args.Data)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func (me *T) QueryOneJob(args struct {
	Collection string
	Filter     bson.M
}) (map[string]interface{}, error) {
	collection := me.C_local.Database("job").Collection(args.Collection)
	var result map[string]interface{}
	err := collection.FindOne(me.Ctx, args.Filter).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (me *T) UpdateOneJob(args struct {
	Collection string
	Filter     bson.M
}) (map[string]interface{}, error) {
	collection := me.C_local.Database("job").Collection(args.Collection)

	collection.Indexes().List(context.TODO())
	var result map[string]interface{}
	err := collection.FindOne(me.Ctx, args.Filter).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (me *T) QueryLastJob(args struct {
	Collection string
}) (map[string]interface{}, error) {
	collection := me.C_local.Database("job").Collection(args.Collection)
	var result map[string]interface{}
	opts := options.FindOne().SetSort(bson.M{"_id": -1})
	err := collection.FindOne(me.Ctx, bson.M{}, opts).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return nil, stderr.ErrNotFound
	}
	if err != nil {
		return nil, stderr.ErrFind
	}
	return result, nil
}

func (me *T) QueryLastJobs(args struct {
	Collection string
	Index      string
	Sort       bson.M
	Filter     bson.M
	Query      []string
	Limit      int64
	Skip       int64
}) ([]map[string]interface{}, error) {
	collection := me.C_local.Database("job").Collection(args.Collection)
	var results []map[string]interface{}
	args.Limit = normalizeLimit(args.Limit)
	args.Skip = normalizeSkip(args.Skip)
	queryCtx, cancel := context.WithTimeout(me.Ctx, queryTimeoutDuration())
	defer cancel()
	//
	op := options.Find()
	op.SetSort(args.Sort)
	op.SetLimit(args.Limit)
	op.SetSkip(args.Skip)
	cursor, err := collection.Find(me.Ctx, args.Filter, op)
	if err == mongo.ErrNoDocuments {
		return nil, stderr.ErrNotFound
	}
	if err != nil {
		return nil, stderr.ErrFind
	}
	defer func() {
		if err := cursor.Close(me.Ctx); err != nil {
			log2.Errorf("Closing cursor error %v", err)
		}
	}()
	if err = cursor.All(me.Ctx, &results); err != nil {
		return nil, stderr.ErrFind
	}
	return results, nil
}

func (me *T) QueryAggregate(args struct {
	Collection string
	Index      string
	Sort       bson.M
	Filter     bson.M
	Pipeline   []bson.M
	Query      []string
}, ret *json.RawMessage) ([]map[string]interface{}, error) {
	normalizePipelineLimit(args.Pipeline)

	var results []map[string]interface{}
	convert := make([]map[string]interface{}, 0)
	collection := me.C_online.Database(me.Db_online).Collection(args.Collection)
	queryCtx, cancel := context.WithTimeout(me.Ctx, queryTimeoutDuration())
	defer cancel()
	op := options.AggregateOptions{}
	op.SetAllowDiskUse(true)

	cursor, err := collection.Aggregate(me.Ctx, args.Pipeline, &op)
	if err == mongo.ErrNoDocuments {
		return nil, stderr.ErrNotFound
	}
	if err != nil {
		return nil, stderr.ErrFind
	}
	defer func() {
		if err := cursor.Close(me.Ctx); err != nil {
			log2.Errorf("Closing cursor error %v", err)
		}
	}()
	if err = cursor.All(me.Ctx, &results); err != nil {
		return nil, stderr.ErrFind
	}

	for _, item := range results {
		if len(args.Query) == 0 {
			convert = append(convert, item)
		} else {
			temp := make(map[string]interface{})
			for _, v := range args.Query {
				temp[v] = item[v]
			}
			convert = append(convert, temp)
		}
	}

	r, err := json.Marshal(convert)
	if err != nil {
		return nil, stderr.ErrFind
	}
	*ret = json.RawMessage(r)
	return convert, nil
}

func (me *T) QueryAggregateJob(args struct {
	Collection string
	Index      string
	Sort       bson.M
	Filter     bson.M
	Pipeline   []bson.M
	Query      []string
}, ret *json.RawMessage) ([]map[string]interface{}, error) {

	var results []map[string]interface{}
	convert := make([]map[string]interface{}, 0)
	collection := me.C_local.Database("job").Collection(args.Collection)
	normalizePipelineLimit(args.Pipeline)
	queryCtx, cancel := context.WithTimeout(me.Ctx, queryTimeoutDuration())
	defer cancel()
	op := options.AggregateOptions{}

	cursor, err := collection.Aggregate(me.Ctx, args.Pipeline, &op)
	if err == mongo.ErrNoDocuments {
		return nil, stderr.ErrNotFound
	}
	if err != nil {
		return nil, stderr.ErrFind
	}
	defer func() {
		if err := cursor.Close(me.Ctx); err != nil {
			log2.Errorf("Closing cursor error %v", err)
		}
	}()
	if err = cursor.All(me.Ctx, &results); err != nil {
		return nil, stderr.ErrFind
	}

	for _, item := range results {
		if len(args.Query) == 0 {
			convert = append(convert, item)
		} else {
			temp := make(map[string]interface{})
			for _, v := range args.Query {
				temp[v] = item[v]
			}
			convert = append(convert, temp)
		}
	}

	r, err := json.Marshal(convert)
	if err != nil {
		return nil, stderr.ErrFind
	}
	*ret = json.RawMessage(r)
	return convert, nil
}

func (me *T) QueryDocument(args struct {
	Collection string
	Index      string
	Sort       bson.M
	Filter     bson.M
}, ret *json.RawMessage) (map[string]interface{}, error) {
	collection := me.C_online.Database(me.Db_online).Collection(args.Collection)
	queryCtx, cancel := context.WithTimeout(me.Ctx, queryTimeoutDuration())
	defer cancel()
	var (
		count int64
		err   error
	)
	if len(args.Filter) == 0 {
		count, err = collection.EstimatedDocumentCount(queryCtx)
	} else {
		co := options.CountOptions{}
		count, err = collection.CountDocuments(queryCtx, args.Filter, &co)
	}
	if err == mongo.ErrNoDocuments {
		return nil, stderr.ErrNotFound
	}
	if err != nil {
		return nil, stderr.ErrFind
	}
	convert := make(map[string]interface{})
	convert["total counts"] = count
	r, err := json.Marshal(convert)
	if err != nil {
		return nil, stderr.ErrFind
	}
	*ret = json.RawMessage(r)
	return convert, nil
}

// 去重查询统计
func (me *T) GetDistinctCount(args struct {
	Collection string
	Index      string
	Sort       bson.M
	Filter     bson.M
	Pipeline   []bson.M
	Query      []string
}, ret *json.RawMessage) (map[string]interface{}, error) {
	var results []map[string]interface{}
	convert := make(map[string]interface{})
	collection := me.C_online.Database(me.Db_online).Collection(args.Collection)
	op := options.AggregateOptions{}
	pipeline := bson.M{
		"$group": bson.M{"_id": "$hash"},
	}
	args.Pipeline = append(args.Pipeline, pipeline)
	args.Pipeline = append(args.Pipeline, bson.M{"$count": "count"})
	cursor, err := collection.Aggregate(me.Ctx, args.Pipeline, &op)
	if err == mongo.ErrNoDocuments {
		return nil, stderr.ErrNotFound
	}
	if err != nil {
		return nil, stderr.ErrFind
	}
	defer func() {
		if err := cursor.Close(me.Ctx); err != nil {
			log2.Errorf("Closing cursor error %v", err)
		}
	}()

	if err = cursor.All(me.Ctx, &results); err != nil {
		return nil, stderr.ErrFind
	}

	convert["total"] = results[0]["count"]

	r, err := json.Marshal(convert)
	if err != nil {
		return nil, stderr.ErrFind
	}
	*ret = json.RawMessage(r)

	return convert, nil

}

func (me *T) InsertDocument(args struct {
	Collection string
	Index      string
	Insert     *Insert
}, ret *json.RawMessage) (map[string]interface{}, error) {
	collection := me.C_online.Database(me.Db_online).Collection(args.Collection)
	_, err := collection.InsertOne(me.Ctx, &args.Insert)
	if err != nil {
		return nil, stderr.ErrInsertDocument
	}
	result := make(map[string]interface{})
	result["msg"] = "Insert document done!"
	r, err := json.Marshal(result)
	if err != nil {
		return nil, stderr.ErrInsertDocument
	}
	*ret = json.RawMessage(r)
	return result, nil
}

func (me *T) InsertSourceCode(args struct {
	Collection string
	Index      string
	Insert     *SourceCode
}, ret *json.RawMessage) (map[string]interface{}, error) {
	collection := me.C_online.Database(me.Db_online).Collection(args.Collection)
	_, err := collection.InsertOne(me.Ctx, &args.Insert)
	if err != nil {
		return nil, stderr.ErrInsertDocument
	}
	result := make(map[string]interface{})
	result["msg"] = "Insert document done!"
	r, err := json.Marshal(result)
	if err != nil {
		return nil, stderr.ErrInsertDocument
	}
	*ret = json.RawMessage(r)
	return result, nil
}
