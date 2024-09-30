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

package view

import (
	"context"
	"fmt"

	vsv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/util/podutils"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	workloadsAPI "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/controllers/apps"
	"github.com/apecloud/kubeblocks/controllers/apps/configuration"
	"github.com/apecloud/kubeblocks/controllers/dataprotection"
	"github.com/apecloud/kubeblocks/controllers/workloads"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

type ReconcilerTree interface {
	Run() error
}

type reconcilerFunc func(client.Client, record.EventRecorder) reconcile.Reconciler

var reconcilerFuncMap = map[viewv1.ObjectType]reconcilerFunc{
	objectType(kbappsv1.SchemeGroupVersion.String(), kbappsv1.ClusterKind):             newClusterReconciler,
	objectType(kbappsv1.SchemeGroupVersion.String(), kbappsv1.ComponentKind):           newComponentReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.SecretKind):                newSecretReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.ServiceKind):               newServiceReconciler,
	objectType(workloadsAPI.SchemeGroupVersion.String(), workloadsAPI.Kind):            newInstanceSetReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.ConfigMapKind):             newConfigMapReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.PersistentVolumeClaimKind): newPVCReconciler,
	objectType(rbacv1.SchemeGroupVersion.String(), constant.ClusterRoleBindingKind):    newClusterRoleBindingReconciler,
	objectType(rbacv1.SchemeGroupVersion.String(), constant.RoleBindingKind):           newRoleBindingReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.ServiceAccountKind):        newSAReconciler,
	objectType(batchv1.SchemeGroupVersion.String(), constant.JobKind):                  newJobReconciler,
	objectType(dpv1alpha1.SchemeGroupVersion.String(), types.BackupKind):               newBackupReconciler,
	objectType(dpv1alpha1.SchemeGroupVersion.String(), types.RestoreKind):              newRestoreReconciler,
	objectType(appsv1alpha1.SchemeGroupVersion.String(), constant.ConfigurationKind):   newConfigurationReconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.PodKind):                   newPodReconciler,
	objectType(appsv1.SchemeGroupVersion.String(), constant.StatefulSetKind):           newSTSReconciler,
	objectType(vsv1.SchemeGroupVersion.String(), constant.VolumeSnapshotKind):          newVolumeSnapshotV1Reconciler,
	objectType(vsv1beta1.SchemeGroupVersion.String(), constant.VolumeSnapshotKind):     newVolumeSnapshotV1Beta1Reconciler,
	objectType(corev1.SchemeGroupVersion.String(), constant.PersistentVolumeKind):      newPVReconciler,
}

type reconcilerTree struct {
	ctx         context.Context
	cli         client.Client
	tree        *graph.DAG
	reconcilers map[viewv1.ObjectType]reconcile.Reconciler
}

func (r *reconcilerTree) Run() error {
	return r.tree.WalkTopoOrder(func(v graph.Vertex) error {
		objType, _ := v.(viewv1.ObjectType)
		reconciler := r.reconcilers[objType]
		gvk, err := objectTypeToGVK(&objType)
		if err != nil {
			return err
		}
		objects, err := getObjectsByGVK(r.ctx, r.cli, gvk, nil)
		if err != nil {
			return err
		}
		for _, object := range objects {
			_, err = reconciler.Reconcile(r.ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(object)})
			if err != nil {
				return err
			}
		}
		return nil
	}, func(v1, v2 graph.Vertex) bool {
		t1, _ := v1.(viewv1.ObjectType)
		t2, _ := v2.(viewv1.ObjectType)
		if t1.APIVersion != t2.APIVersion {
			return t1.APIVersion < t2.APIVersion
		}
		return t1.Kind < t2.Kind
	})
}

func newReconcilerTree(ctx context.Context, mClient client.Client, recorder record.EventRecorder, rules []OwnershipRule) (ReconcilerTree, error) {
	dag := graph.NewDAG()
	reconcilers := make(map[viewv1.ObjectType]reconcile.Reconciler)
	for _, rule := range rules {
		dag.AddVertex(rule.Primary)
		reconciler, err := newReconciler(mClient, recorder, rule.Primary)
		if err != nil {
			return nil, err
		}
		reconcilers[rule.Primary] = reconciler
		for _, resource := range rule.OwnedResources {
			dag.AddVertex(resource.Secondary)
			dag.Connect(rule.Primary, resource.Secondary)
			reconciler, err = newReconciler(mClient, recorder, resource.Secondary)
			if err != nil {
				return nil, err
			}
			reconcilers[resource.Secondary] = reconciler
		}
	}
	// DAG should be valid(one and only one root without cycle)
	if err := dag.Validate(); err != nil {
		return nil, err
	}

	return &reconcilerTree{
		ctx:         ctx,
		cli:         mClient,
		tree:        dag,
		reconcilers: reconcilers,
	}, nil
}

func newReconciler(mClient client.Client, recorder record.EventRecorder, objectType viewv1.ObjectType) (reconcile.Reconciler, error) {
	reconcilerF, ok := reconcilerFuncMap[objectType]
	if ok {
		return reconcilerF(mClient, recorder), nil
	}
	return nil, fmt.Errorf("can't initialize a reconciler for GVK: %s/%s", objectType.APIVersion, objectType.Kind)
}

func newClusterReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &apps.ClusterReconciler{
		Client:   cli,
		Scheme:   cli.Scheme(),
		Recorder: recorder,
	}
}

func newComponentReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &apps.ComponentReconciler{
		Client:   cli,
		Scheme:   cli.Scheme(),
		Recorder: recorder,
	}
}

func newConfigurationReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &configuration.ConfigurationReconciler{
		Client:   cli,
		Scheme:   cli.Scheme(),
		Recorder: recorder,
	}
}

func newInstanceSetReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &workloads.InstanceSetReconciler{
		Client:   cli,
		Scheme:   cli.Scheme(),
		Recorder: recorder,
	}
}

type baseReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type doNothingReconciler struct {
	baseReconciler
}

func (r *doNothingReconciler) Reconcile(_ context.Context, _ reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

type pvcReconciler struct {
	baseReconciler
}

func (r *pvcReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, req.NamespacedName, pvc)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// add finalizer
	if len(pvc.Finalizers) == 0 {
		pvc.Finalizers = []string{"kubernetes.io/pvc-protection"}
		if err = r.Update(ctx, pvc); err != nil {
			return reconcile.Result{}, err
		}
	}

	// handle deletion
	if model.IsObjectDeleting(pvc) {
		// delete pv
		objKey := client.ObjectKey{
			Namespace: pvc.Namespace,
			Name:      pvc.Spec.VolumeName,
		}
		pv := &corev1.PersistentVolume{}
		if err = r.Get(ctx, objKey, pv); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			if err = r.Delete(ctx, pv); err != nil {
				return reconcile.Result{}, err
			}
		}
		// remove finalizer
		pvc.Finalizers = nil
		if err = r.Update(ctx, pvc); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, r.Delete(ctx, pvc)
	}

	// phase from "" to Pending
	if pvc.Status.Phase == "" {
		pvc.Status.Phase = corev1.ClaimPending
		err = r.Status().Update(ctx, pvc)
		return reconcile.Result{}, err
	}

	// from Pending to Bound
	if pvc.Status.Phase == corev1.ClaimPending {
		// Provisioning PV for PVC
		pvName := pvc.Name + "-pv"
		pvcRef, err := getObjectReference(pvc, r.Scheme)
		if err != nil {
			return reconcile.Result{}, err
		}
		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvName,
			},
			Spec: corev1.PersistentVolumeSpec{
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: pvc.Spec.Resources.Requests[corev1.ResourceStorage],
				},
				AccessModes:                   pvc.Spec.AccessModes,
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
				StorageClassName:              "mock-storage-class",
				ClaimRef:                      pvcRef,
			},
		}

		if err := controllerutil.SetControllerReference(pvc, pv, r.Scheme); err != nil {
			return reconcile.Result{}, err
		}

		if err := r.Create(ctx, pv); err != nil {
			return reconcile.Result{}, err
		}

		pvc.Spec.VolumeName = pvName
		pvc.Status.Phase = corev1.ClaimBound
		if err = r.Status().Update(ctx, pvc); err != nil {
			return reconcile.Result{}, err
		}
		if err := r.Status().Update(ctx, pvc); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func newPVCReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &pvcReconciler{
		baseReconciler{
			Client:   c,
			Scheme:   c.Scheme(),
			Recorder: recorder,
		},
	}
}

type pvReconciler struct {
	baseReconciler
}

func (r *pvReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pv := &corev1.PersistentVolume{}
	err := r.Get(ctx, req.NamespacedName, pv)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if model.IsObjectDeleting(pv) {
		return reconcile.Result{}, r.Delete(ctx, pv)
	}

	// phase from "" to Available
	if pv.Status.Phase == "" {
		pv.Status.Phase = corev1.VolumeAvailable
		return reconcile.Result{}, r.Status().Update(ctx, pv)
	}

	// phase from Available to Bound
	if pv.Status.Phase == corev1.VolumeAvailable {
		pv.Status.Phase = corev1.VolumeBound
		return reconcile.Result{}, r.Status().Update(ctx, pv)
	}

	// Delete PV if it's released
	if pv.Status.Phase == corev1.VolumeReleased {
		return ctrl.Result{}, r.Delete(ctx, pv)
	}

	return ctrl.Result{}, nil
}

func newPVReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &pvReconciler{
		baseReconciler{
			Client:   c,
			Scheme:   c.Scheme(),
			Recorder: recorder,
		},
	}
}

type volumeSnapshotV1Beta1Reconciler struct {
	baseReconciler
}

func (r *volumeSnapshotV1Beta1Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	snapshot := &vsv1beta1.VolumeSnapshot{}
	err := r.Get(ctx, req.NamespacedName, snapshot)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// snapshot creation
	if snapshot.Status == nil {
		snapshot.Status = &vsv1beta1.VolumeSnapshotStatus{
			ReadyToUse: pointer.Bool(false),
		}
		if err = r.Status().Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}

		// Update snapshot to Ready state
		snapshot.Status.ReadyToUse = pointer.Bool(true)
		snapshot.Status.Error = nil // Reset any errors

		if err = r.Status().Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func newVolumeSnapshotV1Beta1Reconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &volumeSnapshotV1Beta1Reconciler{
		baseReconciler{
			Client:   c,
			Scheme:   c.Scheme(),
			Recorder: recorder,
		},
	}
}

type volumeSnapshotV1Reconciler struct {
	baseReconciler
}

func (r *volumeSnapshotV1Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	snapshot := &vsv1.VolumeSnapshot{}
	err := r.Get(ctx, req.NamespacedName, snapshot)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// snapshot creation
	if snapshot.Status == nil {
		snapshot.Status = &vsv1.VolumeSnapshotStatus{
			ReadyToUse: pointer.Bool(false),
		}
		if err = r.Status().Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
		// Update the snapshot to indicate it's ready
		snapshot.Status.ReadyToUse = pointer.Bool(true)
		snapshot.Status.Error = nil // No errors encountered
		if err = r.Status().Update(ctx, snapshot); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func newVolumeSnapshotV1Reconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &volumeSnapshotV1Reconciler{
		baseReconciler{
			Client:   c,
			Scheme:   c.Scheme(),
			Recorder: recorder,
		},
	}
}

type stsReconciler struct {
	baseReconciler
}

func (r *stsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, req.NamespacedName, sts)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	generatePodName := func(parent string, ordinal int32) string {
		return fmt.Sprintf("%s-%d", parent, ordinal)
	}
	generatePVCName := func(claimTemplateName, podName string) string {
		return fmt.Sprintf("%s-%s", claimTemplateName, podName)
	}

	// handle deletion
	if model.IsObjectDeleting(sts) {
		// delete all pods
		podList := &corev1.PodList{}
		if err = r.List(ctx, podList, client.MatchingLabels(sts.Spec.Template.Labels)); err != nil {
			return ctrl.Result{}, err
		}
		for _, pod := range podList.Items {
			if err = r.Delete(ctx, &pod); err != nil {
				return ctrl.Result{}, err
			}
		}
		// delete sts
		return ctrl.Result{}, r.Delete(ctx, sts)
	}

	// handle creation
	// Create Pods and PVCs for each replica
	for i := int32(0); i < *sts.Spec.Replicas; i++ {
		podName := generatePodName(sts.Name, i)
		template := sts.Spec.Template
		// 1. build pod
		pod := builder.NewPodBuilder(sts.Namespace, podName).
			AddAnnotationsInMap(template.Annotations).
			AddLabelsInMap(template.Labels).
			SetPodSpec(*template.Spec.DeepCopy()).
			GetObject()

		// 2. build pvcs from template
		pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
		pvcNameMap := make(map[string]string)
		for _, claimTemplate := range sts.Spec.VolumeClaimTemplates {
			pvcName := generatePVCName(claimTemplate.Name, pod.GetName())
			pvc := builder.NewPVCBuilder(sts.Namespace, pvcName).
				AddLabelsInMap(template.Labels).
				SetSpec(*claimTemplate.Spec.DeepCopy()).
				GetObject()
			pvcMap[pvcName] = pvc
			pvcNameMap[pvcName] = claimTemplate.Name
		}

		// 3. update pod volumes
		var pvcs []*corev1.PersistentVolumeClaim
		var volumeList []corev1.Volume
		for pvcName, pvc := range pvcMap {
			pvcs = append(pvcs, pvc)
			volume := builder.NewVolumeBuilder(pvcNameMap[pvcName]).
				SetVolumeSource(corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				}).GetObject()
			volumeList = append(volumeList, *volume)
		}
		intctrlutil.MergeList(&volumeList, &pod.Spec.Volumes, func(item corev1.Volume) func(corev1.Volume) bool {
			return func(v corev1.Volume) bool {
				return v.Name == item.Name
			}
		})

		if err = controllerutil.SetControllerReference(sts, pod, r.Scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Create Pod
		if err = r.Create(ctx, pod); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		// Create PVC
		for _, pvc := range pvcs {
			if err = r.Create(ctx, pvc); err != nil && !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, err
			}
		}
	}

	// Update the status of the StatefulSet
	podList := &corev1.PodList{}
	if err = r.List(ctx, podList, client.MatchingLabels(sts.Spec.Template.Labels)); err != nil {
		return ctrl.Result{}, err
	}
	isCreated := func(pod *corev1.Pod) bool {
		return pod.Status.Phase != ""
	}
	isRunningAndReady := func(pod *corev1.Pod) bool {
		return pod.Status.Phase == corev1.PodRunning && podutils.IsPodReady(pod)
	}
	isTerminating := func(pod *corev1.Pod) bool {
		return pod.DeletionTimestamp != nil
	}
	isRunningAndAvailable := func(pod *corev1.Pod, minReadySeconds int32) bool {
		return podutils.IsPodAvailable(pod, minReadySeconds, metav1.Now())
	}
	replicas := int32(0)
	currentReplicas, updatedReplicas := int32(0), int32(0)
	readyReplicas, availableReplicas := int32(0), int32(0)
	for i := range podList.Items {
		pod := &podList.Items[i]
		if isCreated(pod) {
			replicas++
		}
		if isRunningAndReady(pod) && !isTerminating(pod) {
			readyReplicas++
			if isRunningAndAvailable(pod, sts.Spec.MinReadySeconds) {
				availableReplicas++
			}
		}
		if isCreated(pod) && !isTerminating(pod) {
			updatedReplicas++
		}
	}
	sts.Status.Replicas = replicas
	sts.Status.ReadyReplicas = readyReplicas
	sts.Status.AvailableReplicas = availableReplicas
	sts.Status.CurrentReplicas = currentReplicas
	sts.Status.UpdatedReplicas = updatedReplicas
	totalReplicas := int32(1)
	if sts.Spec.Replicas != nil {
		totalReplicas = *sts.Spec.Replicas
	}
	if sts.Status.Replicas == totalReplicas && sts.Status.UpdatedReplicas == totalReplicas {
		sts.Status.CurrentRevision = sts.Status.UpdateRevision
		sts.Status.CurrentReplicas = totalReplicas
	}
	return ctrl.Result{}, r.Status().Update(ctx, sts)
}

func newSTSReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &stsReconciler{
		baseReconciler{
			Client:   c,
			Scheme:   c.Scheme(),
			Recorder: recorder,
		},
	}
}

func newRestoreReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &dataprotection.RestoreReconciler{
		Client:   c,
		Scheme:   c.Scheme(),
		Recorder: recorder,
	}
}

func newBackupReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	config := intctrlutil.GeKubeRestConfig("kubeblocks-view")
	return &dataprotection.BackupReconciler{
		Client:     c,
		Scheme:     c.Scheme(),
		Recorder:   recorder,
		RestConfig: config,
	}
}

type jobReconciler struct {
	baseReconciler
}

func (r *jobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	job := &batchv1.Job{}
	err := r.Get(ctx, req.NamespacedName, job)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// handle deletion
	if model.IsObjectDeleting(job) {
		for i := int32(0); i < *job.Spec.Completions; i++ {
			podName := fmt.Sprintf("%s-%d", job.Name, i)
			pod := &corev1.Pod{}
			if err = r.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: podName}, pod); err == nil {
				if err = r.Delete(ctx, pod); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
		return ctrl.Result{}, r.Delete(ctx, job)
	}

	// Create Pods for the Job
	for i := int32(0); i < *job.Spec.Completions; i++ {
		podName := fmt.Sprintf("%s-%d", job.Name, i)
		pod := builder.NewPodBuilder(job.Namespace, podName).
			AddLabelsInMap(job.Spec.Template.Labels).
			SetPodSpec(*job.Spec.Template.Spec.DeepCopy()).
			GetObject()
		if err = controllerutil.SetControllerReference(job, pod, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		// Create Pod if not exists
		if err = r.Create(ctx, pod); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, err
		}
		// Update job status based on pod completion
		if err = r.Get(ctx, client.ObjectKeyFromObject(pod), pod); err == nil {
			if pod.Status.Phase == corev1.PodSucceeded {
				job.Status.Succeeded++
			} else if pod.Status.Phase == corev1.PodFailed {
				job.Status.Failed++
			}
		}
	}

	// Update the job status
	if err = r.Status().Update(ctx, job); err != nil {
		return ctrl.Result{}, err
	}

	// Cleanup logic: delete pods if job has succeeded or failed
	if job.Status.Succeeded == *job.Spec.Completions || job.Status.Failed > 0 {
		for i := int32(0); i < *job.Spec.Completions; i++ {
			podName := fmt.Sprintf("%s-%d", job.Name, i)
			pod := &corev1.Pod{}
			if err = r.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: podName}, pod); err == nil {
				if err = r.Delete(ctx, pod); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func newJobReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &jobReconciler{
		baseReconciler{
			Client:   c,
			Scheme:   c.Scheme(),
			Recorder: recorder,
		},
	}
}

func newSAReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newRoleBindingReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newClusterRoleBindingReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newConfigMapReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newSecretReconciler(c client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

func newDoNothingReconciler() reconcile.Reconciler {
	return &doNothingReconciler{}
}

type podReconciler struct {
	baseReconciler
}

func (r *podReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}
	err := r.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// handle deletion
	if model.IsObjectDeleting(pod) {
		return reconcile.Result{}, r.Delete(ctx, pod)
	}

	if pod.Status.Phase == corev1.PodRunning {
		return reconcile.Result{}, nil
	}

	// phase from "" to Pending
	if pod.Status.Phase == "" {
		pod.Status.Phase = corev1.PodPending
		if err = r.Status().Update(ctx, pod); err != nil {
			return reconcile.Result{}, err
		}
	}

	// phase from Pending to ContainerCreating
	// transition to PodScheduled
	// Check if the PodScheduled condition exists
	podScheduled := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled {
			podScheduled = true
		}
	}

	if !podScheduled {
		newCondition := corev1.PodCondition{
			Type:               corev1.PodScheduled,
			Status:             corev1.ConditionTrue,
			Reason:             "PodScheduled",
			Message:            "Pod has been scheduled successfully.",
			LastTransitionTime: metav1.Now(),
		}
		pod.Status.Conditions = append(pod.Status.Conditions, newCondition)
		if err = r.Status().Update(ctx, pod); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Transition to ContainerCreating
	// check if the PodReadyToStartContainers condition exists
	podReadyToStart := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReadyToStartContainers {
			podReadyToStart = true
		}
	}
	if !podReadyToStart {
		conditions := []corev1.PodCondition{
			{
				Type:               corev1.PodInitialized,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               corev1.PodReadyToStartContainers,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               corev1.ContainersReady,
				Status:             corev1.ConditionFalse,
				Reason:             "ContainersNotReady",
				Message:            "containers with unready status",
				LastTransitionTime: metav1.Now(),
			},
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionFalse,
				Reason:             "ContainersNotReady",
				Message:            "containers with unready status",
				LastTransitionTime: metav1.Now(),
			},
		}
		pod.Status.Conditions = append(pod.Status.Conditions, conditions...)
		// containers status to Waiting
		var containerStatuses []corev1.ContainerStatus
		for _, container := range pod.Spec.Containers {
			containerStatus := corev1.ContainerStatus{
				Image:   container.Image,
				ImageID: "",
				Name:    container.Name,
				Ready:   false,
				Started: pointer.Bool(false),
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason: "ContainerCreating",
					},
				},
			}
			containerStatuses = append(containerStatuses, containerStatus)
		}
		pod.Status.ContainerStatuses = containerStatuses
		if err = r.Status().Update(ctx, pod); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Transition to ContainerReady
	// Check ContainerReady condition
	containerReady := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.ContainersReady && condition.Status == corev1.ConditionTrue {
			containerReady = true
		}
	}
	if !containerReady {
		for i := range pod.Status.Conditions {
			cond := &pod.Status.Conditions[i]
			cond.Status = corev1.ConditionTrue
			cond.Reason = ""
			cond.Message = ""
		}
		for i := range pod.Status.ContainerStatuses {
			status := &pod.Status.ContainerStatuses[i]
			status.Ready = true
			status.Started = pointer.Bool(true)
			status.State = corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.Now(),
				},
			}
		}
		pod.Status.Phase = corev1.PodRunning
		if err = r.Status().Update(ctx, pod); err != nil {
			return reconcile.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func newPodReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &podReconciler{
		baseReconciler: baseReconciler{
			Client:   cli,
			Scheme:   cli.Scheme(),
			Recorder: recorder,
		},
	}
}

func newServiceReconciler(cli client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return newDoNothingReconciler()
}

var _ ReconcilerTree = &reconcilerTree{}
var _ reconcile.Reconciler = &doNothingReconciler{}
var _ reconcile.Reconciler = &podReconciler{}
var _ reconcile.Reconciler = &pvcReconciler{}
var _ reconcile.Reconciler = &pvReconciler{}
var _ reconcile.Reconciler = &volumeSnapshotV1Beta1Reconciler{}
var _ reconcile.Reconciler = &volumeSnapshotV1Reconciler{}
var _ reconcile.Reconciler = &stsReconciler{}
