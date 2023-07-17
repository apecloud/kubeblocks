package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/custom"

	"github.com/apecloud/kubeblocks/cmd/probe/internal/component"
	"github.com/go-errors/errors"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/etcd"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/mongodb"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/mysql"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/postgres"
	"github.com/apecloud/kubeblocks/cmd/probe/internal/binding/redis"
	"github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

var builtinMap = make(map[string]BaseInternalOps)
var customOp *custom.HTTPCustom

func RegisterBuiltin() error {

	createErrFmt := "%s create err: %v"
	initErrFmt := "%s init err: %v"

	if mysqlOp, err := mysql.NewMysql(); err != nil {
		return errors.Errorf(createErrFmt, "mysql", err)
	} else {
		builtinMap["mysql"] = mysqlOp
		properties := component.GetProperties("mysql")
		err := mysqlOp.Init(properties)
		if err != nil {
			return errors.Errorf(initErrFmt, "mysql", err)
		}
	}

	if redisOp, err := redis.NewRedis(); err != nil {
		return errors.Errorf(createErrFmt, "redis", err)
	} else {
		builtinMap["redis"] = redisOp
		properties := component.GetProperties("redis")
		err := redisOp.Init(properties)
		if err != nil {
			return errors.Errorf(initErrFmt, "redis", err)
		}
	}

	if pgOp, err := postgres.NewPostgres(); err != nil {
		return errors.Errorf(createErrFmt, "postgres", err)
	} else {
		builtinMap["postgres"] = pgOp
		properties := component.GetProperties("redis")
		err := pgOp.Init(properties)
		if err != nil {
			return errors.Errorf(initErrFmt, "postgres", err)
		}
	}

	if etcdOp, err := etcd.NewEtcd(); err != nil {
		return errors.Errorf(createErrFmt, "etcd", err)
	} else {
		builtinMap["etcd"] = etcdOp
		properties := component.GetProperties("etcd")
		err := etcdOp.Init(properties)
		if err != nil {
			return errors.Errorf(initErrFmt, "etcd", err)
		}
	}

	if mongoOp, err := mongodb.NewMongoDB(); err != nil {
		return errors.Errorf(createErrFmt, "mongodb", err)
	} else {
		builtinMap["mongodb"] = mongoOp
		properties := component.GetProperties("mongodb")
		err := mongoOp.Init(properties)
		if err != nil {
			return errors.Errorf(initErrFmt, "mongodb", err)
		}
	}

	// custom 感觉不一定需要init
	customOp, _ = custom.NewHTTPCustom()

	return nil
}

func GetRouter() func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		// get type
		character := getCharacter(request.URL.Path)
		if character == "" {
			Logger.Error(nil, "character type missing in path")
			return
		}
		// read body
		body := request.Body
		defer body.Close()
		buf := make([]byte, 500)
		n, err := body.Read(buf)
		if err != nil {
			Logger.Error(err, "request body read failed")
			return
		}
		buf = buf[:n]
		request.Context()
		// parse
		meta := &RequestMeta{Metadata: map[string]string{}}
		err = json.Unmarshal(buf, meta)
		if err != nil {
			Logger.Error(err, "request body unmarshal failed")
			return
		}
		// 赋值
		probeRequest := &ProbeRequest{Metadata: meta.Metadata}
		probeRequest.Operation = util.OperationKind(meta.Operation)
		// 派发
		probeResp, err := route(character, request.Context(), probeRequest)
		// 响应
		if err != nil {
			Logger.Error(err, "exec ops failed")
			msg := fmt.Sprintf("exec ops failed: %v", err)
			writer.Header().Add(statusCodeHeader, OperationFailedHTTPCode)
			_, err := writer.Write([]byte(msg))
			if err != nil {
				Logger.Error(err, "ResponseWriter writes error when router")
			}
		} else {
			code, ok := probeResp.Metadata[StatusCode]
			if ok {
				writer.Header().Add(statusCodeHeader, code)
			}
			writer.Header().Add(RespDurationKey, probeResp.Metadata[RespDurationKey])
			writer.Header().Add(RespEndTimeKey, probeResp.Metadata[RespEndTimeKey])
			_, err := writer.Write(probeResp.Data)
			if err != nil {
				Logger.Error(err, "ResponseWriter writes error when router")
			}
		}
	}
}

func getCharacter(url string) string {
	if !strings.HasPrefix(url, bindingPath) {
		return ""
	}
	splits := strings.Split(url, "/")
	if len(splits) != 4 {
		return ""
	}
	return splits[3]
}

func route(character string, ctx context.Context, request *ProbeRequest) (*ProbeResponse, error) {
	ops, ok := builtinMap[character]
	// 如果不是builtin那就用custom
	if !ok {
		// TODO: impl the custom
		return nil, nil
	}
	return ops.Dispatch(ctx, request)
}
