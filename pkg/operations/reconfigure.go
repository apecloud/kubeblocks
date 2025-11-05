/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
)

type reconfigureAction struct {
}

func init() {
	reAction := reconfigureAction{}
	opsManager := GetOpsManager()
	reconfigureBehaviour := OpsBehaviour{
		// REVIEW: can do opsrequest if not running?
		FromClusterPhases: appsv1.GetReconfiguringRunningPhases(),
		// TODO: add cluster reconcile Reconfiguring phase.
		ToClusterPhase: appsv1.UpdatingClusterPhase,
		QueueByCluster: true,
		OpsHandler:     &reAction,
	}
	opsManager.RegisterOps(opsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

var noRequeueAfter time.Duration = 0

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewReconfigureCondition(opsRes.OpsRequest), nil
}

func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r *reconfigureAction) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {

	var parameter = parametersv1alpha1.Parameter{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(resource.OpsRequest), &parameter); err != nil {
		return "", noRequeueAfter, err
	}

	opsDeepCopy := resource.OpsRequest.DeepCopy()
	if !parameters.IsParameterFinished(parameter.Status.Phase) {
		return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsRunningPhase)
	}

	if parameter.Status.Phase == parametersv1alpha1.CFinishedPhase {
		return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsSucceedPhase)
	}

	return opsv1alpha1.OpsFailedPhase, 0, intctrlutil.NewFatalError(fmt.Sprintf("reconfigure parameter failed: %s", parameter.Status.Message))
}

func syncReconfigureForOps(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource, opsDeepCopy *opsv1alpha1.OpsRequest, phase opsv1alpha1.OpsPhase) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if err := PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, resource, opsDeepCopy, phase); err != nil {
		return "", noRequeueAfter, err
	}
	return phase, noRequeueAfter, nil
}

func (r *reconfigureAction) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (err error) {
	if !intctrlutil.ObjectAPIVersionSupported(resource.Cluster) {
		return intctrlutil.NewFatalError(fmt.Sprintf(`api version "%s" is not supported, you can upgrade the cluster to v1 version`, resource.Cluster.APIVersion))
	}

	if len(resource.OpsRequest.Spec.Reconfigures) == 0 {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, `invalid reconfigure request: %s`, resource.OpsRequest.GetName())
	}

	parameter := buildReconfigureParameter(resource.OpsRequest)
	if err = intctrlutil.SetControllerReference(resource.OpsRequest, parameter); err != nil {
		return err
	}

	var checkObj = parametersv1alpha1.Parameter{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(parameter), &checkObj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return cli.Create(reqCtx.Ctx, parameter)
		}
		return err
	}
	return nil
}

func buildReconfigureParameter(ops *opsv1alpha1.OpsRequest) *parametersv1alpha1.Parameter {
	paramBuilder := builder.NewParameterBuilder(ops.Namespace, ops.GetName()).
		AddLabels(constant.AppInstanceLabelKey, ops.Spec.ClusterName).
		AddLabels(constant.OpsRequestNameLabelKey, ops.Name).
		ClusterRef(ops.Spec.ClusterName)
	for _, reconfigure := range ops.Spec.Reconfigures {
		if len(reconfigure.Parameters) != 0 {
			paramBuilder.SetComponentParameters(reconfigure.ComponentName, parameters.TransformComponentParameters(reconfigure.Parameters))
		}
	}
	return paramBuilder.GetObject()
}
