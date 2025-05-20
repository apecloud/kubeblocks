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

package component

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// componentDeletionTransformer handles component deletion
type componentDeletionTransformer struct{}

var _ graph.Transformer = &componentDeletionTransformer{}

func (t *componentDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if transCtx.Component.GetDeletionTimestamp().IsZero() {
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component

	clusterName, err := component.GetClusterName(comp)
	if err != nil {
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, err.Error())
	}

	// step1: update the component status to deleting
	if comp.Status.Phase != appsv1.DeletingComponentPhase {
		comp.Status.Phase = appsv1.DeletingComponentPhase
		graphCli.Status(dag, comp, transCtx.Component)
		return intctrlutil.NewRequeueError(time.Second*1, "updating component status to deleting")
	}

	// step2: delete the sub-resources
	compName, err := component.ShortName(clusterName, comp.Name)
	if err != nil {
		return err
	}
	ml := constant.GetCompLabels(clusterName, compName)

	compScaleIn, ok := comp.Annotations[constant.ComponentScaleInAnnotationKey]
	if ok && compScaleIn == "true" {
		return t.handleCompDeleteWhenScaleIn(transCtx, graphCli, dag, comp, ml)
	}
	return t.handleCompDeleteWhenClusterDelete(transCtx, graphCli, dag, comp, ml)
}

// handleCompDeleteWhenScaleIn handles the component deletion when scale-in, this scenario will delete all the sub-resources owned by the component by default.
func (t *componentDeletionTransformer) handleCompDeleteWhenScaleIn(transCtx *componentTransformContext, graphCli model.GraphClient,
	dag *graph.DAG, comp *appsv1.Component, matchLabels map[string]string) error {
	return t.deleteCompResources(transCtx, graphCli, dag, comp, matchLabels, kindsForCompWipeOut())
}

// handleCompDeleteWhenClusterDelete handles the component deletion when the cluster is being deleted.
func (t *componentDeletionTransformer) handleCompDeleteWhenClusterDelete(transCtx *componentTransformContext, graphCli model.GraphClient,
	dag *graph.DAG, comp *appsv1.Component, matchLabels map[string]string) error {
	var kinds []client.ObjectList
	switch comp.Spec.TerminationPolicy {
	case appsv1.Delete:
		kinds = kindsForCompDelete()
	case appsv1.WipeOut:
		kinds = kindsForCompWipeOut()
	}
	return t.deleteCompResources(transCtx, graphCli, dag, comp, matchLabels, kinds)
}

func (t *componentDeletionTransformer) deleteCompResources(transCtx *componentTransformContext, graphCli model.GraphClient,
	dag *graph.DAG, comp *appsv1.Component, matchLabels map[string]string, kinds []client.ObjectList) error {

	// firstly, delete the workloads owned by the component
	workloads, err := model.ReadCacheSnapshot(transCtx, comp, matchLabels, compOwnedWorkloadKinds()...)
	if err != nil {
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, err.Error())
	}
	if len(workloads) > 0 {
		for _, workload := range workloads {
			graphCli.Delete(dag, workload)
		}
		// wait for the workloads to be deleted to trigger the next reconcile
		transCtx.Logger.Info(fmt.Sprintf("wait for the workloads to be deleted: %v", workloads))
		return graph.ErrPrematureStop
	}

	// secondly, delete the other sub-resources owned by the component
	snapshot, err1 := model.ReadCacheSnapshot(transCtx, comp, matchLabels, kinds...)
	if err1 != nil {
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, err1.Error())
	}
	if len(snapshot) > 0 {
		// delete the sub-resources owned by the component before deleting the component
		for _, object := range snapshot {
			if appsutil.IsOwnedByInstanceSet(object) {
				continue
			}

			switch object.(type) {
			case *corev1.ServiceAccount, *rbacv1.Role, *rbacv1.RoleBinding:
				if err := handleRBACResourceDeletion(object, transCtx, comp, graphCli, dag, matchLabels); err != nil {
					return fmt.Errorf("handle rbac deletion failed: %w", err)
				}
			default:
				graphCli.Delete(dag, object)
			}
		}
		graphCli.Status(dag, comp, transCtx.Component)
		return intctrlutil.NewRequeueError(time.Second*1, "not all component sub-resources deleted")
	} else {
		if err = notifyDependents4CompDeletion(transCtx, dag); err != nil {
			return intctrlutil.NewRequeueError(appsutil.RequeueDuration, fmt.Sprintf("notify dependent components error: %s", err.Error()))
		}
		graphCli.Delete(dag, comp)
	}

	// release the allocated host-network ports for the component
	pm := intctrlutil.GetPortManager()
	if err = pm.ReleaseByPrefix(comp.Name); err != nil {
		return intctrlutil.NewRequeueError(time.Second*1, fmt.Sprintf("release host ports for component %s error: %s", comp.Name, err.Error()))
	}

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}

func handleRBACResourceDeletion(obj client.Object, transCtx *componentTransformContext, comp *appsv1.Component,
	graphCli model.GraphClient, dag *graph.DAG, matchLabels map[string]string) (err error) {
	// orphan a rbac resource so that it can be adopted by another component
	// this means these resources won't get automatically deleted
	if err := controllerutil.RemoveControllerReference(comp, obj, model.GetScheme()); err != nil {
		return err
	}
	for k := range matchLabels {
		delete(obj.GetLabels(), k)
	}

	graphCli.Update(dag, nil, obj)
	gvk, err := apiutil.GVKForObject(obj, model.GetScheme())
	if err != nil {
		return err
	}
	transCtx.Logger.V(1).Info("rbac resources orphaned",
		"component", comp.Name, "name", klog.KObj(obj).String(), "gvk", gvk)
	return nil
}

func notifyDependents4CompDeletion(transCtx *componentTransformContext, dag *graph.DAG) error {
	transformer := &componentNotifierTransformer{}
	return transformer.Transform(transCtx, dag)
}

func compOwnedWorkloadKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.InstanceSetList{},
	}
}

func compOwnedKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.InstanceSetList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&corev1.ServiceAccountList{},
		&rbacv1.RoleList{},
		&rbacv1.RoleBindingList{},
	}
}

func kindsForCompDelete() []client.ObjectList {
	return compOwnedKinds()
}

func kindsForCompWipeOut() []client.ObjectList {
	return kindsForCompDelete()
}
