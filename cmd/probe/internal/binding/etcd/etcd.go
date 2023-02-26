/*
Copyright ApeCloud, Inc.

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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	v3 "go.etcd.io/etcd/client/v3"

	"github.com/apecloud/kubeblocks/cmd/probe/internal"
)

type Etcd struct {
	lock     sync.Mutex
	etcd     *v3.Client
	endpoint string
	logger   logger.Logger
	base     internal.ProbeBase
}

var _ internal.ProbeOperation = &Etcd{}

const (
	endpoint = "endpoint"

	defaultPort        = 2379
	defaultDialTimeout = 400 * time.Millisecond
)

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
	e.base.Init()
	return nil
}

func (e *Etcd) Operations() []bindings.OperationKind {
	return e.base.Operations()
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
		DialTimeout: defaultDialTimeout,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	_, err = cli.Status(ctx, e.endpoint)
	cancel()
	if err != nil {
		cli.Close()
		return err
	}

	e.etcd = cli

	return nil
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

func (e *Etcd) GetRunningPort() int {
	index := strings.Index(e.endpoint, ":")
	if index < 0 {
		return defaultPort
	}
	port, err := strconv.Atoi(e.endpoint[index+1:])
	if err != nil {
		return defaultPort
	}

	return port
}

func (e *Etcd) StatusCheck(ctx context.Context, cmd string, response *bindings.InvokeResponse) ([]byte, error) {
	// TODO implement me when proposal is passed
	// proposal: https://infracreate.feishu.cn/wiki/wikcndch7lMZJneMnRqaTvhQpwb#doxcnOUyQ4Mu0KiUo232dOr5aad
	return nil, nil
}
