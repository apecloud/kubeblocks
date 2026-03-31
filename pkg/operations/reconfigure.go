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
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	configcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
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
	opsDeepCopy := resource.OpsRequest.DeepCopy()
	phase, msg, err := r.aggregatePhase(reqCtx, cli, resource)
	if err != nil {
		return "", noRequeueAfter, err
	}
	if phase == opsv1alpha1.OpsRunningPhase {
		return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsRunningPhase)
	}
	if phase == opsv1alpha1.OpsSucceedPhase {
		return syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsSucceedPhase)
	}
	return opsv1alpha1.OpsFailedPhase, 0, intctrlutil.NewFatalError(fmt.Sprintf("reconfigure component parameter failed: %s", msg))
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
	for _, reconfigure := range resource.OpsRequest.Spec.Reconfigures {
		if len(reconfigure.Parameters) == 0 && len(reconfigure.CustomTemplates) == 0 {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "invalid reconfigure request for component %s: no parameters or customTemplates", reconfigure.ComponentName)
		}
		componentNames, err := resolveReconfigureComponents(reqCtx.Ctx, cli, resource.Cluster, reconfigure.ComponentName)
		if err != nil {
			return err
		}
		for _, componentName := range componentNames {
			if err := applyReconfigureToComponentParameter(reqCtx, cli, resource.Cluster, componentName, reconfigure); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *reconfigureAction) aggregatePhase(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, string, error) {
	for _, reconfigure := range resource.OpsRequest.Spec.Reconfigures {
		componentNames, err := resolveReconfigureComponents(reqCtx.Ctx, cli, resource.Cluster, reconfigure.ComponentName)
		if err != nil {
			return "", "", err
		}
		for _, componentName := range componentNames {
			componentParameter, err := getRunningComponentParameter(reqCtx.Ctx, cli, resource.Cluster.Namespace, resource.Cluster.Name, componentName)
			if err != nil {
				return "", "", err
			}
			if componentParameter.Generation != componentParameter.Status.ObservedGeneration {
				return opsv1alpha1.OpsRunningPhase, "", nil
			}
			switch componentParameter.Status.Phase {
			case parametersv1alpha1.CMergeFailedPhase, parametersv1alpha1.CFailedAndPausePhase:
				return opsv1alpha1.OpsFailedPhase, componentParameter.Status.Message, nil
			case parametersv1alpha1.CFinishedPhase:
				continue
			default:
				return opsv1alpha1.OpsRunningPhase, "", nil
			}
		}
	}
	return opsv1alpha1.OpsSucceedPhase, "", nil
}

func applyReconfigureToComponentParameter(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1.Cluster, componentName string, reconfigure opsv1alpha1.Reconfigure) error {
	componentParameter, err := getRunningComponentParameter(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, componentName)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(componentParameter.DeepCopy())
	if componentParameter.Spec.Desired == nil {
		componentParameter.Spec.Desired = &parametersv1alpha1.ParameterValues{}
	}
	if len(reconfigure.Parameters) != 0 {
		if componentParameter.Spec.Desired.Parameters == nil {
			componentParameter.Spec.Desired.Parameters = parametersv1alpha1.ParameterValueMap{}
		}
		for _, param := range reconfigure.Parameters {
			componentParameter.Spec.Desired.Parameters[param.Key] = param.Value
		}
	}
	if len(reconfigure.CustomTemplates) != 0 {
		if componentParameter.Spec.Desired.Templates == nil {
			componentParameter.Spec.Desired.Templates = map[string]parametersv1alpha1.ConfigTemplateExtension{}
		}
		for templateName, templateExtension := range reconfigure.CustomTemplates {
			componentParameter.Spec.Desired.Templates[templateName] = *templateExtension.DeepCopy()
		}
	}
	if err := cli.Patch(reqCtx.Ctx, componentParameter, patch); err != nil {
		return err
	}
	return nil
}

func resolveReconfigureComponents(ctx context.Context, reader client.Reader, cluster *appsv1.Cluster, componentName string) ([]string, error) {
	if compSpec := cluster.Spec.GetComponentByName(componentName); compSpec != nil {
		return []string{compSpec.Name}, nil
	}
	shardingComp := cluster.Spec.GetShardingByName(componentName)
	if shardingComp == nil {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "component not found: %s", componentName)
	}
	components, err := sharding.ListShardingComponents(ctx, reader, cluster, componentName)
	if err != nil {
		return nil, err
	}
	componentNames := make([]string, 0, len(components))
	for _, comp := range components {
		shortName, err := component.ShortName(cluster.Name, comp.Name)
		if err != nil {
			return nil, err
		}
		componentNames = append(componentNames, shortName)
	}
	return componentNames, nil
}

func getRunningComponentParameter(ctx context.Context, cli client.Client, namespace, clusterName, componentName string) (*parametersv1alpha1.ComponentParameter, error) {
	componentParameter := &parametersv1alpha1.ComponentParameter{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      configcore.GenerateComponentConfigurationName(clusterName, componentName),
	}
	if err := cli.Get(ctx, key, componentParameter); err != nil {
		return nil, err
	}
	return componentParameter, nil
}
