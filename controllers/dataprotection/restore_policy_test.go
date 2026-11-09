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

package dataprotection

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func policyTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	return scheme
}

func serialRestore() *dpv1alpha1.Restore {
	return &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "default", UID: "restore-uid", Finalizers: []string{dptypes.DataProtectionFinalizerName}},
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: "default"}, PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
			VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
		}},
	}
}

func TestRestoreRejectsSerialBeforePreparingDependencies(t *testing.T) {
	for _, dataSource := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "dataSource"}[dataSource], func(t *testing.T) {
			restore := serialRestore()
			if dataSource {
				restore.Spec.PrepareDataConfig.DataSourceRef = &dpv1alpha1.VolumeConfig{VolumeSource: "data"}
			}
			scheme := policyTestScheme(t)
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore).WithObjects(restore).Build()
			r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
			_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
			require.NoError(t, err)
			require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(restore), restore))
			require.Equal(t, dpv1alpha1.RestorePhaseFailed, restore.Status.Phase)
			condition := meta.FindStatusCondition(restore.Status.Conditions, dprestore.ConditionTypeRestoreValidationPassed)
			require.NotNil(t, condition)
			require.Contains(t, condition.Message, "Serial is no longer supported")
			jobs := &batchv1.JobList{}
			require.NoError(t, cli.List(context.Background(), jobs))
			require.Empty(t, jobs.Items)
			pvcs := &corev1.PersistentVolumeClaimList{}
			require.NoError(t, cli.List(context.Background(), pvcs))
			require.Empty(t, pvcs.Items)
		})
	}
}

func TestSerialRestoreDrainsUnrecordedJobAcrossReconciles(t *testing.T) {
	ctx := context.Background()
	restore := serialRestore()
	restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "existing-job", Namespace: restore.Namespace, UID: "job-uid",
		Labels:          dprestore.BuildRestoreLabels(restore.Name),
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(restore, dpv1alpha1.GroupVersion.WithKind("Restore"))},
	}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "worker", Namespace: restore.Namespace,
		Labels:          dprestore.BuildRestoreLabels(restore.Name),
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job"))},
	}, Status: corev1.PodStatus{Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "restore", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
	}}
	pod.Labels["job-name"] = job.Name
	data := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-0", Namespace: restore.Namespace}}
	scheme := policyTestScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore, job, pod).
		WithObjects(restore, job, pod, data).Build()
	reconcile := func() {
		// A fresh reconciler each time must recover entirely from persisted facts.
		r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
		require.NoError(t, err)
		require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), restore))
	}
	reconcile()
	require.Equal(t, dpv1alpha1.RestorePhaseRunning, restore.Status.Phase)
	require.Contains(t, meta.FindStatusCondition(restore.Status.Conditions, "ValidationPassed").Message, "waiting for existing")
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	require.Empty(t, pod.Annotations[dprestore.DataProtectionStopRestoreManagerAnnotationKey], "do not stop a writing container")
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}
	require.NoError(t, cli.Status().Update(ctx, pod))
	reconcile()
	require.Equal(t, dpv1alpha1.RestorePhaseRunning, restore.Status.Phase)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	require.Equal(t, "true", pod.Annotations[dprestore.DataProtectionStopRestoreManagerAnnotationKey])
	pod.Status.Phase = corev1.PodSucceeded
	require.NoError(t, cli.Status().Update(ctx, pod))
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	require.NoError(t, cli.Status().Update(ctx, job))
	reconcile()
	require.Equal(t, dpv1alpha1.RestorePhaseFailed, restore.Status.Phase)
	require.NotNil(t, restore.Status.CompletionTimestamp)
	jobs := &batchv1.JobList{}
	require.NoError(t, cli.List(ctx, jobs))
	require.Len(t, jobs.Items, 1, "never create the next Serial Job")
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(data), data), "preserve recovered data")
}

func TestPVCRejectsSerialForBackupAndRestoreSources(t *testing.T) {
	for _, kind := range []string{dptypes.BackupKind, dptypes.RestoreKind} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			scheme := policyTestScheme(t)
			apiGroup := dptypes.DataprotectionAPIGroup
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "default", UID: "target-uid"},
				Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: &corev1.TypedObjectReference{APIGroup: &apiGroup, Kind: kind, Name: "source"}}}
			objects := []client.Object{pvc}
			if kind == dptypes.BackupKind {
				parameters, err := json.Marshal(map[string]string{dptypes.VolumeRestorePolicyParameterKey: "Serial"})
				require.NoError(t, err)
				pvc.Annotations = map[string]string{constant.RestoreParametersAnnotationKey: string(parameters)}
			} else {
				source := serialRestore()
				source.Name = "source"
				source.Spec.PrepareDataConfig.DataSourceRef = &dpv1alpha1.VolumeConfig{VolumeSource: "data"}
				objects = append(objects, source)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(pvc).WithObjects(objects...).Build()
			r := &VolumePopulatorReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(pvc)})
			require.NoError(t, err)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pvc), pvc))
			condition := findPVCConditionByType(pvc, appsv1.ConditionTypeRestore)
			require.NotNil(t, condition)
			require.Equal(t, corev1.ConditionFalse, condition.Status)
			require.Contains(t, condition.Message, "Serial is no longer supported")
			restores := &dpv1alpha1.RestoreList{}
			require.NoError(t, cli.List(ctx, restores))
			require.Len(t, restores.Items, len(objects)-1, "do not create an execution Restore")
		})
	}
}

func TestRejectedPVCWaitsForExecutionAndPreservesHelperData(t *testing.T) {
	ctx := context.Background()
	scheme := policyTestScheme(t)
	source := serialRestore()
	source.Spec.PrepareDataConfig.DataSourceRef = &dpv1alpha1.VolumeConfig{VolumeSource: "data"}
	apiGroup := dptypes.DataprotectionAPIGroup
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: "target", Namespace: "default", UID: "target-uid", Finalizers: []string{dptypes.DataProtectionFinalizerName},
	}, Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: &corev1.TypedObjectReference{APIGroup: &apiGroup, Kind: dptypes.RestoreKind, Name: source.Name}}}
	helper := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: getPopulatePVCName(pvc.UID), Namespace: pvc.Namespace,
		Labels: map[string]string{dprestore.DataProtectionPopulatePVCLabelKey: getPopulatePVCName(pvc.UID)},
	}, Spec: corev1.PersistentVolumeClaimSpec{VolumeName: "helper-pv"}}
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "helper-pv"},
		Spec: corev1.PersistentVolumeSpec{ClaimRef: &corev1.ObjectReference{Name: helper.Name, Namespace: helper.Namespace}}}
	child := buildExecutionRestore(pvc, helper, source)
	child.UID = "execution-uid"
	child.Finalizers = []string{dptypes.DataProtectionFinalizerName}
	child.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: pvc.Name, UID: pvc.UID}}
	child.Status.Phase = dpv1alpha1.RestorePhaseRunning
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(pvc, child).
		WithObjects(source, pvc, helper, pv, child).Build()
	r := &VolumePopulatorReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(pvc)}
	result, err := r.Reconcile(ctx, request)
	require.NoError(t, err)
	require.Positive(t, result.RequeueAfter)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pvc), pvc))
	require.Contains(t, pvc.Finalizers, dptypes.DataProtectionFinalizerName)
	childReconciler := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	_, err = childReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(child)})
	require.NoError(t, err)
	_, err = r.Reconcile(ctx, request)
	require.NoError(t, err)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pvc), pvc))
	require.NotContains(t, pvc.Finalizers, dptypes.DataProtectionFinalizerName)
	require.Equal(t, corev1.ConditionFalse, findPVCConditionByType(pvc, appsv1.ConditionTypeRestore).Status)
	require.Empty(t, pvc.Spec.VolumeName, "do not hand incomplete data to a business consumer")
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(helper), helper))
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pv), pv))
}

func TestPostReadyRejectsInheritedSerialBeforeStartingJobs(t *testing.T) {
	ctx := context.Background()
	scheme := policyTestScheme(t)
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default", UID: "cluster-uid"},
		Spec: appsv1.ClusterSpec{Restore: &appsv1.ClusterRestore{Parameters: map[string]string{dptypes.VolumeRestorePolicyParameterKey: "Serial"}}}}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "cluster-mysql", Namespace: cluster.Namespace, UID: "component-uid"}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: cluster.Namespace, Labels: map[string]string{
		constant.AppInstanceLabelKey: cluster.Name, constant.KBAppComponentLabelKey: "mysql", dptypes.ClusterUIDLabelKey: string(cluster.UID),
	}}}
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{
		Name: postReadyRestoreName(comp.UID), Namespace: cluster.Namespace, Labels: postReadyRestoreLabels(pvc, comp),
		OwnerReferences: []metav1.OwnerReference{{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ComponentKind, Name: comp.Name, UID: comp.UID}},
		Finalizers:      []string{dptypes.DataProtectionFinalizerName},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore).WithObjects(cluster, comp, restore).Build()
	r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
	require.NoError(t, err)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), restore))
	require.Equal(t, dpv1alpha1.RestorePhaseFailed, restore.Status.Phase)
	jobs := &batchv1.JobList{}
	require.NoError(t, cli.List(ctx, jobs))
	require.Empty(t, jobs.Items)
}

func TestCompletedSerialRestoreKeepsHistoricalResult(t *testing.T) {
	restore := serialRestore()
	restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
	scheme := policyTestScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore).WithObjects(restore).Build()
	r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
	require.NoError(t, err)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(restore), restore))
	require.Equal(t, dpv1alpha1.RestorePhaseCompleted, restore.Status.Phase)
	require.Nil(t, meta.FindStatusCondition(restore.Status.Conditions, "ValidationPassed"))
}

func TestRestorePolicyValidation(t *testing.T) {
	for _, policy := range []string{"", "Parallel"} {
		require.NoError(t, dptypes.ValidateVolumeRestorePolicy(policy, dptypes.VolumeRestorePolicyParameterKey))
	}
	err := dptypes.ValidateVolumeRestorePolicy("Serial", dptypes.VolumeRestorePolicyParameterKey)
	require.ErrorContains(t, err, "Serial is no longer supported")
	err = dptypes.ValidateVolumeRestorePolicy("invalid", dptypes.VolumeRestorePolicyParameterKey)
	require.ErrorContains(t, err, "invalid volume restore policy")
}

func TestSerialRestoreWaitsForPodAfterJobDeletion(t *testing.T) {
	ctx := context.Background()
	restore := serialRestore()
	restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
	jobName := "restore-preparedata-" + string(restore.UID[:8]) + "-backup-0"
	controller := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "orphan-worker", Namespace: restore.Namespace,
		Labels:          dprestore.BuildRestoreLabels(restore.Name),
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: jobName, UID: "deleted-job-uid", Controller: &controller}},
	}, Status: corev1.PodStatus{Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "restore", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
	}}
	scheme := policyTestScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore, pod).WithObjects(restore, pod).Build()
	r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)}
	result, err := r.Reconcile(ctx, request)
	require.NoError(t, err)
	require.Positive(t, result.RequeueAfter)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	require.Equal(t, "true", pod.Annotations[dprestore.DataProtectionStopRestoreManagerAnnotationKey])
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), restore))
	require.Equal(t, dpv1alpha1.RestorePhaseRunning, restore.Status.Phase)
	pod.Status.Phase = corev1.PodSucceeded
	require.NoError(t, cli.Status().Update(ctx, pod))
	_, err = r.Reconcile(ctx, request)
	require.NoError(t, err)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), restore))
	require.Equal(t, dpv1alpha1.RestorePhaseFailed, restore.Status.Phase)
}

func TestSerialRestoreDrainsActualCrossNamespaceExecJob(t *testing.T) {
	ctx := context.Background()
	scheme := policyTestScheme(t)
	oldNamespace := viper.Get(constant.CfgKeyCtrlrMgrNS)
	viper.Set(constant.CfgKeyCtrlrMgrNS, "operator")
	t.Cleanup(func() { viper.Set(constant.CfgKeyCtrlrMgrNS, oldNamespace) })
	restore := serialRestore()
	restore.Spec.ReadyConfig = &dpv1alpha1.ReadyConfig{ExecAction: &dpv1alpha1.ExecAction{Target: dpv1alpha1.ExecActionTarget{
		PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"target": "true"}},
	}}}
	restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
	repo := &dpv1alpha1.BackupRepo{ObjectMeta: metav1.ObjectMeta{Name: "repo"}}
	target := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: restore.Namespace, Labels: map[string]string{"target": "true"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "database"}}}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore, &batchv1.Job{}, &corev1.Pod{}).
		WithObjects(restore, repo, target).Build()
	backupSet := dprestore.BackupActionSet{
		Backup: &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup"}, Status: dpv1alpha1.BackupStatus{BackupRepoName: repo.Name}},
		ActionSet: &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{Restore: &dpv1alpha1.RestoreActionSpec{PostReady: []dpv1alpha1.ActionSpec{{
			Exec: &dpv1alpha1.ExecActionSpec{Command: []string{"true"}},
		}}}}},
	}
	mgr := dprestore.NewRestoreManager(restore, record.NewFakeRecorder(20), scheme, cli)
	jobs, err := mgr.BuildPostReadyActionJobs(intctrlutil.RequestCtx{Ctx: ctx, Req: ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)}}, cli, backupSet, nil, 0)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	job := jobs[0]
	job.UID = "exec-job-uid"
	require.Equal(t, "operator", job.Namespace)
	require.NoError(t, cli.Create(ctx, job))
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "exec-worker", Namespace: job.Namespace,
		Labels: job.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job"))}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "restore", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}}}
	pod.Labels["job-name"] = job.Name
	require.NoError(t, cli.Create(ctx, pod))
	r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
	require.NoError(t, err)
	require.Positive(t, result.RequeueAfter, "the Exec Job has not finished")
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	require.Equal(t, "true", pod.Annotations[dprestore.DataProtectionStopRestoreManagerAnnotationKey])
}

func TestExecutionPolicyCheckAllowsMissingSourceRestore(t *testing.T) {
	ctx := context.Background()
	scheme := policyTestScheme(t)
	source := serialRestore()
	source.Spec.PrepareDataConfig.VolumeClaimRestorePolicy = dpv1alpha1.VolumeClaimRestorePolicyParallel
	source.Spec.PrepareDataConfig.DataSourceRef = &dpv1alpha1.VolumeConfig{VolumeSource: "data"}
	apiGroup := dptypes.DataprotectionAPIGroup
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "default", UID: "target-uid"},
		Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: &corev1.TypedObjectReference{APIGroup: &apiGroup, Kind: dptypes.RestoreKind, Name: source.Name}}}
	helper := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: getPopulatePVCName(pvc.UID), Namespace: pvc.Namespace}}
	child := buildExecutionRestore(pvc, helper, source)
	child.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: pvc.Name, UID: pvc.UID}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	require.NoError(t, executionRestorePolicyError(ctx, cli, child), "a created execution Restore must not depend on the source Restore still existing")
}
