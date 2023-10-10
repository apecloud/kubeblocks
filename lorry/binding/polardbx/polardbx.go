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

package polardbx

import (
	"context"

	"github.com/go-logr/logr"

	. "github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/component/polardbx"
	"github.com/apecloud/kubeblocks/lorry/util"
)

// PolardbxOperations represents polardbx output bindings.
type PolardbxOperations struct {
	BaseOperations
}

type QueryRes []map[string]interface{}

// NewPolardbx returns a new polardbx output binding.
func NewPolardbx(logger logr.Logger) *PolardbxOperations {
	return &PolardbxOperations{BaseOperations: BaseOperations{Logger: logger}}
}

// Init initializes the polardbx binding.
func (polardbxOps *PolardbxOperations) Init(metadata component.Properties) error {
	polardbxOps.Logger.Info("Initializing polardbx binding")
	polardbxOps.BaseOperations.Init(metadata)
	config, err := polardbx.NewConfig(metadata)
	if err != nil {
		polardbxOps.Logger.Error(err, "polardbx config initialize failed")
		return err
	}

	var manager component.DBManager

	manager, err = polardbx.NewManager(polardbxOps.Logger)
	if err != nil {
		polardbxOps.Logger.Error(err, "polardbx DB Manager initialize failed")
		return err
	}

	polardbxOps.Manager = manager
	polardbxOps.DBType = "polardbx"
	// polardbxOps.InitIfNeed = polardbxOps.initIfNeed
	polardbxOps.BaseOperations.GetRole = polardbxOps.GetRole
	polardbxOps.DBPort = config.GetDBPort()

	polardbxOps.RegisterOperationOnDBReady(util.GetRoleOperation, polardbxOps.GetRoleOps, manager)
	polardbxOps.RegisterOperationOnDBReady(util.ExecOperation, polardbxOps.ExecOps, manager)
	polardbxOps.RegisterOperationOnDBReady(util.QueryOperation, polardbxOps.QueryOps, manager)

	return nil
}

func (polardbxOps *PolardbxOperations) GetRole(ctx context.Context, request *ProbeRequest, response *ProbeResponse) (string, error) {
	manager := polardbxOps.Manager.(*polardbx.Manager)
	return manager.GetRole(ctx)
}

func (polardbxOps *PolardbxOperations) GetRunningPort() int {
	return 0
}

func (polardbxOps *PolardbxOperations) ExecOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = "ExecFailed"
		result["message"] = ErrNoSQL
		return result, nil
	}

	manager, _ := polardbxOps.Manager.(*polardbx.Manager)
	count, err := manager.Exec(ctx, sql)
	if err != nil {
		polardbxOps.Logger.Error(err, "exec error", "sql", sql)
		result["event"] = util.OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = util.OperationSuccess
		result["count"] = count
	}
	return result, nil
}

func (polardbxOps *PolardbxOperations) QueryOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = util.OperationFailed
		result["message"] = "no sql provided"
		return result, nil
	}
	manager, _ := polardbxOps.Manager.(*polardbx.Manager)
	data, err := manager.Query(ctx, sql)
	if err != nil {
		polardbxOps.Logger.Error(err, "Query error", "sql", sql)
		result["event"] = util.OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = util.OperationSuccess
		result["message"] = string(data)
	}
	return result, nil
}
