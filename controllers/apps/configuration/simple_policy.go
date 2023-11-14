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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type simplePolicy struct {
}

func init() {
	RegisterPolicy(appsv1alpha1.NormalPolicy, &simplePolicy{})
}

func (s *simplePolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	params.Ctx.Log.V(1).Info("simple policy begin....")

	return restartAndCheckComponent(params, GetRSMRollingUpgradeFuncs(), fromWorkloadObjects(params))
}

func (s *simplePolicy) GetPolicyName() string {
	return string(appsv1alpha1.NormalPolicy)
}

func restartAndCheckComponent(param reconfigureParams, funcs RollingUpgradeFuncs, objs []client.Object) (ReturnedStatus, error) {
	var (
		newVersion = param.getTargetVersionHash()
		configKey  = param.getConfigKey()

		retStatus = ESRetry
		progress  = core.NotStarted
	)

	recordEvent := func(obj client.Object) {
		param.Ctx.Recorder.Eventf(obj,
			corev1.EventTypeNormal, appsv1alpha1.ReasonReconfigureRestart,
			"restarting component[%s] in cluster[%s], version: %s", param.ClusterComponent.Name, param.Cluster.Name, newVersion)
	}
	if obj, err := funcs.RestartComponent(param.Client, param.Ctx, configKey, newVersion, objs, recordEvent); err != nil {
		param.Ctx.Recorder.Eventf(obj,
			corev1.EventTypeWarning, appsv1alpha1.ReasonReconfigureRestartFailed,
			"failed to  restart component[%s] in cluster[%s], version: %s", client.ObjectKeyFromObject(obj), param.Cluster.Name, newVersion)
		return makeReturnedStatus(ESFailedAndRetry), err
	}

	pods, err := funcs.GetPodsFunc(param)
	if err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}
	if len(pods) != 0 {
		progress = CheckReconfigureUpdateProgress(pods, configKey, newVersion)
	}
	if len(pods) == int(progress) {
		retStatus = ESNone
	}
	return makeReturnedStatus(retStatus, withExpected(int32(len(pods))), withSucceed(progress)), nil
}
