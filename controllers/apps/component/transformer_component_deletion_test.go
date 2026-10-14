/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package component

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func failedRestoreFixture() (*appsv1.Component, *dpv1alpha1.Restore, *corev1.PersistentVolumeClaim) {
	ml := constant.GetCompLabels("cluster", "db")
	comp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-db", Namespace: "default", UID: "component-uid"},
		Spec:       appsv1.ComponentSpec{TerminationPolicy: appsv1.Delete},
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: comp.Namespace, UID: "restore-uid", Labels: ml,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(comp, appsv1.GroupVersion.WithKind(appsv1.ComponentKind))}},
		Spec: dpv1alpha1.RestoreSpec{PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
			RestoreVolumeClaimsTemplate: &dpv1alpha1.RestoreVolumeClaimsTemplate{Replicas: 1, StartingIndex: 3,
				Templates: []dpv1alpha1.RestoreVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data-cluster-db", Labels: ml,
					Annotations: map[string]string{constant.RestorePVCComponentUIDAnnotationKey: string(comp.UID)}}}}},
		}},
		Status: dpv1alpha1.RestoreStatus{Phase: dpv1alpha1.RestorePhaseFailed},
	}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-cluster-db-3", Namespace: comp.Namespace,
		UID: "pvc-uid", Labels: ml, Annotations: map[string]string{constant.RestorePVCComponentUIDAnnotationKey: string(comp.UID)}}}
	return comp, restore, pvc
}

func failedRestoreClient(objects ...client.Object) client.Client {
	model.AddScheme(corev1.AddToScheme)
	model.AddScheme(batchv1.AddToScheme)
	model.AddScheme(appsv1.AddToScheme)
	model.AddScheme(dpv1alpha1.AddToScheme)
	model.AddScheme(workloads.AddToScheme)
	return fake.NewClientBuilder().WithScheme(model.GetScheme()).WithObjects(objects...).Build()
}

func TestFailedRestorePVCSelection(t *testing.T) {
	cases := []struct {
		name   string
		change func(*appsv1.Component, *dpv1alpha1.Restore, *corev1.PersistentVolumeClaim)
		want   int
	}{
		{name: "default deletion", want: 1},
		{name: "wipeout", want: 1, change: func(c *appsv1.Component, _ *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			c.Spec.TerminationPolicy = appsv1.WipeOut
		}},
		{name: "retain", change: func(c *appsv1.Component, _ *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			c.Spec.PersistentVolumeClaimRetentionPolicy = &appsv1.PersistentVolumeClaimRetentionPolicy{WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType}
		}},
		{name: "whenScaled does not change Component deletion", want: 1, change: func(c *appsv1.Component, _ *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			c.Annotations = map[string]string{constant.ComponentScaleInAnnotationKey: "true"}
			c.Spec.PersistentVolumeClaimRetentionPolicy = &appsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: appsv1.RetainPersistentVolumeClaimRetentionPolicyType}
		}},
		{name: "component scale in retains whenDeleted", change: func(c *appsv1.Component, _ *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			c.Annotations = map[string]string{constant.ComponentScaleInAnnotationKey: "true"}
			c.Spec.PersistentVolumeClaimRetentionPolicy = &appsv1.PersistentVolumeClaimRetentionPolicy{WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType, WhenScaled: appsv1.DeletePersistentVolumeClaimRetentionPolicyType}
		}},
		{name: "do not terminate", change: func(c *appsv1.Component, _ *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			c.Spec.TerminationPolicy = appsv1.DoNotTerminate
		}},
		{name: "running", change: func(_ *appsv1.Component, r *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			r.Status.Phase = dpv1alpha1.RestorePhaseRunning
		}},
		{name: "completed", change: func(_ *appsv1.Component, r *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			r.Status.Phase = dpv1alpha1.RestorePhaseCompleted
		}},
		{name: "old component UID", change: func(c *appsv1.Component, _ *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			c.UID = "new-component-uid"
		}},
		{name: "other component restore", change: func(_ *appsv1.Component, r *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			r.OwnerReferences[0].UID = "other-component"
		}},
		{name: "other owner", change: func(_ *appsv1.Component, _ *dpv1alpha1.Restore, p *corev1.PersistentVolumeClaim) {
			p.OwnerReferences = []metav1.OwnerReference{{UID: "other-owner"}}
		}},
		{name: "adopted by InstanceSet", change: func(_ *appsv1.Component, _ *dpv1alpha1.Restore, p *corev1.PersistentVolumeClaim) {
			p.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "cluster-db", UID: "its-uid"}}, workloads.GroupVersion.WithKind("InstanceSet"))}
		}},
		{name: "existing PVC without marker", change: func(_ *appsv1.Component, _ *dpv1alpha1.Restore, p *corev1.PersistentVolumeClaim) { p.Annotations = nil }},
		{name: "old restore without marker", change: func(_ *appsv1.Component, r *dpv1alpha1.Restore, _ *corev1.PersistentVolumeClaim) {
			r.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate.Templates[0].Annotations = nil
		}},
		{name: "other PVC marker", change: func(_ *appsv1.Component, _ *dpv1alpha1.Restore, p *corev1.PersistentVolumeClaim) {
			p.Annotations[constant.RestorePVCComponentUIDAnnotationKey] = "other-component"
		}},
		{name: "same labels wrong name", change: func(_ *appsv1.Component, _ *dpv1alpha1.Restore, p *corev1.PersistentVolumeClaim) {
			p.Name = "data-cluster-db-0"
		}},
		{name: "other component labels", change: func(_ *appsv1.Component, _ *dpv1alpha1.Restore, p *corev1.PersistentVolumeClaim) {
			p.Labels = constant.GetCompLabels("cluster", "other")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			comp, restore, pvc := failedRestoreFixture()
			if tc.change != nil {
				tc.change(comp, restore, pvc)
			}
			cli := failedRestoreClient(restore, pvc)
			pvcs, _, err := failedRestorePVCs(&componentTransformContext{Context: context.Background(), Client: cli}, comp, constant.GetCompLabels("cluster", "db"))
			if err != nil || len(pvcs) != tc.want {
				t.Fatalf("selected %d PVCs, error=%v; want %d", len(pvcs), err, tc.want)
			}
		})
	}
}

type restorePVCDeleteRecorder struct {
	client.Client
	options client.DeleteOptions
	calls   int
}

func (c *restorePVCDeleteRecorder) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.calls++
	c.options = client.DeleteOptions{}
	for _, opt := range opts {
		opt.ApplyToDelete(&c.options)
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func TestFailedRestorePVCDeletionOrderingAndPrecondition(t *testing.T) {
	comp, restore, pvc := failedRestoreFixture()
	cli := &restorePVCDeleteRecorder{Client: failedRestoreClient(restore, pvc)}
	transCtx := &componentTransformContext{Context: context.Background(), Client: model.NewGraphClient(cli), Component: comp,
		Logger: logr.Discard(), EventRecorder: record.NewFakeRecorder(8)}
	graphCli := transCtx.Client.(model.GraphClient)
	dag := graph.NewDAG()
	graphCli.Root(dag, comp, comp, nil)
	err := (&componentDeletionTransformer{}).deleteCompResources(transCtx, graphCli, dag, comp, constant.GetCompLabels("cluster", "db"), kindsForCompDelete())
	if err == nil || graphCli.IsAction(dag, comp, model.ActionDeletePtr()) {
		t.Fatalf("component must wait for failed PVC deletion: %v", err)
	}
	vertex := graphCli.FindMatchedVertex(dag, pvc).(*model.ObjectVertex)
	if vertex.Action == nil || *vertex.Action != model.DELETE {
		t.Fatal("PVC was not planned for deletion")
	}
	builder := &componentPlanBuilder{cli: cli, transCtx: transCtx}
	if err := builder.reconcileDeleteObject(context.Background(), vertex); err != nil {
		t.Fatal(err)
	}
	if cli.calls != 1 || cli.options.Preconditions == nil || cli.options.Preconditions.UID == nil || *cli.options.Preconditions.UID != pvc.UID {
		t.Fatalf("actual Delete call lacks UID precondition: %#v", cli.options)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(restore), &dpv1alpha1.Restore{}); err != nil {
		t.Fatalf("Restore was deleted before PVC: %v", err)
	}
	if err := builder.reconcileDeleteObject(context.Background(), vertex); err != nil {
		t.Fatalf("repeat deletion: %v", err)
	}
	pvcs, _, err := failedRestorePVCs(transCtx, comp, constant.GetCompLabels("cluster", "db"))
	if err != nil || len(pvcs) != 0 {
		t.Fatalf("repeat selection: %v, %v", pvcs, err)
	}
}

func TestFailedRestorePVCWaitsForInstanceSet(t *testing.T) {
	comp, restore, pvc := failedRestoreFixture()
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: comp.Name, Namespace: comp.Namespace, UID: types.UID("its-uid"), Labels: constant.GetCompLabels("cluster", "db")}}
	cli := failedRestoreClient(restore, pvc, its)
	graphCli := model.NewGraphClient(cli)
	transCtx := &componentTransformContext{Context: context.Background(), Client: graphCli, Component: comp, Logger: logr.Discard()}
	dag := graph.NewDAG()
	graphCli.Root(dag, comp, comp, nil)
	err := (&componentDeletionTransformer{}).deleteCompResources(transCtx, graphCli, dag, comp, constant.GetCompLabels("cluster", "db"), kindsForCompDelete())
	if err != graph.ErrPrematureStop || graphCli.FindMatchedVertex(dag, pvc) != nil || !graphCli.IsAction(dag, its, model.ActionDeletePtr()) {
		t.Fatalf("PVC deletion must wait for InstanceSet deletion: %v", err)
	}
}

func TestFailedRestorePVCDeletionProtectsReplacedClaim(t *testing.T) {
	_, _, oldPVC := failedRestoreFixture()
	oldPVC.ResourceVersion = "10"
	var requestUID types.UID
	var requestVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/namespaces/default/persistentvolumeclaims/"+oldPVC.Name {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var options metav1.DeleteOptions
		if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
			t.Error(err)
		}
		if options.Preconditions != nil && options.Preconditions.UID != nil {
			requestUID = *options.Preconditions.UID
		}
		if options.Preconditions != nil && options.Preconditions.ResourceVersion != nil {
			requestVersion = *options.Preconditions.ResourceVersion
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(&metav1.Status{TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
			Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict, Code: http.StatusConflict,
			Message: "UID precondition does not match the replacement claim"})
	}))
	defer server.Close()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim"), meta.RESTScopeNamespace)
	cli, err := client.New(&rest.Config{Host: server.URL, ContentConfig: rest.ContentConfig{ContentType: runtime.ContentTypeJSON}}, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		t.Fatal(err)
	}
	builder := &componentPlanBuilder{cli: cli}
	err = builder.reconcileDeleteObject(context.Background(), &model.ObjectVertex{Obj: oldPVC, Action: model.ActionDeletePtr()})
	if !apierrors.IsConflict(err) || requestUID != oldPVC.UID || requestVersion != oldPVC.ResourceVersion {
		t.Fatalf("replacement protection: UID=%q, resourceVersion=%q, error=%v", requestUID, requestVersion, err)
	}
}

func TestFailedRestorePVCDeletionProtectsConcurrentAdoption(t *testing.T) {
	comp, restore, pvc := failedRestoreFixture()
	cli := failedRestoreClient(restore, pvc)
	transCtx := &componentTransformContext{Context: context.Background(), Client: cli}
	selected, _, err := failedRestorePVCs(transCtx, comp, constant.GetCompLabels("cluster", "db"))
	if err != nil || len(selected) != 1 {
		t.Fatalf("selection: %v, %v", selected, err)
	}
	current := selected[0].DeepCopy()
	current.OwnerReferences = []metav1.OwnerReference{{UID: "new-owner"}}
	if err := cli.Update(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	builder := &componentPlanBuilder{cli: cli}
	err = builder.reconcileDeleteObject(context.Background(), &model.ObjectVertex{Obj: selected[0], Action: model.ActionDeletePtr()})
	if !apierrors.IsConflict(err) {
		t.Fatalf("changed claim must fail resourceVersion precondition: %v", err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(current), &corev1.PersistentVolumeClaim{}); err != nil {
		t.Fatalf("adopted claim was removed: %v", err)
	}
}

func restoreMount(name string) []corev1.Volume {
	return []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name}}}}
}

func prepareJobFixture(t *testing.T, restore *dpv1alpha1.Restore, pvc *corev1.PersistentVolumeClaim) *batchv1.Job {
	t.Helper()
	restore.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate.Templates[0].MountPath = "/data"
	cli := failedRestoreClient(pvc)
	manager := dprestore.NewRestoreManager(restore, record.NewFakeRecorder(8), model.GetScheme(), cli)
	backupSet := dprestore.BackupActionSet{Backup: &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "source-backup", Namespace: restore.Namespace}},
		ActionSet: &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{Restore: &dpv1alpha1.RestoreActionSpec{PrepareData: &dpv1alpha1.JobActionSpec{BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{Image: "test-image", Command: []string{"true"}}}}}}}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background(), Req: ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)}}
	jobs, err := manager.BuildPrepareDataJobs(reqCtx, cli, backupSet, &dpv1alpha1.BackupStatusTarget{BackupTarget: dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAny}}}, "prepareData")
	if err != nil || len(jobs) != 1 {
		t.Fatalf("real prepare Job builder: jobs=%d error=%v", len(jobs), err)
	}
	if !controllerutil.ContainsFinalizer(jobs[0], dptypes.DataProtectionFinalizerName) {
		t.Fatal("real prepare Job must carry the DP finalizer")
	}
	jobs[0].UID = "job-uid"
	jobs[0].Status.Active = 1
	created, err := manager.CreateJobsIfNotExist(reqCtx, cli, restore, jobs)
	if err != nil {
		t.Fatal(err)
	}
	return created[0]
}

func planFailedRestoreDeletion(t *testing.T, cli client.Client, comp *appsv1.Component) (*graph.DAG, model.GraphClient, *componentTransformContext) {
	t.Helper()
	graphCli := model.NewGraphClient(cli)
	transCtx := &componentTransformContext{Context: context.Background(), Client: graphCli, Component: comp, Logger: logr.Discard()}
	dag := graph.NewDAG()
	graphCli.Root(dag, comp, comp, nil)
	if err := (&componentDeletionTransformer{}).deleteCompResources(transCtx, graphCli, dag, comp, constant.GetCompLabels("cluster", "db"), kindsForCompDelete()); err == nil {
		t.Fatal("deletion must wait for referenced resources")
	}
	if graphCli.IsAction(dag, comp, model.ActionDeletePtr()) {
		t.Fatal("Component cannot disappear before failed-restore cleanup")
	}
	return dag, graphCli, transCtx
}

func TestFailedRestoreStopsJobAndWaitsForPodsBeforePVC(t *testing.T) {
	comp, restore, pvc := failedRestoreFixture()
	job := prepareJobFixture(t, restore, pvc)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "prepare-pod", Namespace: job.Namespace, UID: "pod-uid",
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job"))}},
		Spec: corev1.PodSpec{NodeName: "node", Volumes: restoreMount(pvc.Name)}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	cli := &restorePVCDeleteRecorder{Client: failedRestoreClient(restore, pvc, job, pod)}
	dag, graphCli, transCtx := planFailedRestoreDeletion(t, cli, comp)
	vertex := graphCli.FindMatchedVertex(dag, job).(*model.ObjectVertex)
	builder := &componentPlanBuilder{cli: cli, transCtx: transCtx}
	if vertex.Action == nil || *vertex.Action != model.UPDATE || graphCli.FindMatchedVertex(dag, pvc) != nil {
		t.Fatal("DP finalizer must be removed before Job/PVC deletion")
	}
	if err := builder.reconcileUpdateObject(context.Background(), vertex); err != nil {
		t.Fatal(err)
	}
	dag, graphCli, _ = planFailedRestoreDeletion(t, cli, comp)
	vertex = graphCli.FindMatchedVertex(dag, job).(*model.ObjectVertex)
	if vertex.PropagationPolicy != client.PropagationPolicy(metav1.DeletePropagationForeground) || graphCli.FindMatchedVertex(dag, pvc) != nil {
		t.Fatal("Job must be foreground-deleted before the PVC")
	}
	if err := builder.reconcileDeleteObject(context.Background(), vertex); err != nil {
		t.Fatal(err)
	}
	if cli.options.PropagationPolicy == nil || *cli.options.PropagationPolicy != metav1.DeletePropagationForeground ||
		cli.options.Preconditions == nil || cli.options.Preconditions.UID == nil || *cli.options.Preconditions.UID != job.UID ||
		cli.options.Preconditions.ResourceVersion == nil || *cli.options.Preconditions.ResourceVersion != vertex.Obj.GetResourceVersion() {
		t.Fatalf("actual Job Delete options=%#v", cli.options)
	}
	// The fake client has no garbage collector: keep the Job's child Pod to prove the wait barrier.
	dag, graphCli, _ = planFailedRestoreDeletion(t, cli, comp)
	if graphCli.FindMatchedVertex(dag, pvc) != nil {
		t.Fatal("PVC deletion cannot precede child Pod removal")
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(restore), &dpv1alpha1.Restore{}); err != nil {
		t.Fatal("Restore evidence disappeared before cleanup")
	}
	if err := cli.Delete(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	dag, graphCli, _ = planFailedRestoreDeletion(t, cli, comp)
	vertex = graphCli.FindMatchedVertex(dag, pvc).(*model.ObjectVertex)
	if err := builder.reconcileDeleteObject(context.Background(), vertex); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{}); !apierrors.IsNotFound(err) {
		t.Fatalf("PVC cleanup after Job/Pod removal: %v", err)
	}
}

func TestFailedRestoreKeepsUnrelatedReferences(t *testing.T) {
	for _, phase := range []corev1.PodPhase{corev1.PodPending, corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed} {
		t.Run(string(phase), func(t *testing.T) {
			comp, restore, pvc := failedRestoreFixture()
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: pvc.Namespace, UID: "other-pod"},
				Spec: corev1.PodSpec{NodeName: "node", Volumes: restoreMount(pvc.Name)}, Status: corev1.PodStatus{Phase: phase}}
			cli := failedRestoreClient(restore, pvc, pod)
			dag, graphCli, _ := planFailedRestoreDeletion(t, cli, comp)
			if graphCli.FindMatchedVertex(dag, pvc) != nil || graphCli.FindMatchedVertex(dag, pod) != nil {
				t.Fatal("reference must block cleanup without deleting an unrelated Pod")
			}
		})
	}
	for _, owner := range []string{"old-restore-uid", "other-component-restore-uid"} {
		t.Run(owner, func(t *testing.T) {
			comp, restore, pvc := failedRestoreFixture()
			job := prepareJobFixture(t, restore, pvc)
			job.OwnerReferences[0].UID = types.UID(owner)
			cli := failedRestoreClient(restore, pvc, job)
			dag, graphCli, _ := planFailedRestoreDeletion(t, cli, comp)
			if graphCli.FindMatchedVertex(dag, job) != nil || graphCli.FindMatchedVertex(dag, pvc) != nil {
				t.Fatal("unknown Restore owner must not be terminated or lose its referenced PVC")
			}
		})
	}
}

func TestFailedRestoreDoesNotTouchAnotherComponent(t *testing.T) {
	comp, restore, pvc := failedRestoreFixture()
	_, otherRestore, otherPVC := failedRestoreFixture()
	otherRestore.Name, otherRestore.UID = "other-restore", "other-restore-uid"
	otherRestore.OwnerReferences[0].Name, otherRestore.OwnerReferences[0].UID = "cluster-other", "other-component-uid"
	otherRestore.Labels = constant.GetCompLabels("cluster", "other")
	claim := &otherRestore.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate.Templates[0]
	claim.Name = "data-cluster-other"
	claim.Labels = otherRestore.Labels
	claim.Annotations[constant.RestorePVCComponentUIDAnnotationKey] = "other-component-uid"
	otherPVC.Name, otherPVC.UID = "data-cluster-other-3", "other-pvc-uid"
	otherPVC.Labels = otherRestore.Labels
	otherPVC.Annotations[constant.RestorePVCComponentUIDAnnotationKey] = "other-component-uid"
	otherJob := prepareJobFixture(t, otherRestore, otherPVC)
	cli := failedRestoreClient(restore, pvc, otherRestore, otherPVC, otherJob)
	dag, graphCli, _ := planFailedRestoreDeletion(t, cli, comp)
	if !graphCli.IsAction(dag, pvc, model.ActionDeletePtr()) || graphCli.FindMatchedVertex(dag, otherPVC) != nil || graphCli.FindMatchedVertex(dag, otherJob) != nil {
		t.Fatal("cleanup must be limited to the selected Component and its exact claims")
	}
}

func TestFailedRestoreJobDeletionProtectsReplacedJob(t *testing.T) {
	_, restore, pvc := failedRestoreFixture()
	oldJob := prepareJobFixture(t, restore, pvc)
	oldJob.ResourceVersion = "10"
	liveUID := types.UID("replacement-job-uid")
	deleted := false
	var requested metav1.DeleteOptions
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/apis/batch/v1/namespaces/default/jobs/"+oldJob.Name {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&requested); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		status := metav1.Status{TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"}, Status: metav1.StatusSuccess, Code: http.StatusOK}
		if requested.Preconditions != nil && requested.Preconditions.UID != nil && *requested.Preconditions.UID != liveUID {
			status.Status, status.Reason, status.Code = metav1.StatusFailure, metav1.StatusReasonConflict, http.StatusConflict
			w.WriteHeader(http.StatusConflict)
		} else {
			deleted = true
		}
		_ = json.NewEncoder(w).Encode(&status)
	}))
	defer server.Close()
	scheme := runtime.NewScheme()
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{batchv1.SchemeGroupVersion})
	mapper.Add(batchv1.SchemeGroupVersion.WithKind("Job"), meta.RESTScopeNamespace)
	cli, err := client.New(&rest.Config{Host: server.URL, ContentConfig: rest.ContentConfig{ContentType: runtime.ContentTypeJSON}}, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		t.Fatal(err)
	}
	builder := &componentPlanBuilder{cli: cli}
	err = builder.reconcileDeleteObject(context.Background(), &model.ObjectVertex{Obj: oldJob, Action: model.ActionDeletePtr(), PropagationPolicy: client.PropagationPolicy(metav1.DeletePropagationForeground)})
	if !apierrors.IsConflict(err) || deleted || requested.Preconditions == nil || requested.Preconditions.UID == nil || *requested.Preconditions.UID != oldJob.UID ||
		requested.Preconditions.ResourceVersion == nil || *requested.Preconditions.ResourceVersion != oldJob.ResourceVersion || requested.PropagationPolicy == nil || *requested.PropagationPolicy != metav1.DeletePropagationForeground {
		t.Fatalf("replacement Job protection: options=%#v deleted=%v error=%v", requested, deleted, err)
	}
}

func TestFailedRestoreDPFinalizerLifecycle(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		for _, foreign := range []bool{false, true} {
			name := fmt.Sprintf("deleting=%t/foreign=%t", deleting, foreign)
			t.Run(name, func(t *testing.T) {
				comp, restore, pvc := failedRestoreFixture()
				job := prepareJobFixture(t, restore, pvc)
				if foreign {
					job.Finalizers = append(job.Finalizers, "example.test/hold")
				}
				cli := failedRestoreClient(restore, pvc, job)
				if deleting {
					if err := cli.Delete(context.Background(), job); err != nil {
						t.Fatal(err)
					}
				}
				dag, graphCli, transCtx := planFailedRestoreDeletion(t, cli, comp)
				vertex := graphCli.FindMatchedVertex(dag, job).(*model.ObjectVertex)
				if vertex.Action == nil || *vertex.Action != model.UPDATE || graphCli.FindMatchedVertex(dag, pvc) != nil {
					t.Fatal("only DP-finalizer update is allowed first")
				}
				updated := vertex.Obj.(*batchv1.Job)
				if controllerutil.ContainsFinalizer(updated, dptypes.DataProtectionFinalizerName) || controllerutil.ContainsFinalizer(updated, "example.test/hold") != foreign {
					t.Fatalf("incorrect finalizer removal: %v", updated.Finalizers)
				}
				builder := &componentPlanBuilder{cli: cli, transCtx: transCtx}
				if err := builder.reconcileUpdateObject(context.Background(), vertex); err != nil {
					t.Fatal(err)
				}
				current := &batchv1.Job{}
				err := cli.Get(context.Background(), client.ObjectKeyFromObject(job), current)
				if deleting && !foreign {
					if !apierrors.IsNotFound(err) {
						t.Fatalf("last DP finalizer must release a deleting Job: %v", err)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					dag, graphCli, _ = planFailedRestoreDeletion(t, cli, comp)
					vertex = graphCli.FindMatchedVertex(dag, job).(*model.ObjectVertex)
					if vertex.Action == nil || *vertex.Action != model.DELETE || graphCli.FindMatchedVertex(dag, pvc) != nil {
						t.Fatal("observe DP finalizer removal before foreground deletion")
					}
					if err := builder.reconcileDeleteObject(context.Background(), vertex); err != nil {
						t.Fatal(err)
					}
					err = cli.Get(context.Background(), client.ObjectKeyFromObject(job), current)
					if foreign {
						if err != nil || !controllerutil.ContainsFinalizer(current, "example.test/hold") {
							t.Fatal("foreign finalizer must be preserved")
						}
						dag, graphCli, _ = planFailedRestoreDeletion(t, cli, comp)
						if graphCli.FindMatchedVertex(dag, pvc) != nil {
							t.Fatal("foreign-finalized Job must continue protecting its claim")
						}
					} else if !apierrors.IsNotFound(err) {
						t.Fatalf("Job cleanup remained blocked: %v", err)
					}
				}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(restore), &dpv1alpha1.Restore{}); err != nil {
					t.Fatal("Restore evidence must be retained")
				}
			})
		}
	}
}

func TestFailedRestoreDPFinalizerUpdatePreconditions(t *testing.T) {
	for _, mode := range []string{"matching", "replaced UID", "adopted resourceVersion"} {
		t.Run(mode, func(t *testing.T) {
			comp, restore, pvc := failedRestoreFixture()
			job := prepareJobFixture(t, restore, pvc)
			job.ResourceVersion = "10"
			job.Finalizers = append(job.Finalizers, "example.test/hold")
			dag, graphCli, _ := planFailedRestoreDeletion(t, failedRestoreClient(restore, pvc, job), comp)
			vertex := graphCli.FindMatchedVertex(dag, job).(*model.ObjectVertex)
			liveUID, liveVersion := job.UID, job.ResourceVersion
			if mode == "replaced UID" {
				liveUID = "replacement-uid"
			}
			if mode == "adopted resourceVersion" {
				liveVersion = "11"
			}
			var request batchv1.Job
			mutated := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut || r.URL.Path != "/apis/batch/v1/namespaces/default/jobs/"+job.Name {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				if request.UID != liveUID || request.ResourceVersion != liveVersion {
					w.WriteHeader(http.StatusConflict)
					_ = json.NewEncoder(w).Encode(&metav1.Status{TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"}, Status: metav1.StatusFailure, Reason: metav1.StatusReasonConflict, Code: http.StatusConflict})
					return
				}
				mutated = true
				request.ResourceVersion = "11"
				_ = json.NewEncoder(w).Encode(&request)
			}))
			defer server.Close()
			scheme := runtime.NewScheme()
			if err := batchv1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{batchv1.SchemeGroupVersion})
			mapper.Add(batchv1.SchemeGroupVersion.WithKind("Job"), meta.RESTScopeNamespace)
			cli, err := client.New(&rest.Config{Host: server.URL, ContentConfig: rest.ContentConfig{ContentType: runtime.ContentTypeJSON}}, client.Options{Scheme: scheme, Mapper: mapper})
			if err != nil {
				t.Fatal(err)
			}
			err = (&componentPlanBuilder{cli: cli}).reconcileUpdateObject(context.Background(), vertex)
			if mode == "matching" {
				if err != nil || !mutated || request.UID != job.UID || len(request.Finalizers) != 1 || request.Finalizers[0] != "example.test/hold" {
					t.Fatalf("guarded finalizer update: %#v %v", request.ObjectMeta, err)
				}
			} else if !apierrors.IsConflict(err) || mutated {
				t.Fatalf("stale Update must preserve replacement/adopted Job: %v", err)
			}
		})
	}
}
