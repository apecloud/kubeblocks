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
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	// pre-check if the desired componentDefinition and serviceVersion can be resolved before updating
	// the cluster spec, to fail the opsRequest fast instead of polluting the cluster spec and retrying forever.
	if err := u.upgradePreCheck(reqCtx, cli, opsRes); err != nil {
		return err
	}
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
		compOpsHelper   componentOpsHelper
		componentDefMap map[string]*appsv1.ComponentDefinition
		err             error
	)
	compOpsHelper = newComponentOpsHelper(upgradeSpec.Components)
	if componentDefMap, err = u.getComponentDefMapWithUpdatedImages(reqCtx, cli, opsRes); err != nil {
		return opsRes.OpsRequest.Status.Phase, 0, err
	}
	podApplyCompOps := func(
		ops *opsv1alpha1.OpsRequest,
		instance Instance,
		pgRes *progressResource) bool {
		upgradeComponent := pgRes.compOps.(opsv1alpha1.UpgradeComponent)
		compDef, ok := componentDefMap[upgradeComponent.GetComponentName()]
		if !ok {
			return true
		}
		return u.instanceImageApplied(instance, compDef.Spec.Runtime.Containers)
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

// upgradePreCheck checks if the desired componentDefinition and serviceVersion can be resolved
// before the cluster spec is updated by the Action.
func (u upgradeOpsHandler) upgradePreCheck(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	for _, v := range opsRes.OpsRequest.Spec.Upgrade.Components {
		compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, v.ComponentName)
		if compSpec == nil {
			return intctrlutil.NewFatalError(fmt.Sprintf(`"can not found the component "%s" in the cluster "%s"`,
				v.ComponentName, opsRes.Cluster.Name))
		}
		compDefName := compSpec.ComponentDef
		if u.needUpdateCompDef(v, opsRes.Cluster) {
			compDefName = *v.ComponentDefinitionName
		}
		serviceVersion := compSpec.ServiceVersion
		if v.ServiceVersion != nil {
			serviceVersion = *v.ServiceVersion
		}
		if compDefName == "" {
			// the specific componentDefinition will be resolved from the cluster topology by the cluster controller.
			continue
		}
		if _, err := u.getCompDefWithUpdatedImages(reqCtx, cli, compDefName, serviceVersion); err != nil {
			return err
		}
	}
	return nil
}

// getComponentDefMapWithUpdatedImages gets the desired componentDefinition map
// that is updated with the corresponding images of the ComponentDefinition and service version.
func (u upgradeOpsHandler) getComponentDefMapWithUpdatedImages(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) (map[string]*appsv1.ComponentDefinition, error) {
	compDefMap := map[string]*appsv1.ComponentDefinition{}
	for _, v := range opsRes.OpsRequest.Spec.Upgrade.Components {
		compSpec := getComponentSpecOrShardingTemplate(opsRes.Cluster, v.ComponentName)
		if compSpec == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`"can not found the component "%s" in the cluster "%s"`,
				v.ComponentName, opsRes.Cluster.Name))
		}
		compDef, err := u.getCompDefWithUpdatedImages(reqCtx, cli, compSpec.ComponentDef, compSpec.ServiceVersion)
		if err != nil {
			return nil, err
		}
		compDefMap[v.ComponentName] = compDef
	}
	return compDefMap, nil
}

// getCompDefWithUpdatedImages gets the componentDefinition that is updated with the images matching the
// given serviceVersion. if compDefName is a name prefix or regex pattern, it is resolved together with
// the serviceVersion first, with the same compatibility semantics as the cluster controller.
// the deterministic resolution failures (no componentDefinition matches the name, or the serviceVersion
// is not supported) are classified as fatal errors to fail the opsRequest fast instead of retrying forever.
func (u upgradeOpsHandler) getCompDefWithUpdatedImages(reqCtx intctrlutil.RequestCtx,
	cli client.Client, compDefName, serviceVersion string) (*appsv1.ComponentDefinition, error) {
	compDef, err := component.GetCompDefByName(reqCtx.Ctx, cli, compDefName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		// compDefName may be a name prefix or a regex pattern that will be resolved to a specific
		// componentDefinition by the cluster controller. resolve it together with the serviceVersion
		// here to detect the deterministic no-match failures before/without relying on the cluster controller,
		// with exactly the same compatibility semantics as the cluster controller's normalization.
		compDef, _, err = component.ResolveCompDefinitionNServiceVersion(reqCtx.Ctx, cli, compDefName, serviceVersion)
		if err != nil {
			if errors.Is(err, component.ErrCompDefinitionNServiceVersionUnmatched) {
				return nil, intctrlutil.NewFatalError(err.Error())
			}
			return nil, err
		}
	}
	if err = component.UpdateCompDefinitionImages4ServiceVersion(reqCtx.Ctx, cli, compDef, serviceVersion); err != nil {
		if strings.Contains(err.Error(), "no matched image found") {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`cannot find matched images with serviceVersion "%s" for the componentDefinition "%s": %s`,
				serviceVersion, compDef.Name, err.Error()))
		}
		return nil, err
	}
	return compDef, nil
}

// podImageApplied checks if the pod has applied the new image.
func (u upgradeOpsHandler) instanceImageApplied(instance Instance, expectContainers []corev1.Container) bool {
	if len(expectContainers) == 0 {
		return true
	}
	imageName := func(image string) string {
		images := strings.Split(image, "/")
		return images[len(images)-1]
	}
	for _, v := range expectContainers {
		if statusImage := instance.GetStatusImage(v.Name); statusImage != "" && imageName(statusImage) != imageName(v.Image) {
			return false
		}
		if image := instance.GetImage(v.Name); image != "" && imageName(image) != imageName(v.Image) {
			return false
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
