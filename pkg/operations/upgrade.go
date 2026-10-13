/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package operations

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type upgradeOpsHandler struct{}

var _ OpsHandler = upgradeOpsHandler{}

func init() {
	upgradeBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may can repair it.
		FromClusterPhases: appsv1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        upgradeOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.UpgradeType, upgradeBehaviour)
}

// ActionStartedCondition the started condition when handle the upgrade request.
func (u upgradeOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewUpgradingCondition(opsRes.OpsRequest), nil
}

func (u upgradeOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var compOpsHelper componentOpsHelper
	upgradeSpec := opsRes.OpsRequest.Spec.Upgrade
	compOpsHelper = newComponentOpsHelper(upgradeSpec.Components)
	if err := compOpsHelper.updateClusterComponentsAndShardings(opsRes.Cluster, func(compSpec *appsv1.ClusterComponentSpec, obj ComponentOpsInterface) error {
		upgradeComp := obj.(opsv1alpha1.UpgradeComponent)
		if u.needUpdateCompDef(upgradeComp, opsRes.Cluster) {
			compSpec.ComponentDef = *upgradeComp.ComponentDefinitionName
		}
		if upgradeComp.ServiceVersion != nil {
			compSpec.ServiceVersion = *upgradeComp.ServiceVersion
		}
		return nil
	}); err != nil {
		return err
	}
	// abort earlier running upgrade opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.UpgradeType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			for _, v := range earlierOps.Spec.Upgrade.Components {
				// abort the earlierOps if exists the same component.
				if _, ok := compOpsHelper.componentOpsSet[v.ComponentName]; ok {
					return true, nil
				}
			}
			return false, nil
		}); err != nil {
		return err
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for upgrade opsRequest.
func (u upgradeOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	upgradeSpec := opsRes.OpsRequest.Spec.Upgrade
	var (
		compOpsHelper componentOpsHelper
		containersMap map[string]map[string][]corev1.Container
		err           error
	)
	compOpsHelper = newComponentOpsHelper(upgradeSpec.Components)
	if containersMap, err = u.getExpectedContainersMap(reqCtx, cli, opsRes); err != nil {
		return opsRes.OpsRequest.Status.Phase, 0, err
	}
	podApplyCompOps := func(
		ops *opsv1alpha1.OpsRequest,
		pod *corev1.Pod,
		pgRes *progressResource) bool {
		upgradeComponent := pgRes.compOps.(opsv1alpha1.UpgradeComponent)
		templates, ok := containersMap[upgradeComponent.GetComponentName()]
		if !ok {
			return true
		}
		templateName := appsv1.GetInstanceTemplateName(opsRes.Cluster.Name, pgRes.fullComponentName, pod.Name)
		containers, ok := templates[templateName]
		if !ok {
			containers = templates[""]
		}
		return u.podImageApplied(pod, containers)
	}
	handleUpgradeProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *opsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, completedCount int32, err error) {
		return handleComponentStatusProgress(reqCtx, cli, opsRes, pgRes, compStatus, podApplyCompOps)
	}
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "upgrade", handleUpgradeProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (u upgradeOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.Upgrade.Components)
	compOpsHelper.saveLastConfigurations(opsRes, func(compSpec appsv1.ClusterComponentSpec, comOps ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration {
		return opsv1alpha1.LastComponentConfiguration{
			ComponentDefinitionName: compSpec.ComponentDef,
			ServiceVersion:          compSpec.ServiceVersion,
		}
	})
	return nil
}

func (u upgradeOpsHandler) getExpectedContainersMap(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) (map[string]map[string][]corev1.Container, error) {
	containersMap := map[string]map[string][]corev1.Container{}
	for _, v := range opsRes.OpsRequest.Spec.Upgrade.Components {
		compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, v.ComponentName)
		if compSpec == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`"can not found the component "%s" in the cluster "%s"`,
				v.ComponentName, opsRes.Cluster.Name))
		}
		compDef, err := component.GetCompDefByName(reqCtx.Ctx, cli, compSpec.ComponentDef)
		if err != nil {
			return nil, err
		}
		if err = component.UpdateCompDefinitionImages4ServiceVersion(reqCtx.Ctx, cli, compDef, compSpec.ServiceVersion); err != nil {
			return nil, err
		}
		templates := map[string][]corev1.Container{"": compDef.Spec.Runtime.Containers}
		for _, template := range compSpec.Instances {
			if template.ServiceVersion == "" {
				continue
			}
			templateCompDef, err := component.GetCompDefByName(reqCtx.Ctx, cli, template.CompDef)
			if err != nil {
				return nil, err
			}
			images, err := component.ResolveInstanceTemplateImages4ServiceVersion(reqCtx.Ctx, cli, templateCompDef, template.ServiceVersion)
			if err != nil {
				return nil, err
			}
			containers := append([]corev1.Container(nil), compDef.Spec.Runtime.Containers...)
			for i := range containers {
				if image := images[containers[i].Name]; image != "" {
					containers[i].Image = image
				}
			}
			templates[template.Name] = containers
		}
		containersMap[v.ComponentName] = templates
	}
	return containersMap, nil
}

// podImageApplied checks if the pod has applied the new image.
func (u upgradeOpsHandler) podImageApplied(pod *corev1.Pod, expectContainers []corev1.Container) bool {
	if len(expectContainers) == 0 {
		return true
	}
	imageName := func(image string) string {
		images := strings.Split(image, "/")
		return images[len(images)-1]
	}
	for _, v := range expectContainers {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == v.Name && imageName(cs.Image) != imageName(v.Image) {
				return false
			}
		}
		for _, c := range pod.Spec.Containers {
			if c.Name == v.Name && imageName(c.Image) != imageName(v.Image) {
				return false
			}
		}
	}
	return true
}

func (u upgradeOpsHandler) needUpdateCompDef(upgradeComp opsv1alpha1.UpgradeComponent, cluster *appsv1.Cluster) bool {
	if upgradeComp.ComponentDefinitionName == nil {
		return false
	}
	// we will ignore the empty ComponentDefinitionName if cluster.Spec.clusterDef is empty.
	return *upgradeComp.ComponentDefinitionName != "" ||
		(*upgradeComp.ComponentDefinitionName == "" && cluster.Spec.ClusterDef != "")
}
