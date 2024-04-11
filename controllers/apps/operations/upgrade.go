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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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

func (u upgradeOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return fmt.Errorf("not implemented")
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for upgrade opsRequest.
func (u upgradeOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	return reconcileActionWithComponentOps(reqCtx, cli, opsRes, "upgrade", nil, handleComponentStatusProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (u upgradeOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compsStatus, err := u.getUpgradeComponentsStatus(reqCtx, cli, opsRes)
	if err != nil {
		return err
	}
	opsRes.OpsRequest.Status.Components = compsStatus
	return nil
}

func (u upgradeOpsHandler) getUpgradeComponentsStatus(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (map[string]appsv1alpha1.OpsRequestComponentStatus, error) {
	// get the changed components map
	changedComponentMap := map[string]struct{}{}

	// TODO: impl

	// get the changed components name map, and record the components infos to OpsRequest.status.
	compStatusMap := map[string]appsv1alpha1.OpsRequestComponentStatus{}
	for _, comp := range opsRes.Cluster.Spec.ComponentSpecs {
		if _, ok := changedComponentMap[comp.ComponentDefRef]; !ok {
			continue
		}
		compStatusMap[comp.Name] = appsv1alpha1.OpsRequestComponentStatus{
			Phase: appsv1alpha1.UpdatingClusterCompPhase,
		}
	}
	return compStatusMap, nil
}
