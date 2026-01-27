/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
)

const (
	reconfigureStatusNone           string = "None"           // finished and quit
	reconfigureStatusRetry          string = "Retry"          // running
	reconfigureStatusFailed         string = "Failed"         // failed and exited
	reconfigureStatusFailedAndRetry string = "FailedAndRetry" // failed but can be retried
)

type reconfigureStatus struct {
	status        string
	expectedCount int32
	succeedCount  int32
}

func makeReconfigureStatus(status string, ops ...func(status *reconfigureStatus)) reconfigureStatus {
	ret := reconfigureStatus{
		status:        status,
		expectedCount: core.Unconfirmed,
		succeedCount:  core.Unconfirmed,
	}
	for _, o := range ops {
		o(&ret)
	}
	return ret
}

type reconfigureContext struct {
	intctrlutil.RequestCtx
	Client client.Client

	ConfigTemplate appsv1.ComponentFileTemplate
	ConfigHash     *string // the hash of the new configuration content

	Cluster              *appsv1.Cluster
	ClusterComponent     *appsv1.ClusterComponentSpec
	SynthesizedComponent *component.SynthesizedComponent
	its                  *workloads.InstanceSet // TODO: use cluster or component API?

	ConfigDescription *parametersv1alpha1.ComponentConfigDescription
	ParametersDef     *parametersv1alpha1.ParametersDefinitionSpec
	Patch             *core.ConfigPatchInfo
}

func (param *reconfigureContext) getTargetConfigHash() *string {
	return param.ConfigHash
}

func (param *reconfigureContext) getTargetReplicas() int {
	return int(param.ClusterComponent.Replicas)
}

type reconfigurePolicy interface {
	Upgrade(rctx reconfigureContext) (reconfigureStatus, error)
}

var (
	reconfigurePolicyMap = map[parametersv1alpha1.ReloadPolicy]reconfigurePolicy{}
)

func registerPolicy(policy parametersv1alpha1.ReloadPolicy, action reconfigurePolicy) {
	reconfigurePolicyMap[policy] = action
}

func enableSyncTrigger(reloadAction *parametersv1alpha1.ReloadAction) bool {
	if reloadAction == nil {
		return false
	}
	if reloadAction.ShellTrigger != nil {
		return !core.IsWatchModuleForShellTrigger(reloadAction.ShellTrigger)
	}
	return false
}

func computeTargetConfigHash(reqCtx *intctrlutil.RequestCtx, data map[string]string) *string {
	hash, err := cfgutil.ComputeHash(data)
	if err != nil {
		if reqCtx != nil {
			reqCtx.Log.Error(err, "failed to get configuration version!")
		}
		return nil
	}
	return &hash
}

func withSucceed(succeedCount int32) func(status *reconfigureStatus) {
	return func(status *reconfigureStatus) {
		status.succeedCount = succeedCount
	}
}

func withExpected(expectedCount int32) func(status *reconfigureStatus) {
	return func(status *reconfigureStatus) {
		status.expectedCount = expectedCount
	}
}

type reconfigureTask struct {
	policy  parametersv1alpha1.ReloadPolicy
	taskCtx reconfigureContext
}

func (r reconfigureTask) reconfigure() (reconfigureStatus, error) {
	if executor, ok := reconfigurePolicyMap[r.policy]; ok {
		return executor.Upgrade(r.taskCtx)
	}
	return reconfigureStatus{}, fmt.Errorf("not support reload action[%s]", r.policy)
}
