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

package operations

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

func init() {
	var (
		reAction   = reconfigureAction{}
		opsManager = GetOpsManager()
	)

	reconfigureBehaviour := &OpsBehaviour{
		FromClusterPhases: []dbaasv1alpha1.Phase{
			// all phase
			dbaasv1alpha1.RunningPhase,
			dbaasv1alpha1.FailedPhase,
			dbaasv1alpha1.AbnormalPhase,
		},
		ToClusterPhase:         dbaasv1alpha1.UpdatingPhase,
		Action:                 reAction.Reconfigure,
		ActionStartedCondition: dbaasv1alpha1.NewReconfigureCondition,
		ReconcileAction:        ReconcileActionWithComponentOps,
	}
	opsManager.RegisterOps(dbaasv1alpha1.ReconfigureType, reconfigureBehaviour)
}

type reconfigureAction struct {
}

func (r *reconfigureAction) Reconfigure(resource *OpsResource) error {
	spec := &resource.OpsRequest.Spec
	if len(spec.ComponentOpsList) != 1 {
		return cfgcore.MakeError("require reconfigure only update one component. current component:%d", len(spec.ComponentOpsList))
	}

	component := spec.ComponentOpsList[0]
	if len(component.ComponentNames) != 1 {
		return cfgcore.MakeError("require reconfigure only update one component, components: %s", component.ComponentNames)
	}

	if component.Reconfigure != nil {
		return cfgcore.MakeError("invalid reconfigure params.")
	}

	var (
		clusterName       = spec.ClusterRef
		componentName     = component.ComponentNames[0]
		cluster           = resource.Cluster
		clusterDefinition = &dbaasv1alpha1.ClusterDefinition{}
		appVersion        = &dbaasv1alpha1.AppVersion{}
	)

	if err := resource.Client.Get(resource.Ctx, client.ObjectKey{
		Name:      cluster.Spec.ClusterDefRef,
		Namespace: cluster.Namespace,
	}, clusterDefinition); err != nil {
		return cfgcore.WrapError(err, "failed to get clusterdefinition[%s]", cluster.Spec.ClusterDefRef)
	}

	if err := resource.Client.Get(resource.Ctx, client.ObjectKey{
		Name:      cluster.Spec.AppVersionRef,
		Namespace: cluster.Namespace,
	}, appVersion); err != nil {
		return cfgcore.WrapError(err, "failed to get appversion[%s]", cluster.Spec.AppVersionRef)
	}

	tpls, err := getConfigTemplatesFromComponent(
		cluster.Spec.Components,
		clusterDefinition.Spec.Components,
		appVersion.Spec.Components,
		componentName)
	if err != nil {
		return cfgcore.WrapError(err, "failed to get config template[%s]", componentName)
	}
	return r.performUpgrade(clusterName, componentName, component.Reconfigure, resource, tpls)
}

func (r *reconfigureAction) performUpgrade(clusterName, componentName string,
	reconfigure *dbaasv1alpha1.UpgradeConfiguration,
	resource *OpsResource,
	tpls []dbaasv1alpha1.ConfigTemplate) error {
	findTpl := func(tplName string) *dbaasv1alpha1.ConfigTemplate {
		if len(tplName) == 0 && len(tpls) == 1 {
			return &tpls[0]
		}
		for _, tpl := range tpls {
			if tpl.Name == tplName {
				return &tpl
			}
		}
		return nil
	}

	// Update params to configmap
	for _, config := range reconfigure.Configurations {
		tpl := findTpl(config.Name)
		if tpl == nil {
			return cfgcore.MakeError("failed to reconfigure, not exist config[%s], all configs: %v", config.Name, tpls)
		}
		if len(tpl.ConfigConstraintRef) == 0 {
			return cfgcore.MakeError("current tpl not support reconfigure, tpl: %v", tpl)
		}
		if err := updateCfgParams(config, *tpl, client.ObjectKey{
			Name:      cfgcore.GetComponentCMName(clusterName, componentName, *tpl),
			Namespace: resource.Cluster.Namespace,
		}, resource.Ctx, resource.Client); err != nil {
			return cfgcore.WrapError(err, "failed to update param!")
		}
	}
	return nil
}
