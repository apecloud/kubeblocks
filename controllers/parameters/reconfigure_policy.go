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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	configmanager "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
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
	Status       ExecStatus
	SucceedCount int32
	ExpectedCount int32
}

type reconfigurePolicy interface {
	// Upgrade is to enable the configuration to take effect.
	Upgrade(params reconfigureParams) (ReturnedStatus, error)

	// GetPolicyName returns name of policy.
	GetPolicyName() string
}

type AutoReloadPolicy struct{}

type reconfigureParams struct {
	// Only supports restart pod or container.
	Restart bool

	// Name is a config template name.
	ConfigSpecName string

	// Configuration files patch.
	ConfigPatch *core.ConfigPatchInfo

	// Configmap object of the configuration template instance in the component.
	ConfigMap *corev1.ConfigMap

	// ConfigConstraint pointer
	ConfigConstraint *appsv1beta1.ConfigConstraintSpec

	// For grpc factory
	ReconfigureClientFactory createReconfigureClient

	// List of containers using this config volume.
	ContainerNames []string

	Client client.Client
	Ctx    intctrlutil.RequestCtx

	Cluster *appsv1.Cluster

	// Associated component for cluster.
	ClusterComponent *appsv1.ClusterComponentSpec

	// Associated component for component and component definition.
	SynthesizedComponent *component.SynthesizedComponent

	// List of InstanceSet using this config template.
	InstanceSetUnits []workloads.InstanceSet
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

var upgradePolicyMap = map[appsv1alpha1.UpgradePolicy]reconfigurePolicy{}

func init() {
	RegisterPolicy(appsv1alpha1.AsyncDynamicReloadPolicy, &AutoReloadPolicy{})
}

// GetClientFactory support ut mock
func GetClientFactory() createReconfigureClient {
	return newGRPCClient
}

func (param *reconfigureParams) getConfigKey() string {
	return param.ConfigSpecName
}

func (param *reconfigureParams) getTargetVersionHash() string {
	hash, err := util.ComputeHash(param.ConfigMap.Data)
	if err != nil {
		param.Ctx.Log.Error(err, "failed to get configuration version!")
		return ""
	}

	return hash
}

func (param *reconfigureParams) maxRollingReplicas() int32 {
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
		param.Ctx.Log.Error(err, "failed to get maxUnavailable!")
		return defaultRolling
	}

	if isPercentage {
		r = int32(math.Floor(float64(v) * float64(replicas) / 100))
	} else {
		r = util.Safe2Int32(min(v, param.getTargetReplicas()))
	}
	return max(r, defaultRolling)
}

func (param *reconfigureParams) getTargetReplicas() int {
	return int(param.ClusterComponent.Replicas)
}

func (param *reconfigureParams) podMinReadySeconds() int32 {
	minReadySeconds := param.SynthesizedComponent.MinReadySeconds
	return max(minReadySeconds, viper.GetInt32(constant.PodMinReadySecondsEnv))
}

func RegisterPolicy(policy appsv1alpha1.UpgradePolicy, action reconfigurePolicy) {
	upgradePolicyMap[policy] = action
}

func (receiver AutoReloadPolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	_ = params
	return makeReturnedStatus(ESNone), nil
}

func (receiver AutoReloadPolicy) GetPolicyName() string {
	return string(appsv1alpha1.AsyncDynamicReloadPolicy)
}

func NewReconfigurePolicy(cc *appsv1beta1.ConfigConstraintSpec, cfgPatch *core.ConfigPatchInfo, policy appsv1alpha1.UpgradePolicy, restart bool) (reconfigurePolicy, error) {
	if cfgPatch != nil && !cfgPatch.IsModify {
		// not walk here
		return nil, core.MakeError("cfg not modify. [%v]", cfgPatch)
	}

	// if not specify policy, auto decision reconfiguring policy.
	if enableAutoDecision(restart, policy) {
		dynamicUpdate, err := core.IsUpdateDynamicParameters(cc, cfgPatch)
		if err != nil {
			return nil, err
		}

		// make decision
		switch {
		case !dynamicUpdate: // static parameters update
		case configmanager.IsAutoReload(cc.ReloadAction): // if core support hot update, don't need to do anything
			policy = appsv1alpha1.AsyncDynamicReloadPolicy
		case enableSyncTrigger(cc.ReloadAction): // sync config-manager exec hot update
			policy = appsv1alpha1.SyncDynamicReloadPolicy
		default: // config-manager auto trigger to hot update
			policy = appsv1alpha1.AsyncDynamicReloadPolicy
		}
	}

	// if not specify policy, or cannot decision policy, use default policy.
	if policy == appsv1alpha1.NonePolicy {
		policy = appsv1alpha1.NormalPolicy
		if cc.NeedDynamicReloadAction() && enableSyncTrigger(cc.ReloadAction) {
			policy = appsv1alpha1.DynamicReloadAndRestartPolicy
		}
	}

	if action, ok := upgradePolicyMap[policy]; ok {
		return action, nil
	}
	return nil, core.MakeError("not supported upgrade policy:[%s]", policy)
}

func enableAutoDecision(restart bool, policy appsv1alpha1.UpgradePolicy) bool {
	return !restart && policy == appsv1alpha1.NonePolicy
}

func enableSyncTrigger(reloadAction *appsv1beta1.ReloadAction) bool {
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

func fromWorkloadObjects(params reconfigureParams) []client.Object {
	r := make([]client.Object, 0)
	for _, unit := range params.InstanceSetUnits {
		r = append(r, &unit)
	}
	return r
}
