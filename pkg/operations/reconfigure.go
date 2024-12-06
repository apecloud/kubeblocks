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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewReconfigureCondition(opsRes.OpsRequest), nil
}

func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r *reconfigureAction) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {

	var parameters = &parametersv1alpha1.Parameter{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(resource.OpsRequest), parameters); err != nil {
		return "", 30 * time.Second, err
	}

	opsDeepCopy := resource.OpsRequest.DeepCopy()
	if !intctrlutil.IsParameterFinished(parameters.Status.Phase) {
		return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsRunningPhase)
	}

	if parameters.Status.Phase == parametersv1alpha1.CFinishedPhase {
		return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsSucceedPhase)
	}

	return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsFailedPhase)
}

func syncReconfigureForOps(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource, opsDeepCopy *opsv1alpha1.OpsRequest, phase opsv1alpha1.OpsPhase) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if err := PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, resource, opsDeepCopy, phase); err != nil {
		return "", 30 * time.Second, err
	}
	return phase, 30 * time.Second, nil
}

func (r *reconfigureAction) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (err error) {
	parameter, err := buildReconfigureParameter(resource.OpsRequest)
	if err != nil {
		return err
	}

	var param = &parametersv1alpha1.Parameter{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKeyFromObject(parameter), param); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return cli.Create(reqCtx.Ctx, parameter)
		}
		return err
	}
	return nil
}

func buildReconfigureParameter(ops *opsv1alpha1.OpsRequest) (*parametersv1alpha1.Parameter, error) {
	if len(ops.Spec.Reconfigures) == 0 {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, `invalid reconfigure request: %s`, ops.GetName())
	}

	paramBuilder := builder.NewParameterBuilder(ops.Namespace, ops.GetName()).
		ClusterRef(ops.Spec.ClusterName)
	for _, reconfigure := range ops.Spec.Reconfigures {
		if len(reconfigure.Parameters) != 0 {
			paramBuilder.SetComponentParameters(reconfigure.ComponentName, intctrlutil.TransformComponentParameters(reconfigure.Parameters))
		}
	}

	return paramBuilder.GetObject(), nil
}
