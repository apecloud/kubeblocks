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
	"time"

	v3 "go.etcd.io/etcd/client/v3"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

const (
	endpoint = "endpoint"

	defaultPort        = 2379
	defaultDialTimeout = 600 * time.Millisecond
)

type Manager struct {
	engines.DBManagerBase
	etcd     *v3.Client
	endpoint string
}

var _ engines.DBManager = &Manager{}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("ETCD")

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		DBManagerBase: *managerBase,
	}

	var endpoints []string
	endpoint, ok := properties[endpoint]
	if ok {
		mgr.endpoint = endpoint
		endpoints = []string{endpoint}
	}

	cli, err := v3.New(v3.Config{
		Endpoints:   endpoints,
		DialTimeout: defaultDialTimeout,
	})
	if err != nil {
		return nil, err
	}

	mgr.etcd = cli
	return mgr, nil
}

func (mgr *Manager) IsDBStartupReady() bool {
	if mgr.DBStartupReady {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	status, err := mgr.etcd.Status(ctx, mgr.endpoint)
	cancel()
	if err != nil {
		mgr.Logger.Info("get etcd status failed", "error", err, "status", status)
		return false
	}

	mgr.DBStartupReady = true
	mgr.Logger.Info("DB startup ready")
	return true
}

func (mgr *Manager) GetRunningPort() int {
	index := strings.Index(mgr.endpoint, ":")
	if index < 0 {
		return defaultPort
	}
	port, err := strconv.Atoi(mgr.endpoint[index+1:])
	if err != nil {
		return defaultPort
	}

	return port
}
