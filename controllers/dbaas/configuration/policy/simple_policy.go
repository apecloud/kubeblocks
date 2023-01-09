/*
Copyright ApeCloud Inc.

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

package policy

import (
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type simplePolicy struct {
}

func init() {
	RegisterPolicy(dbaasv1alpha1.NormalPolicy, &simplePolicy{})
}

func (s *simplePolicy) Upgrade(params ReconfigureParams) (ExecStatus, error) {
	params.Ctx.Log.V(1).Info("simple policy begin....")

	switch params.ComponentType() {
	case dbaasv1alpha1.Stateful, dbaasv1alpha1.Consensus:
		return rollingStatefulSets(params)
		// process consensus
	default:
		return ESNotSupport, cfgcore.MakeError("not support component type:[%s]", params.ComponentType())
	}
}

func (s *simplePolicy) GetPolicyName() string {
	return string(dbaasv1alpha1.NormalPolicy)
}

func rollingStatefulSets(param ReconfigureParams) (ExecStatus, error) {
	var (
		units      = param.ComponentUnits
		client     = param.Client
		newVersion = param.getModifyVersion()
		configKey  = param.getConfigKey()
	)

	if configKey == "" {
		return ESFailed, cfgcore.MakeError("failed to found config meta. configmap : %s", param.TplName)
	}

	for _, sts := range units {
		if err := restartStsWithRolling(client, param.Ctx, sts, configKey, newVersion); err != nil {
			param.Ctx.Log.Error(err, "failed to restart statefulSet.", "stsName", sts.GetName())
			return ESAndRetryFailed, err
		}
	}
	return ESNone, nil
}

func restartStsWithRolling(cli client.Client, ctx intctrlutil.RequestCtx, sts appsv1.StatefulSet, configKey string, newVersion string) error {
	// cfgAnnotationKey := fmt.Sprintf("%s-%s", UpgradeRestartAnnotationKey, strings.ReplaceAll(configKey, "_", "-"))
	cfgAnnotationKey := cfgcore.GenerateUniqKeyWithConfig(cfgcore.UpgradeRestartAnnotationKey, configKey)

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
