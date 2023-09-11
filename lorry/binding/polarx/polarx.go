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

package polarx

import (
	"context"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"

	. "github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/component/polarx"
	. "github.com/apecloud/kubeblocks/lorry/util"
)

// PolarxOperations represents PolarX output bindings.
type PolarxOperations struct {
	BaseOperations
}

type QueryRes []map[string]interface{}

// NewPolarx returns a new PolarX output binding.
func NewPolarx(logger logger.Logger) bindings.OutputBinding {
	return &PolarxOperations{BaseOperations: BaseOperations{Logger: logger}}
}

// Init initializes the PolarX binding.
func (polarxOps *PolarxOperations) Init(metadata bindings.Metadata) error {
	polarxOps.Logger.Debug("Initializing PolarX binding")
	polarxOps.BaseOperations.Init(metadata)
	config, err := polarx.NewConfig(metadata.Properties)
	if err != nil {
		polarxOps.Logger.Errorf("PolarX config initialize failed: %v", err)
		return err
	}

	var manager component.DBManager

	manager, err = polarx.NewManager(polarxOps.Logger)
	if err != nil {
		polarxOps.Logger.Errorf("PolarX DB Manager initialize failed: %v", err)
		return err
	}

	polarxOps.Manager = manager
	polarxOps.DBType = "polarx"
	// polarxOps.InitIfNeed = polarxOps.initIfNeed
	polarxOps.BaseOperations.GetRole = polarxOps.GetRole
	polarxOps.DBPort = config.GetDBPort()

	polarxOps.RegisterOperationOnDBReady(GetRoleOperation, polarxOps.GetRoleOps, manager)
	polarxOps.RegisterOperationOnDBReady(ExecOperation, polarxOps.ExecOps, manager)
	polarxOps.RegisterOperationOnDBReady(QueryOperation, polarxOps.QueryOps, manager)

	return nil
}

func (polarxOps *PolarxOperations) GetRole(ctx context.Context, request *bindings.InvokeRequest, response *bindings.InvokeResponse) (string, error) {
	manager := polarxOps.Manager.(*polarx.Manager)
	return manager.GetRole(ctx)
}

func (polarxOps *PolarxOperations) GetRunningPort() int {
	return 0
}

func (polarxOps *PolarxOperations) ExecOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = "ExecFailed"
		result["message"] = ErrNoSQL
		return result, nil
	}

	manager, _ := polarxOps.Manager.(*polarx.Manager)
	count, err := manager.Exec(ctx, sql)
	if err != nil {
		polarxOps.Logger.Infof("exec error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["count"] = count
	}
	return result, nil
}

func (polarxOps *PolarxOperations) QueryOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	result := OpsResult{}
	sql, ok := req.Metadata["sql"]
	if !ok || sql == "" {
		result["event"] = OperationFailed
		result["message"] = "no sql provided"
		return result, nil
	}
	manager, _ := polarxOps.Manager.(*polarx.Manager)
	data, err := manager.Query(ctx, sql)
	if err != nil {
		polarxOps.Logger.Infof("Query error: %v", err)
		result["event"] = OperationFailed
		result["message"] = err.Error()
	} else {
		result["event"] = OperationSuccess
		result["message"] = string(data)
	}
	return result, nil
}
