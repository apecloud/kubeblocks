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
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type upgradeOpsHandler struct{}

type upgradeExpectedImages struct {
	definition           *appsv1.ComponentDefinition
	containersByTemplate map[string][]corev1.Container
	pendingOwner         bool
}

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
		containersMap map[string]upgradeExpectedImages
		err           error
	)
	compOpsHelper = newComponentOpsHelper(upgradeSpec.Components)
	specs, pending, err := u.getUpgradeComponentSpecs(reqCtx, cli, opsRes)
	if err != nil {
		return opsRes.OpsRequest.Status.Phase, 0, err
	}
	if pending {
		return opsv1alpha1.OpsRunningPhase, time.Second, nil
	}
	if containersMap, err = u.getExpectedContainersMap(reqCtx, cli, opsRes.Cluster, specs); err != nil {
		return opsRes.OpsRequest.Status.Phase, 0, err
	}
	for _, expected := range containersMap {
		if expected.pendingOwner {
			return opsv1alpha1.OpsRunningPhase, time.Second, nil
		}
	}
	podApplyCompOps := func(
		ops *opsv1alpha1.OpsRequest,
		pod *corev1.Pod,
		pgRes *progressResource) bool {
		expected, ok := containersMap[pgRes.fullComponentName]
		if !ok || expected.pendingOwner {
			return false
		}
		templateName, labeled := instancetemplate.TemplateNameFromLabels(pod.Labels)
		if !labeled {
			if specs[pgRes.fullComponentName].FlatInstanceOrdinal {
				return false
			}
			templateName = appsv1.GetInstanceTemplateName(opsRes.Cluster.Name, pgRes.fullComponentName, pod.Name)
		}
		containers, ok := expected.containersByTemplate[templateName]
		if !ok {
			containers = expected.containersByTemplate[""]
		}
		return u.podImageApplied(pod, containers)
	}
	handleUpgradeProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *opsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, completedCount int32, err error) {
		if spec, ok := specs[pgRes.fullComponentName]; ok {
			pgRes.clusterComponent = spec
			pgRes.componentDef = containersMap[pgRes.fullComponentName].definition
		}
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

func (u upgradeOpsHandler) getUpgradeComponentSpecs(reqCtx intctrlutil.RequestCtx,
	cli client.Client, opsRes *OpsResource) (map[string]*appsv1.ClusterComponentSpec, bool, error) {
	specs := map[string]*appsv1.ClusterComponentSpec{}
	for _, target := range opsRes.OpsRequest.Spec.Upgrade.Components {
		found := false
		for i := range opsRes.Cluster.Spec.ComponentSpecs {
			spec := &opsRes.Cluster.Spec.ComponentSpecs[i]
			if spec.Name == target.ComponentName {
				specs[spec.Name] = spec
				found = true
			}
		}
		for i := range opsRes.Cluster.Spec.Shardings {
			spec := &opsRes.Cluster.Spec.Shardings[i]
			if spec.Name != target.ComponentName {
				continue
			}
			shards, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, spec.Name)
			if err != nil {
				return nil, false, err
			}
			if len(shards) == 0 || len(shards) != int(spec.Shards) {
				return nil, true, nil
			}
			for _, shard := range shards {
				if shard.Generation == 0 || shard.Annotations[constant.KubeBlocksGenerationKey] != strconv.FormatInt(opsRes.Cluster.Generation, 10) {
					return nil, true, nil
				}
				name := shard.Labels[constant.KBAppComponentLabelKey]
				specs[name] = &appsv1.ClusterComponentSpec{
					Name: name, ComponentDef: shard.Spec.CompDef, ServiceVersion: shard.Spec.ServiceVersion,
					Replicas: shard.Spec.Replicas, Instances: shard.Spec.Instances,
					FlatInstanceOrdinal: shard.Spec.FlatInstanceOrdinal,
				}
			}
			found = true
		}
		if !found {
			return nil, false, intctrlutil.NewFatalError(fmt.Sprintf("cannot find component %q in cluster %q", target.ComponentName, opsRes.Cluster.Name))
		}
	}
	return specs, false, nil
}

func (u upgradeOpsHandler) getExpectedContainersMap(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1.Cluster,
	specs map[string]*appsv1.ClusterComponentSpec) (map[string]upgradeExpectedImages, error) {
	containersMap := map[string]upgradeExpectedImages{}
	for name, compSpec := range specs {
		compDef, err := component.GetCompDefByName(reqCtx.Ctx, cli, compSpec.ComponentDef)
		if err != nil {
			return nil, err
		}
		heterogeneous := false
		for _, template := range compSpec.Instances {
			heterogeneous = heterogeneous || template.ServiceVersion != ""
		}
		isShard := true
		for _, spec := range cluster.Spec.ComponentSpecs {
			if spec.Name == name {
				isShard = false
				break
			}
		}
		if !heterogeneous && !isShard {
			if err = component.UpdateCompDefinitionImages4ServiceVersion(reqCtx.Ctx, cli, compDef, compSpec.ServiceVersion); err != nil {
				return nil, err
			}
			containersMap[name] = upgradeExpectedImages{definition: compDef, containersByTemplate: map[string][]corev1.Container{"": compDef.Spec.Runtime.Containers}}
			continue
		}
		containersMap[name] = upgradeExpectedImages{definition: compDef, pendingOwner: true}
		comp := &appsv1.Component{}
		if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: component.FullName(cluster.Name, name)}, comp); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}
			continue
		}
		if comp.Generation == 0 || comp.Annotations[constant.KubeBlocksGenerationKey] != strconv.FormatInt(cluster.Generation, 10) {
			continue
		}
		workloads, err := component.ListOwnedWorkloads(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, name)
		if err != nil {
			return nil, err
		}
		if len(workloads) != 1 || workloads[0].Annotations[constant.KubeBlocksGenerationKey] != strconv.FormatInt(comp.Generation, 10) {
			continue
		}
		its := workloads[0]
		templates := map[string][]corev1.Container{"": its.Spec.Template.Spec.Containers}
		for _, template := range its.Spec.Instances {
			containers := append([]corev1.Container(nil), its.Spec.Template.Spec.Containers...)
			for i := range containers {
				if image := template.Images[containers[i].Name]; image != "" {
					containers[i].Image = image
				}
			}
			templates[template.Name] = containers
		}
		containersMap[name] = upgradeExpectedImages{definition: compDef, containersByTemplate: templates}
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
