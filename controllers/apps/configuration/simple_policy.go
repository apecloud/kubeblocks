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
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type simplePolicy struct {
}

func init() {
	RegisterPolicy(appsv1alpha1.NormalPolicy, &simplePolicy{})
}

func (s *simplePolicy) Upgrade(params reconfigureParams) (ReturnedStatus, error) {
	params.Ctx.Log.V(1).Info("simple policy begin....")

	switch params.WorkloadType() {
	case appsv1alpha1.Stateful, appsv1alpha1.Consensus, appsv1alpha1.Replication:
		return rollingStatefulSets(params)
	default:
		return makeReturnedStatus(ESNotSupport), cfgcore.MakeError("not support component workload type:[%s]", params.WorkloadType())
	}
}

func (s *simplePolicy) GetPolicyName() string {
	return string(appsv1alpha1.NormalPolicy)
}

func rollingStatefulSets(param reconfigureParams) (ReturnedStatus, error) {
	var (
		units      = param.ComponentUnits
		client     = param.Client
		newVersion = param.getTargetVersionHash()
		configKey  = param.getConfigKey()
		progress   = cfgcore.NotStarted
		retStatus  = ESRetry
	)

	if configKey == "" {
		return makeReturnedStatus(ESFailed), cfgcore.MakeError("failed to find config meta. configmap : %s", param.ConfigSpecName)
	}

	for _, sts := range units {
		if err := restartStsWithRolling(client, param.Ctx, sts, configKey, newVersion); err != nil {
			param.Ctx.Log.Error(err, "failed to restart statefulSet.", "stsName", sts.GetName())
			return makeReturnedStatus(ESAndRetryFailed), err
		}
	}

	pods, err := GetComponentPods(param)
	if err != nil {
		return makeReturnedStatus(ESAndRetryFailed), err
	}
	if len(pods) != 0 {
		progress = CheckReconfigureUpdateProgress(pods, configKey, newVersion)
	}
	if len(pods) == int(progress) {
		retStatus = ESNone
	}
	return makeReturnedStatus(retStatus, withExpected(int32(len(pods))), withSucceed(progress)), nil
}

func restartStsWithRolling(cli client.Client, ctx intctrlutil.RequestCtx, sts appsv1.StatefulSet, configKey string, newVersion string) error {
	// cfgAnnotationKey := fmt.Sprintf("%s-%s", UpgradeRestartAnnotationKey, strings.ReplaceAll(configKey, "_", "-"))
	cfgAnnotationKey := cfgcore.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)

	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = map[string]string{}
	}

	lastVersion := ""
	if updatedVersion, ok := sts.Spec.Template.Annotations[cfgAnnotationKey]; ok {
		lastVersion = updatedVersion
	}

	// updated UpgradeRestartAnnotationKey
	if lastVersion == newVersion {
		return nil
	}

	sts.Spec.Template.Annotations[cfgAnnotationKey] = newVersion
	if err := cli.Update(ctx.Ctx, &sts); err != nil {
		return err
	}

	return nil
}
