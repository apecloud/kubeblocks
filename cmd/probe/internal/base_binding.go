/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"context"
	"encoding/json"
	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	"strconv"
	"time"
)

const (
	ExecOperation         bindings.OperationKind = "exec"
	RunningCheckOperation bindings.OperationKind = "runningCheck"
	StatusCheckOperation  bindings.OperationKind = "statusCheck"
	RoleCheckOperation    bindings.OperationKind = "roleCheck"
	QueryOperation        bindings.OperationKind = "query"
	CloseOperation        bindings.OperationKind = "close"

	// CommandSQLKey keys from request's metadata.
	CommandSQLKey = "sql"
)

var (
	oriRole              = ""
	roleCheckFailedCount = 0
	roleCheckCount       = 0
	eventAggregationNum  = 10
	eventIntervalNum     = 60
)

type ProbeBase struct {
	Operation ProbeOperation
	Logger    logger.Logger
}

type ProbeOperation interface {
	InitIfNeed() error
	Exec(context.Context, string) (int64, error)
	RunningCheck(context.Context, *bindings.InvokeResponse) ([]byte, error)
	StatusCheck(context.Context, string, *bindings.InvokeResponse) ([]byte, error)
	GetRole(context.Context, string) (string, error)
	Query(context.Context, string) ([]byte, error)
	Close() error
}

func (p *ProbeBase) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{
		ExecOperation,
		QueryOperation,
		CloseOperation,
		RunningCheckOperation,
		StatusCheckOperation,
		RoleCheckOperation,
	}
}

func (p *ProbeBase) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	if req == nil {
		return nil, errors.Errorf("invoke request required")
	}

	var sql string
	var ok bool
	startTime := time.Now()
	resp := &bindings.InvokeResponse{
		Metadata: map[string]string{
			RespOpKey:        string(req.Operation),
			RespSQLKey:       "test",
			RespStartTimeKey: startTime.Format(time.RFC3339Nano),
		},
	}

	updateRespMetadata := func() (*bindings.InvokeResponse, error) {
		endTime := time.Now()
		resp.Metadata[RespEndTimeKey] = endTime.Format(time.RFC3339Nano)
		resp.Metadata[RespDurationKey] = endTime.Sub(startTime).String()
		return resp, nil
	}

	if req.Operation == RunningCheckOperation {
		d, err := p.Operation.RunningCheck(ctx, resp)
		if err != nil {
			return nil, err
		}
		resp.Data = d
		return updateRespMetadata()
	}

	if err := p.Operation.InitIfNeed(); err != nil {
		resp.Data = []byte("db not ready")
		return updateRespMetadata()
	}

	if req.Operation == CloseOperation {
		return nil, p.Operation.Close()
	}

	if req.Metadata == nil {
		return nil, errors.Errorf("metadata required")
	}
	p.Logger.Debugf("operation: %v", req.Operation)

	sql, ok = req.Metadata[CommandSQLKey]
	if !ok {
		//return nil, errors.Errorf("required metadata not set: %s", CommandSQLKey)
		p.Logger.Errorf("required metadata not set: %s", CommandSQLKey)
	}

	switch req.Operation { //nolint:exhaustive
	case ExecOperation:
		r, err := p.Operation.Exec(ctx, sql)
		if err != nil {
			return nil, err
		}
		resp.Metadata[RespRowsAffectedKey] = strconv.FormatInt(r, 10)

	case QueryOperation:
		d, err := p.Operation.Query(ctx, sql)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	case StatusCheckOperation:
		d, err := p.Operation.StatusCheck(ctx, sql, resp)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	case RoleCheckOperation:
		d, err := p.roleObserve(ctx, sql, resp)
		if err != nil {
			return nil, err
		}
		resp.Data = d

	default:
		return nil, errors.Errorf("invalid operation type: %s. Expected %s, %s, or %s",
			req.Operation, ExecOperation, QueryOperation, CloseOperation)
	}

	return updateRespMetadata()
}

func (p *ProbeBase) roleObserve(ctx context.Context, cmd string, response *bindings.InvokeResponse) ([]byte, error) {
	result := &ProbeMessage{}
	result.OriginalRole = oriRole
	role, err := p.Operation.GetRole(ctx, cmd)
	if err != nil {
		p.Logger.Infof("error executing roleCheck: %v", err)
		result.Event = "roleCheckFailed"
		result.Message = err.Error()
		if roleCheckFailedCount++; roleCheckFailedCount%eventAggregationNum == 1 {
			p.Logger.Infof("role checks failed %v times continuously", roleCheckFailedCount)
			response.Metadata[StatusCode] = CheckFailedHTTPCode
		}
		msg, _ := json.Marshal(result)
		return msg, nil
	}

	result.Role = role
	if oriRole != role {
		result.Event = "roleChanged"
		oriRole = role
		roleCheckCount = 0
	} else {
		result.Event = "roleUnchanged"
	}

	// reporting role event periodically to get pod's role label updating accurately
	// in case of event losing.
	if roleCheckCount++; roleCheckCount%eventIntervalNum == 1 {
		response.Metadata[StatusCode] = CheckFailedHTTPCode
	}
	msg, _ := json.Marshal(result)
	p.Logger.Infof(string(msg))
	return msg, nil
}
