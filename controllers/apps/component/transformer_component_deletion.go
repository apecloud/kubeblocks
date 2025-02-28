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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
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
		return nil
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
			skipDeletion, err := handleRBACResourceDeletion(object, transCtx, comp, graphCli, dag)
			if err != nil {
				return fmt.Errorf("handle rbac deletion failed: %w", err)
			}
			if skipDeletion {
				continue
			}
			graphCli.Delete(dag, object)
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
	graphCli model.GraphClient, dag *graph.DAG) (skipDeletion bool, err error) {
	switch v := obj.(type) {
	case *corev1.ServiceAccount, *rbacv1.Role, *rbacv1.RoleBinding:
		// list other components that reference the same componentdefinition
		transCtx.Logger.V(1).Info("handling rbac resources deletion",
			"comp", comp.Name, "name", klog.KObj(v).String())
		compDefName := comp.Spec.CompDef
		compList := &appsv1.ComponentList{}
		if err := transCtx.Client.List(transCtx.Context, compList, client.InNamespace(comp.Namespace),
			client.MatchingLabels{constant.ComponentDefinitionLabelKey: compDefName}); err != nil {
			return false, err
		}
		// if any, transfer ownership to any other component
		for _, otherComp := range compList.Items {
			// skip current component
			if otherComp.Name == comp.Name {
				continue
			}
			// skip deleting component
			if !otherComp.DeletionTimestamp.IsZero() {
				continue
			}

			if err := controllerutil.RemoveControllerReference(comp, v, model.GetScheme()); err != nil {
				return false, err
			}
			if err := controllerutil.SetControllerReference(&otherComp, v, model.GetScheme()); err != nil {
				return false, err
			}
			// component controller selects a comp's subresource by labels, so change them too
			clusterName, err := component.GetClusterName(&otherComp)
			if err != nil {
				return false, err
			}
			compShortName, err := component.ShortName(clusterName, otherComp.Name)
			if err != nil {
				return false, err
			}
			newLabels := constant.GetCompLabels(clusterName, compShortName)
			for k, val := range newLabels {
				v.GetLabels()[k] = val
			}
			graphCli.Update(dag, nil, v)
			gvk, err := apiutil.GVKForObject(v, model.GetScheme())
			if err != nil {
				return false, err
			}
			transCtx.Logger.V(1).Info("rbac resources owner transferred, skip deletion",
				"fromComp", comp.Name, "toComp", otherComp.Name, "name", klog.KObj(v).String(), "gvk", gvk)
			return true, nil
		}
		return false, nil
	default:
		return false, nil
	}
}

func notifyDependents4CompDeletion(transCtx *componentTransformContext, dag *graph.DAG) error {
	var (
		ctx  = transCtx.Context
		cli  = transCtx.Client
		comp = transCtx.Component
	)

	compDef := &appsv1.ComponentDefinition{}
	if err := transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: comp.Spec.CompDef}, compDef); err != nil {
		return errors.Wrap(err, "failed to get the component definition for dependents notifier at deletion")
	}

	synthesizedComp, err := component.BuildSynthesizedComponent(ctx, cli, compDef, comp)
	if err != nil {
		return errors.Wrap(err, "failed to build synthesized component for dependents notifier at deletion")
	}

	bak := transCtx.SynthesizeComponent
	defer func() {
		transCtx.SynthesizeComponent = bak
	}()

	transCtx.SynthesizeComponent = synthesizedComp
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
		&corev1.PersistentVolumeClaimList{},
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
