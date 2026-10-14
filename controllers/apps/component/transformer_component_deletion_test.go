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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
			pvcs, err := failedRestorePVCs(&componentTransformContext{Context: context.Background(), Client: cli}, comp, constant.GetCompLabels("cluster", "db"))
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
	pvcs, err := failedRestorePVCs(transCtx, comp, constant.GetCompLabels("cluster", "db"))
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
	selected, err := failedRestorePVCs(transCtx, comp, constant.GetCompLabels("cluster", "db"))
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
