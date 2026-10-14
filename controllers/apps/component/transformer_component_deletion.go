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

package component

import (
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
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

	// Failed prepareData may leave claims before an InstanceSet can adopt them.
	pvcs, restores, err := failedRestorePVCs(transCtx, comp, matchLabels)
	if err != nil {
		return err
	}
	if len(pvcs) > 0 {
		waiting, err := stopFailedRestoreJobs(transCtx, graphCli, dag, comp.Namespace, restores, pvcs)
		if err != nil {
			return err
		}
		if waiting {
			return intctrlutil.NewRequeueError(time.Second, "waiting for failed restore Jobs and PVC references to disappear")
		}
		for _, pvc := range pvcs {
			graphCli.Delete(dag, pvc)
		}
		return intctrlutil.NewRequeueError(time.Second, "not all failed restore PVCs deleted")
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

func failedRestorePVCs(transCtx *componentTransformContext, comp *appsv1.Component, matchLabels map[string]string) ([]*corev1.PersistentVolumeClaim, []*dpv1alpha1.Restore, error) {
	if comp.UID == "" || (comp.Spec.TerminationPolicy != appsv1.Delete && comp.Spec.TerminationPolicy != appsv1.WipeOut) {
		return nil, nil, nil
	}
	if policy := comp.Spec.PersistentVolumeClaimRetentionPolicy; policy != nil {
		retention := policy.WhenDeleted
		if retention != "" && retention != appsv1.DeletePersistentVolumeClaimRetentionPolicyType {
			return nil, nil, nil
		}
	}
	var restores dpv1alpha1.RestoreList
	if err := transCtx.Client.List(transCtx.Context, &restores, client.InNamespace(comp.Namespace), client.MatchingLabels(matchLabels)); err != nil {
		return nil, nil, err
	}
	claims := make(map[string]dpv1alpha1.RestoreVolumeClaim)
	restoreByClaim := make(map[string][]*dpv1alpha1.Restore)
	for i := range restores.Items {
		restore := &restores.Items[i]
		owner := metav1.GetControllerOf(restore)
		cfg := restore.Spec.PrepareDataConfig
		if restore.UID == "" || owner == nil || owner.UID != comp.UID || owner.Name != comp.Name || owner.Kind != appsv1.ComponentKind ||
			restore.Status.Phase != dpv1alpha1.RestorePhaseFailed || cfg == nil || cfg.RestoreVolumeClaimsTemplate == nil {
			continue
		}
		template := cfg.RestoreVolumeClaimsTemplate
		for _, claim := range template.Templates {
			if claim.Annotations[constant.RestorePVCComponentUIDAnnotationKey] != string(comp.UID) {
				continue
			}
			for i := int32(0); i < template.Replicas; i++ {
				name := fmt.Sprintf("%s-%d", claim.Name, template.StartingIndex+i)
				claims[name] = claim
				restoreByClaim[name] = append(restoreByClaim[name], restore)
			}
		}
	}
	var pvcs []*corev1.PersistentVolumeClaim
	selectedRestores := make(map[types.UID]*dpv1alpha1.Restore)
	for name, claim := range claims {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := transCtx.Client.Get(transCtx.Context, client.ObjectKey{Namespace: comp.Namespace, Name: name}, pvc); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, nil, err
		}
		if pvc.UID == "" || len(pvc.OwnerReferences) > 0 || pvc.Annotations[constant.RestorePVCComponentUIDAnnotationKey] != string(comp.UID) ||
			!labels.SelectorFromSet(matchLabels).Matches(labels.Set(pvc.Labels)) || !labels.SelectorFromSet(claim.Labels).Matches(labels.Set(pvc.Labels)) {
			continue
		}
		pvcs = append(pvcs, pvc)
		for _, restore := range restoreByClaim[name] {
			selectedRestores[restore.UID] = restore
		}
	}
	var owners []*dpv1alpha1.Restore
	for _, restore := range selectedRestores {
		owners = append(owners, restore)
	}
	return pvcs, owners, nil
}

// A Failed Restore can still have active Jobs after a fatal revalidation error.
func stopFailedRestoreJobs(transCtx *componentTransformContext, graphCli model.GraphClient, dag *graph.DAG,
	namespace string, restores []*dpv1alpha1.Restore, pvcs []*corev1.PersistentVolumeClaim) (bool, error) {
	owners := make(map[types.UID]*dpv1alpha1.Restore)
	for _, restore := range restores {
		owners[restore.UID] = restore
	}
	claims := make(map[string]bool)
	for _, pvc := range pvcs {
		claims[pvc.Name] = true
	}
	var jobs batchv1.JobList
	if err := transCtx.Client.List(transCtx.Context, &jobs, client.InNamespace(namespace)); err != nil {
		return false, err
	}
	waiting := false
	for i := range jobs.Items {
		job := &jobs.Items[i]
		if !restoreClaimsReferenced(job.Spec.Template.Spec.Volumes, claims) {
			continue
		}
		waiting = true
		owner := metav1.GetControllerOf(job)
		if owner == nil || job.UID == "" {
			continue
		}
		restore := owners[owner.UID]
		if restore == nil || owner.Name != restore.Name || owner.Kind != "Restore" {
			continue
		}
		if controllerutil.ContainsFinalizer(job, dptypes.DataProtectionFinalizerName) {
			updated := job.DeepCopy()
			controllerutil.RemoveFinalizer(updated, dptypes.DataProtectionFinalizerName)
			// Update preserves UID/RV checks; observe finalizer removal before foreground deletion.
			graphCli.Update(dag, job, updated)
			continue
		}
		// Delete currently drops propagation options; Do retains foreground deletion in the graph.
		graphCli.Do(dag, nil, job, model.ActionDeletePtr(), nil, model.WithPropagationPolicy(client.PropagationPolicy(metav1.DeletePropagationForeground)))
		waiting = true
	}
	var pods corev1.PodList
	if err := transCtx.Client.List(transCtx.Context, &pods, client.InNamespace(namespace)); err != nil {
		return false, err
	}
	for _, pod := range pods.Items {
		// Scheduled terminal Pods can still hold PVC protection. Never delete an unrelated reference.
		if restoreClaimsReferenced(pod.Spec.Volumes, claims) {
			waiting = true
		}
	}
	return waiting, nil
}

func restoreClaimsReferenced(volumes []corev1.Volume, claims map[string]bool) bool {
	for _, volume := range volumes {
		if volume.PersistentVolumeClaim != nil && claims[volume.PersistentVolumeClaim.ClaimName] {
			return true
		}
	}
	return false
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
