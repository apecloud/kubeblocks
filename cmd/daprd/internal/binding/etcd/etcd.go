/*
Copyright 2021 The Dapr Authors
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
	"errors"
	"time"

	"github.com/dapr/kit/logger"

	"github.com/dapr/components-contrib/bindings"
	v3 "go.etcd.io/etcd/client/v3"
)

const (
	queryOperation bindings.OperationKind = "query"

	endpoint = "endpoint"
)

type Binding struct {
	etcd     *v3.Client
	endpoint string
	logger   logger.Logger
}

// NewEtcd returns a new etcd binding instance.
func NewEtcd(logger logger.Logger) bindings.OutputBinding {
	return &Binding{logger: logger}
}

func (b *Binding) Init(metadata bindings.Metadata) error {

	ep := metadata.Properties[endpoint]

	cli, err := v3.New(v3.Config{
		Endpoints:   []string{ep},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}

	b.etcd = cli
	b.endpoint = ep

	return nil
}

func (b *Binding) Operations() []bindings.OperationKind {
	return []bindings.OperationKind{queryOperation}
}

func (b *Binding) Close() (err error) {
	return b.etcd.Close()
}

func (b *Binding) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	resp, err := b.etcd.Status(ctx, b.endpoint)
	if err != nil {
		return nil, err
	}

	ir := new(bindings.InvokeResponse)
	ir.Metadata = req.Metadata
	result := "follower"
	switch {
	case resp.Leader == resp.Header.MemberId:
		result = "leader"
	case resp.IsLearner:
		result = "learner"
	}
	ir.Data = []byte(result)

	return ir, nil
}

func (b *Binding) Read(ctx context.Context, handler bindings.Handler) error {
	b.logger.Warnf("read not defined")
	return errors.New("read not defined")

}
