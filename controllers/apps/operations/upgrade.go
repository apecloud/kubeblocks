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

package operations

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type upgradeOpsHandler struct{}

var _ OpsHandler = upgradeOpsHandler{}

func init() {
	upgradeBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may can repair it.
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        upgradeOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.UpgradeType, upgradeBehaviour)
}

// ActionStartedCondition the started condition when handle the upgrade request.
func (u upgradeOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewHorizontalScalingCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.clusterVersionRef with opsRequest.spec.upgrade.clusterVersionRef
func (u upgradeOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var compOpsHelper componentOpsHelper
	upgradeSpec := opsRes.OpsRequest.Spec.Upgrade
	if u.existClusterVersion(opsRes.OpsRequest) {
		// TODO: remove this deprecated API after v0.9
		opsRes.Cluster.Spec.ClusterVersionRef = *opsRes.OpsRequest.Spec.Upgrade.ClusterVersionRef
	} else {
		compOpsHelper = newComponentOpsHelper(upgradeSpec.Components)
		if err := compOpsHelper.updateClusterComponentsAndShardings(opsRes.Cluster, func(compSpec *appsv1alpha1.ClusterComponentSpec, obj ComponentOpsInterface) error {
			upgradeComp := obj.(appsv1alpha1.UpgradeComponent)
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
	}
	// abort earlier running upgrade opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []appsv1alpha1.OpsType{appsv1alpha1.UpgradeType},
		func(earlierOps *appsv1alpha1.OpsRequest) (bool, error) {
			if u.existClusterVersion(earlierOps) {
				return true, nil
			}
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
func (u upgradeOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	upgradeSpec := opsRes.OpsRequest.Spec.Upgrade
	var (
		compOpsHelper       componentOpsHelper
		componentDefMap     map[string]*appsv1alpha1.ComponentDefinition
		componentVersionMap map[string]appsv1alpha1.ClusterComponentVersion
		err                 error
	)
	if u.existClusterVersion(opsRes.OpsRequest) {
		// TODO: remove this deprecated API after v0.9
		compOpsHelper = newComponentOpsHelper(u.getCompOpsListForClusterVersion(opsRes))
		if componentVersionMap, err = u.getClusterComponentVersionMap(reqCtx.Ctx, cli, *upgradeSpec.ClusterVersionRef); err != nil {
			return opsRes.OpsRequest.Status.Phase, 0, err
		}
	} else {
		compOpsHelper = newComponentOpsHelper(upgradeSpec.Components)
		if componentDefMap, err = u.getComponentDefMapWithUpdatedImages(reqCtx, cli, opsRes); err != nil {
			return opsRes.OpsRequest.Status.Phase, 0, err
		}
	}
	componentUpgraded := func(cluster *appsv1alpha1.Cluster,
		lastCompConfiguration appsv1alpha1.LastComponentConfiguration,
		upgradeComp appsv1alpha1.UpgradeComponent) bool {
		if u.needUpdateCompDef(upgradeComp, opsRes.Cluster) &&
			lastCompConfiguration.ComponentDefinitionName != *upgradeComp.ComponentDefinitionName {
			return true
		}
		if upgradeComp.ServiceVersion != nil && lastCompConfiguration.ServiceVersion != *upgradeComp.ServiceVersion {
			return true
		}
		return false
	}
	podApplyCompOps := func(
		ops *appsv1alpha1.OpsRequest,
		pod *corev1.Pod,
		compOps ComponentOpsInterface,
		insTemplateName string) bool {
		if u.existClusterVersion(opsRes.OpsRequest) {
			// TODO: remove this deprecated API after v0.9
			compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, compOps.GetComponentName())
			if compSpec == nil {
				return true
			}
			compVersion, ok := componentVersionMap[compSpec.ComponentDefRef]
			if !ok {
				return true
			}
			return u.podImageApplied(pod, compVersion.VersionsCtx.Containers)
		}
		upgradeComponent := compOps.(appsv1alpha1.UpgradeComponent)
		lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[compOps.GetComponentName()]
		if !componentUpgraded(opsRes.Cluster, lastCompConfiguration, upgradeComponent) {
			// if componentDefinition and serviceVersion no changes, return true
			return true
		}
		compDef, ok := componentDefMap[compOps.GetComponentName()]
		if !ok {
			return true
		}
		return u.podImageApplied(pod, compDef.Spec.Runtime.Containers)
	}
	handleUpgradeProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, completedCount int32, err error) {
		return handleComponentStatusProgress(reqCtx, cli, opsRes, pgRes, compStatus, podApplyCompOps)
	}
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "upgrade", handleUpgradeProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (u upgradeOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRes.OpsRequest.Status.LastConfiguration.ClusterVersionRef = opsRes.Cluster.Spec.ClusterVersionRef
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.Upgrade.Components)
	compOpsHelper.saveLastConfigurations(opsRes, func(compSpec appsv1alpha1.ClusterComponentSpec, comOps ComponentOpsInterface) appsv1alpha1.LastComponentConfiguration {
		return appsv1alpha1.LastComponentConfiguration{
			ComponentDefinitionName: compSpec.ComponentDef,
			ServiceVersion:          compSpec.ServiceVersion,
		}
	})
	return nil
}

// getClusterComponentVersionMap gets the components of ClusterVersion and converts the component list to map.
func (u upgradeOpsHandler) getClusterComponentVersionMap(ctx context.Context,
	cli client.Client, clusterVersionName string) (map[string]appsv1alpha1.ClusterComponentVersion, error) {
	clusterVersion := &appsv1alpha1.ClusterVersion{}
	if err := cli.Get(ctx, client.ObjectKey{Name: clusterVersionName}, clusterVersion); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	components := map[string]appsv1alpha1.ClusterComponentVersion{}
	for _, v := range clusterVersion.Spec.ComponentVersions {
		components[v.ComponentDefRef] = v
	}
	return components, nil
}

func (u upgradeOpsHandler) getCompOpsListForClusterVersion(opsRes *OpsResource) []appsv1alpha1.ComponentOps {
	var compOpsList []appsv1alpha1.ComponentOps
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		compOpsList = append(compOpsList, appsv1alpha1.ComponentOps{ComponentName: v.Name})
	}
	for _, v := range opsRes.Cluster.Spec.ShardingSpecs {
		compOpsList = append(compOpsList, appsv1alpha1.ComponentOps{ComponentName: v.Name})
	}
	return compOpsList
}

// getComponentDefMapWithUpdatedImages gets the desired componentDefinition map
// that is updated with the corresponding images of the ComponentDefinition and service version.
func (u upgradeOpsHandler) getComponentDefMapWithUpdatedImages(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) (map[string]*appsv1alpha1.ComponentDefinition, error) {
	compDefMap := map[string]*appsv1alpha1.ComponentDefinition{}
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
		compDefMap[v.ComponentName] = compDef
	}
	return compDefMap, nil
}

// podImageApplied checks if the pod has applied the new image.
func (u upgradeOpsHandler) podImageApplied(pod *corev1.Pod, expectContainers []corev1.Container) bool {
	if len(expectContainers) == 0 {
		return true
	}
	for _, v := range expectContainers {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == v.Name && cs.Image != v.Image {
				return false
			}
		}
		for _, c := range pod.Spec.Containers {
			if c.Name == v.Name && c.Image != v.Image {
				return false
			}
		}
	}
	return true
}

func (u upgradeOpsHandler) existClusterVersion(ops *appsv1alpha1.OpsRequest) bool {
	return ops.Spec.Upgrade.ClusterVersionRef != nil && *ops.Spec.Upgrade.ClusterVersionRef != ""
}

func (u upgradeOpsHandler) needUpdateCompDef(upgradeComp appsv1alpha1.UpgradeComponent, cluster *appsv1alpha1.Cluster) bool {
	if upgradeComp.ComponentDefinitionName == nil {
		return false
	}
	// we will ignore the empty ComponentDefinitionName if cluster.Spec.ClusterDefRef is empty.
	return *upgradeComp.ComponentDefinitionName != "" ||
		(*upgradeComp.ComponentDefinitionName == "" && cluster.Spec.ClusterDefRef != "")
}
