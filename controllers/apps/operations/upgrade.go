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

package operations

import (
	"context"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type upgradeOpsHandler struct{}

var _ OpsHandler = upgradeOpsHandler{}

func init() {
	upgradeBehaviour := OpsBehaviour{
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.SpecReconcilingClusterPhase, // appsv1alpha1.VersionUpgradingPhase,
		OpsHandler:        upgradeOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.UpgradeType, upgradeBehaviour)
}

// ActionStartedCondition the started condition when handle the upgrade request.
func (u upgradeOpsHandler) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewHorizontalScalingCondition(opsRequest)
}

// Action modifies Cluster.spec.clusterVersionRef with opsRequest.spec.upgrade.clusterVersionRef
func (u upgradeOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRes.Cluster.Spec.ClusterVersionRef = opsRes.OpsRequest.Spec.Upgrade.ClusterVersionRef
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for upgrade opsRequest.
func (u upgradeOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	return ReconcileActionWithComponentOps(reqCtx, cli, opsRes, "upgrade", handleComponentStatusProgress)
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (u upgradeOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return opsRequest.GetUpgradeComponentNameMap()
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (u upgradeOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compsStatus, err := u.getUpgradeComponentsStatus(reqCtx, cli, opsRes)
	if err != nil {
		return err
	}
	opsRes.OpsRequest.Status.LastConfiguration.ClusterVersionRef = opsRes.Cluster.Spec.ClusterVersionRef
	opsRes.OpsRequest.Status.Components = compsStatus
	return nil
}

// getUpgradeComponentsStatus compares the ClusterVersions before and after upgrade, and get the changed components map.
func (u upgradeOpsHandler) getUpgradeComponentsStatus(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (map[string]appsv1alpha1.OpsRequestComponentStatus, error) {
	lastComponents, err := u.getClusterComponentVersionMap(reqCtx.Ctx, cli,
		opsRes.Cluster.Spec.ClusterVersionRef)
	if err != nil {
		return nil, err
	}
	components, err := u.getClusterComponentVersionMap(reqCtx.Ctx, cli,
		opsRes.OpsRequest.Spec.Upgrade.ClusterVersionRef)
	if err != nil {
		return nil, err
	}
	// get the changed components map
	changedComponentMap := map[string]struct{}{}
	for k, v := range components {
		lastComp := lastComponents[k]
		if !reflect.DeepEqual(v, lastComp) {
			changedComponentMap[k] = struct{}{}
		}
	}
	// get the changed components name map, and record the components infos to OpsRequest.status.
	compStatusMap := map[string]appsv1alpha1.OpsRequestComponentStatus{}
	for _, comp := range opsRes.Cluster.Spec.ComponentSpecs {
		if _, ok := changedComponentMap[comp.ComponentDefRef]; !ok {
			continue
		}
		compStatusMap[comp.Name] = appsv1alpha1.OpsRequestComponentStatus{
			Phase: appsv1alpha1.SpecReconcilingClusterCompPhase,
		}
	}
	return compStatusMap, nil
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
