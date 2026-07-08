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
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
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
		return r.syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsRunningPhase)
	}
	if phase == opsv1alpha1.OpsSucceedPhase {
		return r.syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsSucceedPhase)
	}
	return opsv1alpha1.OpsFailedPhase, 0, intctrlutil.NewFatalError(fmt.Sprintf("reconfigure failed: %s", msg))
}

func (r *reconfigureAction) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (err error) {
	if len(resource.OpsRequest.Spec.Reconfigures) == 0 {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, `invalid reconfigure request: %s`, resource.OpsRequest.GetName())
	}
	for _, reconfigure := range resource.OpsRequest.Spec.Reconfigures {
		if len(reconfigure.Parameters) == 0 {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "invalid reconfigure request for component %s: no parameters", reconfigure.ComponentName)
		}
		compNames, err := r.resolveReconfigureComponents(reqCtx.Ctx, cli, resource.Cluster, reconfigure.ComponentName)
		if err != nil {
			return err
		}
		for _, compName := range compNames {
			if err := r.applyReconfigureToParameters(reqCtx, cli, resource.Cluster, compName, reconfigure); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *reconfigureAction) syncReconfigureForOps(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource, opsDeepCopy *opsv1alpha1.OpsRequest, phase opsv1alpha1.OpsPhase) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if err := PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, resource, opsDeepCopy, phase); err != nil {
		return "", noRequeueAfter, err
	}
	return phase, noRequeueAfter, nil
}

func (r *reconfigureAction) aggregatePhase(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, string, error) {
	for _, reconfigure := range resource.OpsRequest.Spec.Reconfigures {
		compNames, err := r.resolveReconfigureComponents(reqCtx.Ctx, cli, resource.Cluster, reconfigure.ComponentName)
		if err != nil {
			return "", "", err
		}
		for _, compName := range compNames {
			compParam, err := r.getRunningComponentParameter(reqCtx.Ctx, cli, resource.Cluster.Namespace, resource.Cluster.Name, compName)
			if err != nil {
				return "", "", err
			}
			if compParam.Generation != compParam.Status.ObservedGeneration {
				return opsv1alpha1.OpsRunningPhase, "", nil
			}
			if msg, ok := r.hasPermanentReconfigureFailure(reqCtx.Ctx, cli, resource, compParam, compName); ok {
				if err := r.withdrawReconfigureDesiredFromComponents(reqCtx, cli, resource, compNames, reconfigure); err != nil {
					return "", "", err
				}
				return opsv1alpha1.OpsFailedPhase, msg, nil
			}
			switch compParam.Status.Phase {
			case parametersv1alpha1.CMergeFailedPhase, parametersv1alpha1.CFailedAndPausePhase:
				return opsv1alpha1.OpsFailedPhase, compParam.Status.Message, nil
			case parametersv1alpha1.CFinishedPhase:
				continue
			default:
				return opsv1alpha1.OpsRunningPhase, "", nil
			}
		}
	}
	return opsv1alpha1.OpsSucceedPhase, "", nil
}

func (r *reconfigureAction) hasPermanentReconfigureFailure(ctx context.Context, reader client.Reader,
	resource *OpsResource, compParam *parametersv1alpha1.ComponentParameter, compName string) (string, bool) {
	for i := range compParam.Status.ConfigurationItemStatus {
		itemStatus := &compParam.Status.ConfigurationItemStatus[i]
		targetConfigHash, ok := getCurrentConfigHash(ctx, reader, compParam.GetNamespace(), resource.Cluster, compName, itemStatus.Name)
		if !ok || !isConsumablePermanentReconfigureFailure(resource.OpsRequest, compParam, itemStatus, targetConfigHash) {
			continue
		}
		return renderSafeReconfigureFailureMessage(itemStatus.Name, itemStatus.ReconcileDetail.Reason), true
	}
	return "", false
}

func isConsumablePermanentReconfigureFailure(ops *opsv1alpha1.OpsRequest, compParam *parametersv1alpha1.ComponentParameter,
	itemStatus *parametersv1alpha1.ConfigTemplateItemDetailStatus, targetConfigHash string) bool {
	if ops == nil || compParam == nil || itemStatus == nil || itemStatus.ReconcileDetail == nil {
		return false
	}
	detail := itemStatus.ReconcileDetail
	if detail.OperationUID == "" ||
		detail.ConfigName == "" ||
		detail.TargetConfigHash == "" ||
		detail.ComponentParameterGeneration == 0 {
		return false
	}
	if detail.OperationUID != string(ops.UID) ||
		detail.ConfigName != itemStatus.Name ||
		detail.TargetConfigHash != targetConfigHash ||
		detail.ComponentParameterGeneration != compParam.Generation {
		return false
	}
	return isPermanentFailureReason(detail.FailureClass, detail.Reason)
}

func getCurrentConfigHash(ctx context.Context, reader client.Reader, namespace string, cluster *appsv1.Cluster, compName, configName string) (string, bool) {
	if cluster == nil {
		return "", false
	}
	comp := cluster.Spec.GetComponentByName(compName)
	if comp != nil {
		return configHashFromConfigs(comp.Configs, configName)
	}
	componentObj := &appsv1.Component{}
	if err := reader.Get(ctx, client.ObjectKey{Namespace: namespace, Name: component.FullName(cluster.Name, compName)}, componentObj); err != nil {
		return "", false
	}
	return configHashFromConfigs(componentObj.Spec.Configs, configName)
}

func configHashFromConfigs(configs []appsv1.ClusterComponentConfig, configName string) (string, bool) {
	for i := range configs {
		config := &configs[i]
		if config.Name == nil || *config.Name != configName || config.ConfigHash == nil || *config.ConfigHash == "" {
			continue
		}
		return *config.ConfigHash, true
	}
	return "", false
}

func isPermanentFailureReason(failureClass, reason string) bool {
	if failureClass != parametersv1alpha1.ReconfigureFailureClassPermanent {
		return false
	}
	switch reason {
	case parametersv1alpha1.ReconfigureFailureReasonInvalidParameter,
		parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter,
		parametersv1alpha1.ReconfigureFailureReasonActionRejectedPermanent:
		return true
	default:
		return false
	}
}

func (r *reconfigureAction) withdrawReconfigureDesiredFromComponents(reqCtx intctrlutil.RequestCtx, cli client.Client,
	resource *OpsResource, compNames []string, reconfigure opsv1alpha1.Reconfigure) error {
	for _, compName := range compNames {
		compParam, err := r.getRunningComponentParameter(reqCtx.Ctx, cli, resource.Cluster.Namespace, resource.Cluster.Name, compName)
		if err != nil {
			return err
		}
		if _, ok := r.hasPermanentReconfigureFailure(reqCtx.Ctx, cli, resource, compParam, compName); !ok {
			continue
		}
		if err := withdrawReconfigureDesired(reqCtx, cli, compParam, reconfigure); err != nil {
			return err
		}
	}
	return nil
}

func withdrawReconfigureDesired(reqCtx intctrlutil.RequestCtx, cli client.Client,
	compParam *parametersv1alpha1.ComponentParameter, reconfigure opsv1alpha1.Reconfigure) error {
	if compParam.Spec.Desired == nil || len(compParam.Spec.Desired.Assignments) == 0 || len(reconfigure.Parameters) == 0 {
		return nil
	}
	changed := false
	for _, param := range reconfigure.Parameters {
		current, ok := compParam.Spec.Desired.Assignments[param.Key]
		if !ok || !sameParameterValue(current, param.Value) {
			continue
		}
		delete(compParam.Spec.Desired.Assignments, param.Key)
		changed = true
	}
	if !changed {
		return nil
	}
	return cli.Update(reqCtx.Ctx, compParam)
}

func sameParameterValue(a, b *string) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

func renderSafeReconfigureFailureMessage(configName, reason string) string {
	if reason == "" {
		reason = parametersv1alpha1.ReconfigureFailureReasonUnknown
	}
	if configName == "" {
		return fmt.Sprintf("reconfigure action was rejected with reason %s", reason)
	}
	return fmt.Sprintf("reconfigure action for config %s was rejected with reason %s", configName, reason)
}

func (r *reconfigureAction) applyReconfigureToParameters(reqCtx intctrlutil.RequestCtx, cli client.Client,
	cluster *appsv1.Cluster, compName string, reconfigure opsv1alpha1.Reconfigure) error {
	compParam, err := r.getRunningComponentParameter(reqCtx.Ctx, cli, cluster.Namespace, cluster.Name, compName)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(compParam.DeepCopy())
	if compParam.Spec.Desired == nil {
		compParam.Spec.Desired = &parametersv1alpha1.ParameterInputs{}
	}
	if len(reconfigure.Parameters) != 0 {
		if compParam.Spec.Desired.Assignments == nil {
			compParam.Spec.Desired.Assignments = map[string]*string{}
		}
		for _, param := range reconfigure.Parameters {
			compParam.Spec.Desired.Assignments[param.Key] = param.Value
		}
	}
	return cli.Patch(reqCtx.Ctx, compParam, patch)
}

func (r *reconfigureAction) resolveReconfigureComponents(ctx context.Context, reader client.Reader, cluster *appsv1.Cluster, compName string) ([]string, error) {
	if compSpec := cluster.Spec.GetComponentByName(compName); compSpec != nil {
		return []string{compSpec.Name}, nil
	}
	shardingComp := cluster.Spec.GetShardingByName(compName)
	if shardingComp == nil {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "component not found: %s", compName)
	}
	comps, err := sharding.ListShardingComponents(ctx, reader, cluster, compName)
	if err != nil {
		return nil, err
	}
	compNames := make([]string, 0, len(comps))
	for _, comp := range comps {
		shortName, err := component.ShortName(cluster.Name, comp.Name)
		if err != nil {
			return nil, err
		}
		compNames = append(compNames, shortName)
	}
	return compNames, nil
}

func (r *reconfigureAction) getRunningComponentParameter(ctx context.Context, cli client.Client, namespace, clusterName, compName string) (*parametersv1alpha1.ComponentParameter, error) {
	compParam := &parametersv1alpha1.ComponentParameter{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      parameterscore.GenerateComponentConfigurationName(clusterName, compName),
	}
	if err := cli.Get(ctx, key, compParam); err != nil {
		return nil, err
	}
	return compParam, nil
}
