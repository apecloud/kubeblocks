/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	. "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
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
			e.Logger.Errorf("Etcd connection init failed: %v", err)
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
	return nil, nil
}
