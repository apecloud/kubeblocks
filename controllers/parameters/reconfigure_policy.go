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
)

// ExecStatus defines running result for Reconfiguring policy (fsm).
// ESNone describes policy has finished and quit.
// ESRetry describes fsm is running.
// ESFailed describes fsm is failed and exited.
// ESNotSupport describes fsm does not support the feature.
// ESFailedAndRetry describes fsm is failed in current state, but can be retried.
// +enum
type ExecStatus string

const (
	ESNone           ExecStatus = "None"
	ESRetry          ExecStatus = "Retry"
	ESFailed         ExecStatus = "Failed"
	ESNotSupport     ExecStatus = "NotSupport"
	ESFailedAndRetry ExecStatus = "FailedAndRetry"
)

type returnedStatus struct {
	Status        ExecStatus
	SucceedCount  int32
	ExpectedCount int32
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

type reconfigurePolicy interface {
	// Upgrade is to enable the configuration to take effect.
	Upgrade(rctx reconfigureContext) (returnedStatus, error)
}

type ReloadAction interface {
	ExecReload() (returnedStatus, error)
	ReloadType() string
}

var (
	reconfigurePolicyMap = map[parametersv1alpha1.ReloadPolicy]reconfigurePolicy{}
)

type reconfigureTask struct {
	parametersv1alpha1.ReloadPolicy
	taskCtx reconfigureContext
}

func registerPolicy(policy parametersv1alpha1.ReloadPolicy, action reconfigurePolicy) {
	reconfigurePolicyMap[policy] = action
}

func (param *reconfigureContext) getTargetConfigHash() *string {
	return param.ConfigHash
}

func (param *reconfigureContext) getTargetReplicas() int {
	return int(param.ClusterComponent.Replicas)
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

func withSucceed(succeedCount int32) func(status *returnedStatus) {
	return func(status *returnedStatus) {
		status.SucceedCount = succeedCount
	}
}

func withExpected(expectedCount int32) func(status *returnedStatus) {
	return func(status *returnedStatus) {
		status.ExpectedCount = expectedCount
	}
}

func makeReturnedStatus(status ExecStatus, ops ...func(status *returnedStatus)) returnedStatus {
	ret := returnedStatus{
		Status:        status,
		SucceedCount:  core.Unconfirmed,
		ExpectedCount: core.Unconfirmed,
	}
	for _, o := range ops {
		o(&ret)
	}
	return ret
}

func (r reconfigureTask) ReloadType() string {
	return string(r.ReloadPolicy)
}

func (r reconfigureTask) ExecReload() (returnedStatus, error) {
	if executor, ok := reconfigurePolicyMap[r.ReloadPolicy]; ok {
		return executor.Upgrade(r.taskCtx)
	}

	return returnedStatus{}, fmt.Errorf("not support reload action[%s]", r.ReloadPolicy)
}
