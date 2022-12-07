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
	"sync"
	"time"

	"github.com/apecloud/kubeblocks/cmd/probe/internal"
	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	v3 "go.etcd.io/etcd/client/v3"
)

const (
	endpoint = "endpoint"
)

type Etcd struct {
	lock     sync.Mutex
	etcd     *v3.Client
	endpoint string
	logger   logger.Logger
	base     internal.ProbeBase
}

// NewEtcd returns a new etcd binding instance.
func NewEtcd(logger logger.Logger) bindings.OutputBinding {
	return &Etcd{logger: logger}
}

func (e *Etcd) Init(metadata bindings.Metadata) error {
	e.endpoint = metadata.Properties[endpoint]
	e.base = internal.ProbeBase{
		Logger:    e.logger,
		Operation: e,
	}

	return nil
}

func (e *Etcd) Operations() []bindings.OperationKind {
	return e.base.Operations()
}

func (e *Etcd) Close() (err error) {
	return e.etcd.Close()
}

func (e *Etcd) Invoke(ctx context.Context, req *bindings.InvokeRequest) (*bindings.InvokeResponse, error) {
	return e.base.Invoke(ctx, req)
}

func (e *Etcd) InitIfNeed() error {
	e.lock.Lock()
	defer e.lock.Unlock()

	if e.etcd != nil {
		return nil
	}

	cli, err := v3.New(v3.Config{
		Endpoints:   []string{e.endpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}

	_, err = cli.Status(context.Background(), e.endpoint)
	if err != nil {
		return err
	}

	e.etcd = cli

	return nil
}

func (e *Etcd) Exec(ctx context.Context, cmd string) (int64, error) {
	//TODO implement me
	return 0, nil
}

func (e *Etcd) RunningCheck(ctx context.Context, response *bindings.InvokeResponse) ([]byte, error) {
	//TODO implement me
	return nil, nil
}

func (e *Etcd) StatusCheck(ctx context.Context, cmd string, response *bindings.InvokeResponse) ([]byte, error) {
	//TODO implement me
	return nil, nil
}

func (e *Etcd) GetRole(ctx context.Context, cmd string) (string, error) {
	resp, err := e.etcd.Status(ctx, e.endpoint)
	if err != nil {
		return "", err
	}

	role := "follower"
	switch {
	case resp.Leader == resp.Header.MemberId:
		role = "leader"
	case resp.IsLearner:
		role = "learner"
	}

	return role, nil
}

func (e *Etcd) Query(ctx context.Context, cmd string) ([]byte, error) {
	//TODO implement me
	return nil, nil
}
