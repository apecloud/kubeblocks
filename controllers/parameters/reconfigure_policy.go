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
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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

type ReturnedStatus struct {
	Status        ExecStatus
	SucceedCount  int32
	ExpectedCount int32
}

type reconfigurePolicy interface {
	// Upgrade is to enable the configuration to take effect.
	Upgrade(rctx reconfigureContext) (ReturnedStatus, error)

	// GetPolicyName returns name of policy.
	GetPolicyName() string
}

type AutoReloadPolicy struct{}

type reconfigureContext struct {
	intctrlutil.RequestCtx
	Client client.Client

	Cluster        *appsv1.Cluster
	ConfigTemplate appsv1.ComponentFileTemplate

	// Associated component for cluster.
	ClusterComponent *appsv1.ClusterComponentSpec

	// Associated component for component and component definition.
	SynthesizedComponent *component.SynthesizedComponent

	// List of InstanceSet using this config template.
	InstanceSetUnits []workloads.InstanceSet

	// Configmap object of the configuration template instance in the component.
	ConfigMap *corev1.ConfigMap

	// ConfigConstraint pointer
	// ConfigConstraint *appsv1beta1.ConfigConstraintSpec

	// For grpc factory
	ReconfigureClientFactory createReconfigureClient

	// List of containers using this config volume.
	ContainerNames []string

	ConfigDescription *parametersv1alpha1.ComponentConfigDescription
	ParametersDef     *parametersv1alpha1.ParametersDefinitionSpec
	Patch             *core.ConfigPatchInfo
}

var (
	// lazy creation of grpc connection
	// TODO support connection pool
	newGRPCClient = func(addr string) (cfgproto.ReconfigureClient, error) {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		return cfgproto.NewReconfigureClient(conn), nil
	}
)

var upgradePolicyMap = map[parametersv1alpha1.ReloadPolicy]reconfigurePolicy{}

func init() {
	registerPolicy(parametersv1alpha1.AsyncDynamicReloadPolicy, &AutoReloadPolicy{})
}

// GetClientFactory support ut mock
func GetClientFactory() createReconfigureClient {
	return newGRPCClient
}

func (param *reconfigureContext) generateConfigIdentifier() string {
	key := param.ConfigTemplate.Name
	if param.ConfigDescription != nil && param.ConfigDescription.Name != "" {
		hash, _ := util.ComputeHash(param.ConfigDescription.Name)
		key = key + "-" + hash
	}
	return strings.ReplaceAll(key, "_", "-")
}

func (param *reconfigureContext) getTargetVersionHash() string {
	hash, err := util.ComputeHash(param.ConfigMap.Data)
	if err != nil {
		param.Log.Error(err, "failed to get configuration version!")
		return ""
	}

	return hash
}

func (param *reconfigureContext) getTargetReplicas() int {
	return int(param.ClusterComponent.Replicas)
}

func registerPolicy(policy parametersv1alpha1.ReloadPolicy, action reconfigurePolicy) {
	upgradePolicyMap[policy] = action
}

func (receiver AutoReloadPolicy) Upgrade(params reconfigureContext) (ReturnedStatus, error) {
	_ = params
	return makeReturnedStatus(ESNone), nil
}

func (receiver AutoReloadPolicy) GetPolicyName() string {
	return string(parametersv1alpha1.AsyncDynamicReloadPolicy)
}

func enableSyncTrigger(reloadAction *parametersv1alpha1.ReloadAction) bool {
	if reloadAction == nil {
		return false
	}

	if reloadAction.TPLScriptTrigger != nil {
		return !core.IsWatchModuleForTplTrigger(reloadAction.TPLScriptTrigger)
	}

	if reloadAction.ShellTrigger != nil {
		return !core.IsWatchModuleForShellTrigger(reloadAction.ShellTrigger)
	}
	return false
}

func withSucceed(succeedCount int32) func(status *ReturnedStatus) {
	return func(status *ReturnedStatus) {
		status.SucceedCount = succeedCount
	}
}

func withExpected(expectedCount int32) func(status *ReturnedStatus) {
	return func(status *ReturnedStatus) {
		status.ExpectedCount = expectedCount
	}
}

func makeReturnedStatus(status ExecStatus, ops ...func(status *ReturnedStatus)) ReturnedStatus {
	ret := ReturnedStatus{
		Status:        status,
		SucceedCount:  core.Unconfirmed,
		ExpectedCount: core.Unconfirmed,
	}
	for _, o := range ops {
		o(&ret)
	}
	return ret
}
