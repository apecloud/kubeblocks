/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package components

import (
	"context"
	"strconv"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensus"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replication"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// componentContext wrapper for handling component status procedure context parameters.
type componentContext struct {
	reqCtx        intctrlutil.RequestCtx
	cli           client.Client
	recorder      record.EventRecorder
	component     types.Component
	obj           client.Object
	componentSpec *appsv1alpha1.ClusterComponentSpec
}

// NewComponentByType creates a component object.
func NewComponentByType(
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	component *appsv1alpha1.ClusterComponentSpec,
	componentDef appsv1alpha1.ClusterComponentDefinition,
) (types.Component, error) {
	if err := util.ComponentRuntimeReqArgsCheck(cli, cluster, component); err != nil {
		return nil, err
	}
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return consensus.NewConsensusComponent(cli, cluster, component, componentDef)
	case appsv1alpha1.Replication:
		return replication.NewReplicationComponent(cli, cluster, component, componentDef)
	case appsv1alpha1.Stateful:
		return stateful.NewStatefulComponent(cli, cluster, component, componentDef)
	case appsv1alpha1.Stateless:
		return stateless.NewStatelessComponent(cli, cluster, component, componentDef)
	default:
		panic("unknown workload type")
	}
}

// newComponentContext creates a componentContext object.
func newComponentContext(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	recorder record.EventRecorder,
	component types.Component,
	obj client.Object,
	componentSpec *appsv1alpha1.ClusterComponentSpec) componentContext {

	return componentContext{
		reqCtx:        reqCtx,
		cli:           cli,
		recorder:      recorder,
		component:     component,
		obj:           obj,
		componentSpec: componentSpec,
	}
}

// updateComponentStatusInClusterStatus updates cluster.Status.Components if the component status changed
func updateComponentStatusInClusterStatus(
	compCtx componentContext,
	cluster *appsv1alpha1.Cluster) (time.Duration, error) {
	componentStatusSynchronizer, err := newClusterStatusSynchronizer(compCtx.reqCtx.Ctx, compCtx.cli, cluster,
		compCtx.componentSpec, compCtx.component)
	if err != nil {
		return 0, err
	}
	if componentStatusSynchronizer == nil {
		return 0, nil
	}

	wait, err := componentStatusSynchronizer.Update(compCtx.reqCtx.Ctx, compCtx.obj, &compCtx.reqCtx.Log,
		compCtx.recorder)
	if err != nil {
		return 0, err
	}

	var requeueAfter time.Duration
	if wait {
		requeueAfter = time.Minute
	}
	return requeueAfter, opsutil.MarkRunningOpsRequestAnnotation(compCtx.reqCtx.Ctx, compCtx.cli, cluster)
}

func workloadCompClusterReconcile(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	operand client.Object,
	processor func(*appsv1alpha1.Cluster, *appsv1alpha1.ClusterComponentSpec, types.Component) (ctrl.Result, error),
) (ctrl.Result, error) {
	var err error
	var cluster *appsv1alpha1.Cluster

	if cluster, err = util.GetClusterByObject(reqCtx.Ctx, cli, operand); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	} else if cluster == nil {
		return intctrlutil.Reconciled()
	}

	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// create a component object
	componentName := operand.GetLabels()[constant.KBAppComponentLabelKey]
	componentSpec := cluster.Spec.GetComponentByName(componentName)
	if componentSpec == nil {
		return intctrlutil.Reconciled()
	}
	componentDef := clusterDef.GetComponentDefByName(componentSpec.ComponentDefRef)
	component, err := NewComponentByType(cli, cluster, componentSpec, *componentDef)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return processor(cluster, componentSpec, component)
}

// patchWorkloadCustomLabel patches workload custom labels.
func patchWorkloadCustomLabel(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec) error {
	if cluster == nil || componentSpec == nil {
		return nil
	}
	compDef, err := util.GetComponentDefByCluster(ctx, cli, *cluster, componentSpec.ComponentDefRef)
	if err != nil {
		return err
	}
	for _, customLabelSpec := range compDef.CustomLabelSpecs {
		// TODO if the customLabelSpec.Resources is empty, we should add the label to the workload resources under the component.
		for _, resource := range customLabelSpec.Resources {
			gvk, err := util.ParseCustomLabelPattern(resource.GVK)
			if err != nil {
				return err
			}
			// only handle workload kind
			if !slices.Contains(util.GetCustomLabelWorkloadKind(), gvk.Kind) {
				continue
			}
			if err := util.PatchGVRCustomLabels(ctx, cli, cluster, resource, componentSpec.Name, customLabelSpec.Key, customLabelSpec.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func updateComponentInfoToPods(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec) error {
	if cluster == nil || componentSpec == nil {
		return nil
	}
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.GetName(),
		constant.KBAppComponentLabelKey: componentSpec.Name,
	}
	podList := corev1.PodList{}
	if err := cli.List(ctx, &podList, ml); err != nil {
		return err
	}
	replicasStr := strconv.Itoa(int(componentSpec.Replicas))
	for _, pod := range podList.Items {
		if pod.Annotations != nil &&
			pod.Annotations[constant.ComponentReplicasAnnotationKey] == replicasStr {
			continue
		}
		patch := client.MergeFrom(pod.DeepCopy())
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[constant.ComponentReplicasAnnotationKey] = replicasStr
		if err := cli.Patch(ctx, &pod, patch); err != nil {
			return err
		}
	}
	return nil
}
