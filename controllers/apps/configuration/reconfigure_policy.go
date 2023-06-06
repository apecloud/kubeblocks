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

package configuration

import (
	"math"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ExecStatus defines running result for Reconfiguring policy (fsm).
// ESNone describes policy has finished and quit.
// ESRetry describes fsm is running.
// ESFailed describes fsm is failed and exited.
// ESNotSupport describes fsm does not support the feature.
// ESAndRetryFailed describes fsm is failed in current state, but can be retried.
// +enum
type ExecStatus string

const (
	ESNone           ExecStatus = "None"
	ESRetry          ExecStatus = "Retry"
	ESFailed         ExecStatus = "Failed"
	ESNotSupport     ExecStatus = "NotSupport"
	ESAndRetryFailed ExecStatus = "FailedAndRetry"
)

type ReturnedStatus struct {
	Status        ExecStatus
	SucceedCount  int32
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
	ConfigPatch *cfgcore.ConfigPatchInfo

	// Configmap object of the configuration template instance in the component.
	ConfigMap *corev1.ConfigMap

	// ConfigConstraint pointer
	ConfigConstraint *appsv1alpha1.ConfigConstraintSpec

	// For grpc factory
	ReconfigureClientFactory createReconfigureClient

	// List of containers using this config volume.
	ContainerNames []string

	Client client.Client
	Ctx    intctrlutil.RequestCtx

	Cluster *appsv1alpha1.Cluster

	// Associated component for cluster.
	ClusterComponent *appsv1alpha1.ClusterComponentSpec
	// Associated component for clusterdefinition.
	Component *appsv1alpha1.ClusterComponentDefinition

	// List of StatefulSets using this config template.
	ComponentUnits []appv1.StatefulSet
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
	RegisterPolicy(appsv1alpha1.AutoReload, &AutoReloadPolicy{})
}

func (param *reconfigureParams) WorkloadType() appsv1alpha1.WorkloadType {
	return param.Component.WorkloadType
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

	if param.Component.GetMaxUnavailable() == nil {
		return defaultRolling
	}

	v, isPercentage, err := intctrlutil.GetIntOrPercentValue(param.Component.GetMaxUnavailable())
	if err != nil {
		param.Ctx.Log.Error(err, "failed to get maxUnavailable!")
		return defaultRolling
	}

	if isPercentage {
		r = int32(math.Floor(float64(v) * float64(replicas) / 100))
	} else {
		r = int32(util.Min(v, param.getTargetReplicas()))
	}
	return util.Max(r, defaultRolling)
}

func (param *reconfigureParams) getTargetReplicas() int {
	return int(param.ClusterComponent.Replicas)
}

func (param *reconfigureParams) podMinReadySeconds() int32 {
	minReadySeconds := param.ComponentUnits[0].Spec.MinReadySeconds
	return util.Max(minReadySeconds, viper.GetInt32(constant.PodMinReadySecondsEnv))
}

func RegisterPolicy(policy appsv1alpha1.UpgradePolicy, action reconfigurePolicy) {
	upgradePolicyMap[policy] = action
}

func (receiver AutoReloadPolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	_ = params
	return makeReturnedStatus(ESNone), nil
}

func (receiver AutoReloadPolicy) GetPolicyName() string {
	return string(appsv1alpha1.AutoReload)
}

func NewReconfigurePolicy(cc *appsv1alpha1.ConfigConstraintSpec, cfgPatch *cfgcore.ConfigPatchInfo, policy appsv1alpha1.UpgradePolicy, restart bool) (reconfigurePolicy, error) {
	if !cfgPatch.IsModify {
		// not walk here
		return nil, cfgcore.MakeError("cfg not modify. [%v]", cfgPatch)
	}

	if enableAutoDecision(restart, policy) {
		if dynamicUpdate, err := cfgcore.IsUpdateDynamicParameters(cc, cfgPatch); err != nil {
			return nil, err
		} else if dynamicUpdate {
			policy = appsv1alpha1.AutoReload
		}
		if enableSyncReload(policy, cc.ReloadOptions) {
			policy = appsv1alpha1.OperatorSyncUpdate
		}
	}
	if policy == appsv1alpha1.NonePolicy {
		policy = appsv1alpha1.NormalPolicy
	}
	if action, ok := upgradePolicyMap[policy]; ok {
		return action, nil
	}
	return nil, cfgcore.MakeError("not supported upgrade policy:[%s]", policy)
}

func enableAutoDecision(restart bool, policy appsv1alpha1.UpgradePolicy) bool {
	return !restart && policy == appsv1alpha1.NonePolicy
}

func enableSyncReload(policyType appsv1alpha1.UpgradePolicy, options *appsv1alpha1.ReloadOptions) bool {
	return policyType == appsv1alpha1.AutoReload && enableSyncTrigger(options)
}

func enableSyncTrigger(options *appsv1alpha1.ReloadOptions) bool {
	if options == nil || options.TPLScriptTrigger == nil {
		return false
	}

	trigger := options.TPLScriptTrigger
	return trigger.Sync != nil && *trigger.Sync
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
		SucceedCount:  cfgcore.Unconfirmed,
		ExpectedCount: cfgcore.Unconfirmed,
	}
	for _, o := range ops {
		o(&ret)
	}
	return ret
}
