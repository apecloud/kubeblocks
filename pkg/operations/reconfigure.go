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
	"encoding/json"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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

// reconfigurePriorValue records the desired-assignment state of one parameter
// key before this ops applied its write. Existed=false means the key was not
// present in the ComponentParameter desired assignments at that time.
type reconfigurePriorValue struct {
	Existed bool    `json:"existed"`
	Value   *string `json:"value,omitempty"`
}

// SaveLastConfiguration snapshots, per component, the prior desired-assignment
// state of every key this ops is about to write, so that the failure path can
// restore (rather than blindly delete) previously accepted desired intent.
// The ops framework invokes this hook before Action, while the
// ComponentParameter still holds the pre-apply state; the snapshot is stored
// as an annotation on the OpsRequest.
func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	ops := opsRes.OpsRequest
	if _, ok := ops.Annotations[constant.ReconfigurePriorParametersAnnotationKey]; ok {
		return nil
	}
	snapshot := map[string]map[string]reconfigurePriorValue{}
	for _, reconfigure := range ops.Spec.Reconfigures {
		compNames, err := r.resolveReconfigureComponents(reqCtx.Ctx, cli, opsRes.Cluster, reconfigure.ComponentName)
		if err != nil {
			return err
		}
		for _, compName := range compNames {
			compParam, err := r.getRunningComponentParameter(reqCtx.Ctx, cli, opsRes.Cluster.Namespace, opsRes.Cluster.Name, compName)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			prior, ok := snapshot[compName]
			if !ok {
				prior = map[string]reconfigurePriorValue{}
				snapshot[compName] = prior
			}
			for _, param := range reconfigure.Parameters {
				pv := reconfigurePriorValue{}
				if err == nil && compParam.Spec.Desired != nil {
					if v, exist := compParam.Spec.Desired.Assignments[param.Key]; exist {
						pv.Existed = true
						pv.Value = v
					}
				}
				prior[param.Key] = pv
			}
		}
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if ops.Annotations == nil {
		ops.Annotations = map[string]string{}
	}
	ops.Annotations[constant.ReconfigurePriorParametersAnnotationKey] = string(data)
	return cli.Update(reqCtx.Ctx, ops)
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
	// The merge failed, so the assignments this ops wrote will never be applied,
	// yet they would stay in the ComponentParameter desired spec and keep failing
	// the projection for every later reconfigure. Withdraw this ops's own writes
	// (and only them) so the failed intent does not outlive the failed ops.
	if err := r.withdrawReconfigureFromParameters(reqCtx, cli, resource); err != nil {
		return "", noRequeueAfter, err
	}
	return opsv1alpha1.OpsFailedPhase, 0, intctrlutil.NewFatalError(fmt.Sprintf("reconfigure failed: %s", msg))
}

// withdrawReconfigureFromParameters restores, in the ComponentParameter
// desired assignments, the pre-ops state of every key this failed ops wrote —
// but only while the current value still equals this ops's write (a newer
// writer wins). Value equality against the ops spec alone cannot prove
// ownership: the same key=value pair may be previously accepted desired intent
// from an earlier successful reconfigure, in which case it must be kept, not
// deleted. Ownership is therefore anchored on the prior-state snapshot taken
// by SaveLastConfiguration; without a snapshot this path leaves the desired
// state untouched. No schema validation is performed here.
func (r *reconfigureAction) withdrawReconfigureFromParameters(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) error {
	raw, ok := resource.OpsRequest.Annotations[constant.ReconfigurePriorParametersAnnotationKey]
	if !ok {
		// no prior-state snapshot (e.g. ops created before this mechanism):
		// do not mutate existing desired state on failure.
		return nil
	}
	snapshot := map[string]map[string]reconfigurePriorValue{}
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return err
	}
	sameValue := func(a, b *string) bool {
		if a == nil || b == nil {
			return a == b
		}
		return *a == *b
	}
	for _, reconfigure := range resource.OpsRequest.Spec.Reconfigures {
		compNames, err := r.resolveReconfigureComponents(reqCtx.Ctx, cli, resource.Cluster, reconfigure.ComponentName)
		if err != nil {
			return err
		}
		for _, compName := range compNames {
			prior, ok := snapshot[compName]
			if !ok {
				continue
			}
			compParam, err := r.getRunningComponentParameter(reqCtx.Ctx, cli, resource.Cluster.Namespace, resource.Cluster.Name, compName)
			if err != nil {
				return client.IgnoreNotFound(err)
			}
			if compParam.Spec.Desired == nil || len(compParam.Spec.Desired.Assignments) == 0 {
				continue
			}
			patch := client.MergeFrom(compParam.DeepCopy())
			changed := false
			for _, param := range reconfigure.Parameters {
				pv, snapshotted := prior[param.Key]
				if !snapshotted {
					continue
				}
				current, exist := compParam.Spec.Desired.Assignments[param.Key]
				if !exist || !sameValue(current, param.Value) {
					// already gone, or a newer writer re-set the key: leave it.
					continue
				}
				if pv.Existed {
					if sameValue(pv.Value, param.Value) {
						// the same key=value was already accepted desired intent
						// before this ops: keep it.
						continue
					}
					compParam.Spec.Desired.Assignments[param.Key] = pv.Value
				} else {
					delete(compParam.Spec.Desired.Assignments, param.Key)
				}
				changed = true
			}
			if !changed {
				continue
			}
			if err := cli.Patch(reqCtx.Ctx, compParam, patch); err != nil {
				return err
			}
		}
	}
	return nil
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
