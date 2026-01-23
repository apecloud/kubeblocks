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
	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

var restartPolicyInstance = &restartPolicy{}

type restartPolicy struct{}

func init() {
	registerPolicy(parametersv1alpha1.RestartPolicy, restartPolicyInstance)
}

func (s *restartPolicy) Upgrade(rctx reconfigureContext) (returnedStatus, error) {
	rctx.Log.V(1).Info("simple policy begin....")

	return s.restartAndVerifyComponent(rctx, GetInstanceSetRollingUpgradeFuncs())
}

func (s *restartPolicy) restartAndVerifyComponent(rctx reconfigureContext, funcs RollingUpgradeFuncs) (returnedStatus, error) {
	var (
		newVersion = rctx.getTargetVersionHash()
		configKey  = rctx.generateConfigIdentifier()

		retStatus = ESRetry
		progress  = core.NotStarted
	)

	if err := funcs.RestartComponent(rctx.Client, rctx.RequestCtx, configKey, newVersion, rctx.Cluster, rctx.ClusterComponent.Name); err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}

	pods, err := funcs.GetPodsFunc(rctx)
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}

	if len(pods) != 0 {
		progress = checkReconfigureUpdateProgress(pods, configKey, newVersion)
	}

	if len(pods) == int(progress) {
		// check component phase when all pods are of expected version and ready
		comp, err := intctrlcomp.GetComponentByName(rctx.Ctx, rctx.Client, rctx.Cluster.Namespace, constant.GenerateClusterComponentName(rctx.Cluster.Name, rctx.ClusterComponent.Name))
		if err != nil {
			return makeReturnedStatus(ESFailedAndRetry), err
		}

		if comp.Status.Phase != appsv1.RunningComponentPhase {
			retStatus = ESRetry
		} else {
			retStatus = ESNone
		}
	}
	return makeReturnedStatus(retStatus, withExpected(int32(len(pods))), withSucceed(progress)), nil
}

// checkReconfigureUpdateProgress checks pods of the component is ready.
func checkReconfigureUpdateProgress(pods []corev1.Pod, configKey, version string) int32 {
	var (
		readyPods        int32 = 0
		cfgAnnotationKey       = core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)
	)

	for _, pod := range pods {
		annotations := pod.Annotations
		if len(annotations) != 0 && annotations[cfgAnnotationKey] == version && intctrlutil.IsPodReady(&pod) {
			readyPods++
		}
	}
	return readyPods
}
