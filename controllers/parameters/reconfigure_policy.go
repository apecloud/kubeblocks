/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"math"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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
	ConfigTemplate appsv1.ComponentTemplateSpec

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
	UpdatedParameters map[string]string
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

func (param *reconfigureContext) getConfigKey() string {
	key := param.ConfigTemplate.Name
	if param.ConfigDescription != nil && param.ConfigDescription.Name != "" {
		hash, _ := util.ComputeHash(param.ConfigDescription.Name)
		key = key + "/" + hash
	}
	return key
}

func (param *reconfigureContext) getTargetVersionHash() string {
	hash, err := util.ComputeHash(param.ConfigMap.Data)
	if err != nil {
		param.Log.Error(err, "failed to get configuration version!")
		return ""
	}

	return hash
}

func (param *reconfigureContext) maxRollingReplicas() int32 {
	var (
		defaultRolling int32 = 1
		r              int32
		replicas       = param.getTargetReplicas()
	)

	if param.SynthesizedComponent == nil {
		return defaultRolling
	}

	var maxUnavailable *intstr.IntOrString
	for _, its := range param.InstanceSetUnits {
		if its.Spec.UpdateStrategy.RollingUpdate != nil {
			maxUnavailable = its.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable
		}
		if maxUnavailable != nil {
			break
		}
	}

	if maxUnavailable == nil {
		return defaultRolling
	}

	// TODO(xingran&zhangtao): review this logic, set to nil in new API version
	v, isPercentage, err := intctrlutil.GetIntOrPercentValue(maxUnavailable)
	if err != nil {
		param.Log.Error(err, "failed to get maxUnavailable!")
		return defaultRolling
	}

	if isPercentage {
		r = int32(math.Floor(float64(v) * float64(replicas) / 100))
	} else {
		r = util.Safe2Int32(min(v, param.getTargetReplicas()))
	}
	return max(r, defaultRolling)
}

func (param *reconfigureContext) getTargetReplicas() int {
	return int(param.ClusterComponent.Replicas)
}

func (param *reconfigureContext) podMinReadySeconds() int32 {
	minReadySeconds := param.SynthesizedComponent.MinReadySeconds
	return max(minReadySeconds, viper.GetInt32(constant.PodMinReadySecondsEnv))
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

func fromWorkloadObjects(params reconfigureContext) []client.Object {
	r := make([]client.Object, 0)
	for _, unit := range params.InstanceSetUnits {
		r = append(r, &unit)
	}
	return r
}
