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

package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dapr/kit/logger"

	"github.com/dapr/components-contrib/bindings"
	v3 "go.etcd.io/etcd/client/v3"
)

const (
	queryOperation bindings.OperationKind = "query"

	endpoint = "endpoint"

	// keys from response's metadata.
	respOpKey        = "operation"
	respSQLKey       = "sql"
	respStartTimeKey = "start-time"
	respEndTimeKey   = "end-time"
	respDurationKey  = "duration"
)

var oriRole = ""

type Binding struct {
	etcd     *v3.Client
	endpoint string
	logger   logger.Logger
}

// NewEtcd returns a new etcd binding instance.
func NewEtcd(logger logger.Logger) bindings.OutputBinding {
	return &Binding{logger: logger}
}

func (b *Binding) InitDelay() error {
	cli, err := v3.New(v3.Config{
		Endpoints:   []string{b.endpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}

	_, err = cli.Status(context.Background(), b.endpoint)
	if err != nil {
		return err
	}

	b.etcd = cli

	return nil
}

func (b *Binding) Init(metadata bindings.Metadata) error {
	b.endpoint = metadata.Properties[endpoint]

	return nil
}

func (b *Binding) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{queryOperation}
}

func (b *Binding) Close() (err error) {
	return b.etcd.Close()
}

func (b *Binding) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	startTime := time.Now()
	resp := &bindings.InvokeResponse{
		Metadata: map[string]string{
			respOpKey:        string(req.Operation),
			respSQLKey:       "test",
			respStartTimeKey: startTime.Format(time.RFC3339Nano),
		},
	}

	if req == nil {
		return nil, errors.New("invoke request required")
	}

	if b.etcd == nil {
		go b.InitDelay()
		resp.Data = []byte("db not ready")
		return resp, nil
	}

	data, err := b.roleCheck(ctx)
	if err != nil {
		return nil, err
	}
	resp.Data = data

	endTime := time.Now()
	resp.Metadata[respEndTimeKey] = endTime.Format(time.RFC3339Nano)
	resp.Metadata[respDurationKey] = endTime.Sub(startTime).String()

	return resp, nil
}

func (b *Binding) roleCheck(ctx context.Context) ([]byte, error) {
	resp, err := b.etcd.Status(ctx, b.endpoint)
	if err != nil {
		return nil, err
	}

	role := "follower"
	switch {
	case resp.Leader == resp.Header.MemberId:
		role = "leader"
	case resp.IsLearner:
		role = "learner"
	}

	if oriRole != role {
		result := map[string]string{}
		result["event"] = "roleChanged"
		result["originalRole"] = oriRole
		result["role"] = role
		msg, _ := json.Marshal(result)
		b.logger.Infof(string(msg))
		oriRole = role
		return nil, errors.New(string(msg))
	}

	return []byte(oriRole), nil
}
