/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

	// GetPolicyName return name of policy.
	GetPolicyName() string
}

type AutoReloadPolicy struct{}

type reconfigureParams struct {
	// Only support restart pod or container.
	Restart bool

	// Name is a config template name.
	TplName string

	// Configuration files patch.
	ConfigPatch *cfgcore.ConfigPatchInfo

	// Configmap object of the configuration template instance in the component.
	CfgCM *corev1.ConfigMap

	// ConfigConstraint pointer
	ConfigConstraint *appsv1alpha1.ConfigConstraintSpec

	// For grpc factory
	ReconfigureClientFactory createReconfigureClient

	// List of container, using this config volume.
	ContainerNames []string

	Client client.Client
	Ctx    intctrlutil.RequestCtx

	Cluster *appsv1alpha1.Cluster

	// Associated component for cluster.
	ClusterComponent *appsv1alpha1.ClusterComponentSpec
	// Associated component for clusterdefinition.
	Component *appsv1alpha1.ClusterComponentDefinition

	// List of StatefulSet, using this config template.
	ComponentUnits []appv1.StatefulSet
}

var (
	// lazy create grpc connection
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
	for _, tpl := range param.Component.ConfigSpec.ConfigTemplateRefs {
		if tpl.Name == param.TplName {
			return tpl.VolumeName
		}
	}
	return ""
}

func (param *reconfigureParams) getTargetVersionHash() string {
	hash, err := cfgcore.ComputeHash(param.CfgCM.Data)
	if err != nil {
		param.Ctx.Log.Error(err, "failed to cal configuration version!")
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

	if param.Component.MaxUnavailable == nil {
		return defaultRolling
	}

	v, isPercent, err := intctrlutil.GetIntOrPercentValue(param.Component.MaxUnavailable)
	if err != nil {
		param.Ctx.Log.Error(err, "failed to get MaxUnavailable!")
		return defaultRolling
	}

	if isPercent {
		r = int32(math.Floor(float64(v) * float64(replicas) / 100))
	} else {
		r = int32(cfgcore.Min(v, param.getTargetReplicas()))
	}
	return cfgcore.Max(r, defaultRolling)
}

func (param *reconfigureParams) getTargetReplicas() int {
	return int(param.ClusterComponent.Replicas)
}

func (param *reconfigureParams) podMinReadySeconds() int32 {
	minReadySeconds := param.ComponentUnits[0].Spec.MinReadySeconds
	return cfgcore.Max(minReadySeconds, viper.GetInt32(constant.PodMinReadySecondsEnv))
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

func NewReconfigurePolicy(tpl *appsv1alpha1.ConfigConstraintSpec, cfgPatch *cfgcore.ConfigPatchInfo, policy appsv1alpha1.UpgradePolicy, restart bool) (reconfigurePolicy, error) {
	if !cfgPatch.IsModify {
		// not exec here
		return nil, cfgcore.MakeError("cfg not modify. [%v]", cfgPatch)
	}

	actionType := policy
	if !restart {
		if dynamicUpdate, err := isUpdateDynamicParameters(tpl, cfgPatch); err != nil {
			return nil, err
		} else if dynamicUpdate {
			actionType = appsv1alpha1.AutoReload
		}
	}

	if action, ok := upgradePolicyMap[actionType]; ok {
		return action, nil
	}
	return nil, cfgcore.MakeError("not support upgrade policy:[%s]", actionType)
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
