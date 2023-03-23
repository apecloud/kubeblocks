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

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

type Etcd struct {
	lock     sync.Mutex
	etcd     *v3.Client
	endpoint string
	BaseOperations
}

const (
	endpoint = "endpoint"

	defaultPort        = 2379
	defaultDialTimeout = 400 * time.Millisecond
)

// NewEtcd returns a new etcd binding instance.
func NewEtcd(logger logger.Logger) bindings.OutputBinding {
	return &Etcd{BaseOperations: BaseOperations{Logger: logger}}
}

func (e *Etcd) Init(metadata bindings.Metadata) error {
	e.endpoint = metadata.Properties[endpoint]
	e.BaseOperations.Init(metadata)
	e.DBType = "etcd"
	e.InitIfNeed = e.initIfNeed
	e.DBPort = e.GetRunningPort()
	e.OperationMap[GetRoleOperation] = e.GetRoleOps
	return nil
}

func (e *Etcd) initIfNeed() bool {
	if e.etcd == nil {
		go func() {
			err := e.InitDelay()
			e.Logger.Errorf("MongoDB connection init failed: %v", err)
		}()
		return true
	}
	return false
}

func (e *Etcd) InitDelay() error {
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

func (e *Etcd) GetRole(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (string, error) {
	etcdResp, err := e.etcd.Status(ctx, e.endpoint)
	if err != nil {
		return "", err
	}

	role := "follower"
	switch {
	case etcdResp.Leader == etcdResp.Header.MemberId:
		role = "leader"
	case etcdResp.IsLearner:
		role = "learner"
	}

	return role, nil
}

func (e *Etcd) GetRoleOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	role, err := e.GetRole(ctx, req, resp)
	if err != nil {
		return nil, err
	}
	opsRes := OpsResult{}
	opsRes["role"] = role
	return opsRes, nil
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
