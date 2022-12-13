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
	"fmt"
	"net"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	RunningCheckOperation bindings.OperationKind = "runningCheck"
	StatusCheckOperation  bindings.OperationKind = "statusCheck"
	RoleCheckOperation    bindings.OperationKind = "roleCheck"

	// CommandSQLKey keys from request's metadata.
	CommandSQLKey = "sql"

	defaultEventAggregationNum = 10
	defaultEventIntervalNum    = 60
)

type ProbeBase struct {
	Operation               ProbeOperation
	Logger                  logger.Logger
	oriRole                 string
	runningCheckFailedCount int
	roleCheckFailedCount    int
	roleCheckCount          int
	eventAggregationNum     int
	eventIntervalNum        int
	dbPort                  int
}

// ProbeOperation abstracts the interfaces a binding implementation needs to support.
// these interfaces together providing probing service: runningCheck, statusCheck, roleCheck
type ProbeOperation interface {
	// InitIfNeed binding initiates
	InitIfNeed() error
	// GetRunningPort get binding service port.
	// runningCheck will run its check on this port
	GetRunningPort() int
	// StatusCheck TODO proposal TBD
	StatusCheck(context.Context, string, *bindings.InvokeResponse) ([]byte, error)
	// GetRole get consensus role name of the binding service
	// roleCheck will call this interface when necessary
	// return role name of the binding service
	GetRole(context.Context, string) (string, error)
}

func (p *ProbeBase) Init() {
	viper.SetDefault("KB_AGGREGATION_NUMBER", defaultEventAggregationNum)
	p.eventAggregationNum = viper.GetInt("KB_AGGREGATION_NUMBER")
	viper.SetDefault("KB_EVENT_INTERNAL_NUMBER", defaultEventIntervalNum)
	p.eventIntervalNum = viper.GetInt("KB_EVENT_INTERNAL_NUMBER")
	viper.SetDefault("KB_SERVICE_PORT", p.Operation.GetRunningPort())
	p.dbPort = viper.GetInt("KB_SERVICE_PORT")
}

func (p *ProbeBase) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{
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
		d, err := p.runningCheck(ctx, resp)
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
			req.Operation, RunningCheckOperation, StatusCheckOperation, RoleCheckOperation)
	}

	return updateRespMetadata()
}

func (p *ProbeBase) roleObserve(ctx context.Context, cmd string, response *bindings.InvokeResponse) ([]byte, error) {
	result := &ProbeMessage{}
	result.OriginalRole = p.oriRole
	role, err := p.Operation.GetRole(ctx, cmd)
	if err != nil {
		p.Logger.Infof("error executing roleCheck: %v", err)
		result.Event = "roleCheckFailed"
		result.Message = err.Error()
		if p.roleCheckFailedCount++; p.roleCheckFailedCount%p.eventAggregationNum == 1 {
			p.Logger.Infof("role checks failed %v times continuously", p.roleCheckFailedCount)
			response.Metadata[StatusCode] = CheckFailedHTTPCode
		}
		msg, _ := json.Marshal(result)
		return msg, nil
	}

	result.Role = role
	if p.oriRole != role {
		result.Event = "roleChanged"
		p.oriRole = role
		p.roleCheckCount = 0
	} else {
		result.Event = "roleUnchanged"
	}

	// reporting role event periodically to get pod's role label updating accurately
	// in case of event losing.
	if p.roleCheckCount++; p.roleCheckCount%p.eventIntervalNum == 1 {
		response.Metadata[StatusCode] = CheckFailedHTTPCode
	}
	msg, _ := json.Marshal(result)
	p.Logger.Infof(string(msg))
	return msg, nil
}

// runningCheck checks whether the binding service is in running status:
// the port is open or is close consecutively in eventAggregationNum times
func (p *ProbeBase) runningCheck(ctx context.Context, resp *bindings.InvokeResponse) ([]byte, error) {
	var message string
	result := ProbeMessage{}
	marshalResult := func() ([]byte, error) {
		result.Message = message
		return json.Marshal(result)
	}

	host := fmt.Sprintf("127.0.0.1:%d", p.dbPort)
	conn, err := net.DialTimeout("tcp", host, 900*time.Millisecond)
	if err != nil {
		message = fmt.Sprintf("running check %s error: %v", host, err)
		result.Event = "runningCheckFailed"
		p.Logger.Errorf(message)
		if p.runningCheckFailedCount++; p.runningCheckFailedCount%p.eventAggregationNum == 1 {
			p.Logger.Infof("running checks failed %v times continuously", p.runningCheckFailedCount)
			resp.Metadata[StatusCode] = CheckFailedHTTPCode
		}
		return marshalResult()
	}
	defer conn.Close()
	p.runningCheckFailedCount = 0
	message = "TCP Connection Established Successfully!"
	if tcpCon, ok := conn.(*net.TCPConn); ok {
		tcpCon.SetLinger(0)
	}
	return marshalResult()
}
