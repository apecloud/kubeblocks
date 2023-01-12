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
	"context"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type upgradeOpsHandler struct{}

var _ OpsHandler = upgradeOpsHandler{}

func init() {
	upgradeBehaviour := OpsBehaviour{
		FromClusterPhases: []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase},
		ToClusterPhase:    dbaasv1alpha1.UpdatingPhase,
		OpsHandler:        upgradeOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.UpgradeType, upgradeBehaviour)
}

// ActionStartedCondition the started condition when handle the upgrade request.
func (u upgradeOpsHandler) ActionStartedCondition(opsRequest *dbaasv1alpha1.OpsRequest) *metav1.Condition {
	return dbaasv1alpha1.NewHorizontalScalingCondition(opsRequest)
}

// Action modifies Cluster.spec.clusterVersionRef with opsRequest.spec.upgrade.clusterVersionRef
func (u upgradeOpsHandler) Action(opsRes *OpsResource) error {
	opsRes.Cluster.Spec.ClusterVersionRef = opsRes.OpsRequest.Spec.Upgrade.ClusterVersionRef
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for upgrade opsRequest.
func (u upgradeOpsHandler) ReconcileAction(opsRes *OpsResource) (dbaasv1alpha1.Phase, time.Duration, error) {
	return ReconcileActionWithComponentOps(opsRes, "upgrade", handleComponentStatusProgress)
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (u upgradeOpsHandler) GetRealAffectedComponentMap(opsRequest *dbaasv1alpha1.OpsRequest) realAffectedComponentMap {
	return opsRequest.GetUpgradeComponentNameMap()
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (u upgradeOpsHandler) SaveLastConfiguration(opsRes *OpsResource) error {
	statusComponents, err := u.getUpgradeStatusComponents(opsRes)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(opsRes.OpsRequest.DeepCopy())
	opsRes.OpsRequest.Status.LastConfiguration = dbaasv1alpha1.LastConfiguration{
		ClusterVersionRef: opsRes.Cluster.Spec.ClusterVersionRef,
	}
	opsRes.OpsRequest.Status.Components = statusComponents
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.OpsRequest, patch)
}

// getUpgradeStatusComponents compares the ClusterVersions before and after upgrade, and get the changed components map.
func (u upgradeOpsHandler) getUpgradeStatusComponents(opsRes *OpsResource) (map[string]dbaasv1alpha1.OpsRequestStatusComponent, error) {
	lastClusterVersionCompMap, err := u.getClusterVersionComponentMap(opsRes.Ctx, opsRes.Client,
		opsRes.Cluster.Spec.ClusterVersionRef)
	if err != nil {
		return nil, err
	}
	clusterVersionCompMap, err := u.getClusterVersionComponentMap(opsRes.Ctx, opsRes.Client,
		opsRes.OpsRequest.Spec.Upgrade.ClusterVersionRef)
	if err != nil {
		return nil, err
	}
	// get the changed components type map
	changedComponentMap := map[string]struct{}{}
	for k, v := range clusterVersionCompMap {
		lastComp := lastClusterVersionCompMap[k]
		if !reflect.DeepEqual(v, lastComp) {
			changedComponentMap[k] = struct{}{}
		}
	}
	// get the changed components name map, and record the components infos to OpsRequest.status.
	statusComponentMap := map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	for k, v := range opsRes.Cluster.Status.Components {
		if _, ok := changedComponentMap[v.Type]; !ok {
			continue
		}
		statusComponentMap[k] = dbaasv1alpha1.OpsRequestStatusComponent{
			Phase: dbaasv1alpha1.UpdatingPhase,
		}
	}
	return statusComponentMap, nil
}

// getClusterVersionComponentMap gets the ClusterVersion and coverts the component list to map.
func (u upgradeOpsHandler) getClusterVersionComponentMap(ctx context.Context,
	cli client.Client, clusterVersionName string) (map[string]dbaasv1alpha1.ClusterVersionComponent, error) {
	clusterVersion := &dbaasv1alpha1.ClusterVersion{}
	if err := cli.Get(ctx, client.ObjectKey{Name: clusterVersionName}, clusterVersion); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	clusterVersionComponentMap := map[string]dbaasv1alpha1.ClusterVersionComponent{}
	for _, v := range clusterVersion.Spec.Components {
		clusterVersionComponentMap[v.Type] = v
	}
	return clusterVersionComponentMap, nil
}
