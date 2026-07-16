/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	customops "github.com/apecloud/kubeblocks/pkg/operations/custom"
)

type managedJobOwnerGateReader struct {
	client.Reader
	clusterErr error
	clusterGet int
	jobGet     int
	jobList    int
}

type managedJobRoundReader struct {
	client.Reader
	clusterGet     int
	jobGet         int
	unexpectedRead int
	order          []string
}

func (r *managedJobRoundReader) Get(ctx context.Context, key client.ObjectKey,
	obj client.Object, opts ...client.GetOption) error {
	switch obj.(type) {
	case *appsv1.Cluster:
		r.clusterGet++
		r.order = append(r.order, "Cluster")
	case *batchv1.Job:
		r.jobGet++
		r.order = append(r.order, "Job")
	default:
		r.unexpectedRead++
		return errors.New("dynamic ManagedJob input read is forbidden after the exact Job exists")
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func (r *managedJobRoundReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	r.unexpectedRead++
	return errors.New("dynamic ManagedJob input list is forbidden after the exact Job exists")
}

type managedJobRoundClient struct {
	client.Client
	dryRunCreates int
	jobCreates    int
	defaultedJob  *batchv1.Job
}

func (c *managedJobRoundClient) Create(ctx context.Context, obj client.Object,
	opts ...client.CreateOption) error {
	if job, ok := obj.(*batchv1.Job); ok {
		if len(opts) > 0 {
			c.dryRunCreates++
			c.defaultedJob = job.DeepCopy()
		} else {
			c.jobCreates++
		}
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (r *managedJobOwnerGateReader) Get(ctx context.Context, key client.ObjectKey,
	obj client.Object, opts ...client.GetOption) error {
	switch obj.(type) {
	case *appsv1.Cluster:
		r.clusterGet++
		if r.clusterErr != nil {
			return r.clusterErr
		}
	case *batchv1.Job:
		r.jobGet++
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func (r *managedJobOwnerGateReader) List(ctx context.Context, list client.ObjectList,
	opts ...client.ListOption) error {
	if reflect.TypeOf(list) == reflect.TypeOf(&batchv1.JobList{}) {
		r.jobList++
	}
	return r.Reader.List(ctx, list, opts...)
}

func TestSnapshottedCustomOpsDefinitionNotFoundIsFatal(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "redis", UID: types.UID("cluster-uid"),
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	opsRequest := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "managed-shard-add",
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(cluster, appsv1.GroupVersion.WithKind(appsv1.ClusterKind)),
		},
	}, Spec: opsv1alpha1.OpsRequestSpec{
		Type: opsv1alpha1.CustomType, ClusterName: cluster.Name,
		SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{CustomOps: &opsv1alpha1.CustomOps{
			OpsDefinitionName: "missing-managed-definition",
			ExecutionSnapshot: &opsv1alpha1.CustomOpsExecutionSnapshot{
				OpsDefinitionUID:        "missing-uid",
				OpsDefinitionGeneration: 1,
				OpsDefinitionSpecHash:   strings.Repeat("a", 64),
				TargetSnapshotHash:      strings.Repeat("b", 64),
			},
		}},
	}}
	err := initOpsDefAndValidate(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, &OpsResource{
		Reader: cli, OpsRequest: opsRequest, Cluster: cluster.DeepCopy(),
	})
	if err == nil || !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		t.Fatalf("error=%v, want fatal NotFound for snapshotted OpsDefinition", err)
	}
}

func TestManagedJobRequiresExactLiveClusterControllerOwner(t *testing.T) {
	const namespace = "default"
	const clusterName = "redis"
	cluster := &appsv1.Cluster{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ClusterKind},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
			UID:       types.UID("current-cluster-uid"),
		},
	}
	newOpsRequest := func() *opsv1alpha1.OpsRequest {
		return &opsv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "managed-shard-add"},
			Spec: opsv1alpha1.OpsRequestSpec{
				ClusterName: clusterName,
				Type:        opsv1alpha1.CustomType,
				SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{CustomOps: &opsv1alpha1.CustomOps{
					ExecutionSnapshot: &opsv1alpha1.CustomOpsExecutionSnapshot{},
				}},
			},
		}
	}
	owner := func(name string, uid types.UID) metav1.OwnerReference {
		controller := true
		return metav1.OwnerReference{
			APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ClusterKind,
			Name: name, UID: uid, Controller: &controller,
		}
	}

	tests := []struct {
		name        string
		mutate      func(*opsv1alpha1.OpsRequest, *appsv1.Cluster)
		readerErr   error
		wantFatal   bool
		wantReadErr bool
	}{
		{name: "missing controller owner", wantFatal: true},
		{name: "different cluster owner", wantFatal: true, mutate: func(ops *opsv1alpha1.OpsRequest, _ *appsv1.Cluster) {
			ops.OwnerReferences = []metav1.OwnerReference{owner("other-cluster", types.UID("other-cluster-uid"))}
		}},
		{name: "same-name stale cluster UID", wantFatal: true, mutate: func(ops *opsv1alpha1.OpsRequest, _ *appsv1.Cluster) {
			ops.OwnerReferences = []metav1.OwnerReference{owner(clusterName, types.UID("old-cluster-uid"))}
		}},
		{name: "live cluster terminating", wantFatal: true, mutate: func(ops *opsv1alpha1.OpsRequest, live *appsv1.Cluster) {
			ops.OwnerReferences = []metav1.OwnerReference{owner(clusterName, live.UID)}
			live.Finalizers = []string{"test-finalizer"}
			now := metav1.Now()
			live.DeletionTimestamp = &now
		}},
		{name: "APIReader error", readerErr: errors.New("temporary live cluster read failure"), wantReadErr: true},
		{name: "exact current cluster owner", mutate: func(ops *opsv1alpha1.OpsRequest, live *appsv1.Cluster) {
			ops.OwnerReferences = []metav1.OwnerReference{owner(clusterName, live.UID)}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			liveCluster := cluster.DeepCopy()
			liveCluster.Annotations = map[string]string{"source": "api-reader"}
			opsRequest := newOpsRequest()
			if test.mutate != nil {
				test.mutate(opsRequest, liveCluster)
			}
			scheme := runtime.NewScheme()
			if err := appsv1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			baseReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(liveCluster).Build()
			reader := &managedJobOwnerGateReader{Reader: baseReader, clusterErr: test.readerErr}
			cachedCluster := cluster.DeepCopy()
			cachedCluster.Annotations = map[string]string{"source": "cache"}
			opsRes := &OpsResource{
				Reader: reader, OpsRequest: opsRequest, Cluster: cachedCluster,
			}
			err := validateManagedJobOpsRequestOwner(context.Background(), reader, opsRes)
			switch {
			case test.wantReadErr:
				if !errors.Is(err, test.readerErr) {
					t.Fatalf("error=%v, want APIReader error", err)
				}
			case test.wantFatal:
				if err == nil || !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
					t.Fatalf("error=%v, want fatal owner gate rejection", err)
				}
			default:
				if err != nil {
					t.Fatalf("exact owner gate failed: %v", err)
				}
				if opsRes.Cluster.Annotations["source"] != "api-reader" {
					t.Fatalf("ManagedJob retained stale cached Cluster inputs: %#v", opsRes.Cluster.Annotations)
				}
			}
			if reader.clusterGet != 1 {
				t.Fatalf("live Cluster GETs=%d, want exactly 1", reader.clusterGet)
			}
			if reader.jobGet != 0 || reader.jobList != 0 {
				t.Fatalf("owner gate touched Jobs: get=%d list=%d", reader.jobGet, reader.jobList)
			}
		})
	}
}

func TestOrdinaryCustomDoesNotRequireManagedJobOwnerGate(t *testing.T) {
	opsRequest := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{
		Type:               opsv1alpha1.CustomType,
		SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{CustomOps: &opsv1alpha1.CustomOps{}},
	}}
	if err := validateManagedJobOpsRequestOwner(context.Background(), nil, &OpsResource{
		OpsRequest: opsRequest, Cluster: &appsv1.Cluster{},
	}); err != nil {
		t.Fatalf("ordinary Custom unexpectedly required ManagedJob owner gate: %v", err)
	}
}

func TestManagedJobRoundReadsLiveClusterThenExistingJobOnly(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		batchv1.AddToScheme,
		appsv1.AddToScheme,
		opsv1alpha1.AddToScheme,
	} {
		if err := addToScheme(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cluster := &appsv1.Cluster{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ClusterKind},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default", Name: "redis", UID: types.UID("cluster-uid"),
			Annotations: map[string]string{"source": "api-reader"},
		},
		Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
			Name: "shard", ComponentDef: "redis-cluster", Replicas: 1,
		}}},
	}
	opsRequest := &opsv1alpha1.OpsRequest{
		TypeMeta: metav1.TypeMeta{APIVersion: opsv1alpha1.GroupVersion.String(), Kind: "OpsRequest"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace, Name: "managed-shard-add", UID: types.UID("ops-uid"),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, appsv1.GroupVersion.WithKind(appsv1.ClusterKind)),
			},
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			ClusterName: cluster.Name,
			Type:        opsv1alpha1.CustomType,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{CustomOps: &opsv1alpha1.CustomOps{
				ExecutionSnapshot: &opsv1alpha1.CustomOpsExecutionSnapshot{
					OpsDefinitionUID:        "opsdef-uid",
					OpsDefinitionGeneration: 1,
					OpsDefinitionSpecHash:   strings.Repeat("a", 64),
					TargetSnapshotHash:      strings.Repeat("b", 64),
				},
			}},
		},
	}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	cli := &managedJobRoundClient{Client: base}
	action := &opsv1alpha1.OpsAction{
		Name: "managed-job",
		Workload: &opsv1alpha1.OpsWorkloadAction{
			Type: opsv1alpha1.ManagedJobWorkload,
			PodSpec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "worker", Image: "redis:7",
			}}},
		},
	}
	componentSpec := &cluster.Spec.ComponentSpecs[0]
	componentOps := &opsv1alpha1.CustomOpsComponent{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: componentSpec.Name},
	}
	planner := customops.NewWorkloadAction(opsRequest, cluster, &opsv1alpha1.OpsDefinition{},
		componentOps, componentSpec, opsv1alpha1.ProgressStatusDetail{})
	actionCtx := customops.ActionContext{
		ReqCtx: intctrlutil.RequestCtx{Ctx: ctx}, Client: cli, Reader: base, Action: action,
	}
	planned, err := planner.Execute(actionCtx)
	if err != nil {
		t.Fatalf("plan ManagedJob: %v", err)
	}
	if len(planned.ActionTasks) != 1 || cli.defaultedJob == nil {
		t.Fatalf("planned tasks=%d defaultedJob=%v, want one persisted plan", len(planned.ActionTasks), cli.defaultedJob != nil)
	}
	liveJob := cli.defaultedJob.DeepCopy()
	liveJob.UID = types.UID("managed-job-uid")
	if err := base.Create(ctx, liveJob); err != nil {
		t.Fatalf("create exact live Job fixture: %v", err)
	}
	baselineDryRuns, baselineCreates := cli.dryRunCreates, cli.jobCreates

	roundReader := &managedJobRoundReader{Reader: base}
	opsRes := &OpsResource{
		Reader:     roundReader,
		OpsRequest: opsRequest,
		Cluster: &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace, Name: cluster.Name, UID: cluster.UID,
			Annotations: map[string]string{"source": "cache"},
		}},
	}
	if err := validateManagedJobOpsRequestOwner(ctx, roundReader, opsRes); err != nil {
		t.Fatalf("live Cluster owner gate: %v", err)
	}
	consumer := customops.NewWorkloadAction(opsRequest, opsRes.Cluster, &opsv1alpha1.OpsDefinition{},
		componentOps, &opsRes.Cluster.Spec.ComponentSpecs[0], opsv1alpha1.ProgressStatusDetail{
			ActionTasks: planned.ActionTasks,
		})
	actionCtx.Reader = roundReader
	status, err := consumer.CheckStatus(actionCtx)
	if err != nil {
		t.Fatalf("bind existing ManagedJob: %v", err)
	}
	if len(status.ActionTasks) != 1 || status.ActionTasks[0].DispatchState != opsv1alpha1.CreatedActionTaskDispatchState ||
		status.ActionTasks[0].WorkloadUID != string(liveJob.UID) {
		t.Fatalf("bound task=%#v, want exact existing Job UID", status.ActionTasks)
	}
	if roundReader.clusterGet != 1 || roundReader.jobGet != 1 || roundReader.unexpectedRead != 0 {
		t.Fatalf("direct reads: Cluster=%d Job=%d unexpected=%d, want 1/1/0",
			roundReader.clusterGet, roundReader.jobGet, roundReader.unexpectedRead)
	}
	if !reflect.DeepEqual(roundReader.order, []string{"Cluster", "Job"}) {
		t.Fatalf("direct read order=%v, want [Cluster Job]", roundReader.order)
	}
	if cli.dryRunCreates != baselineDryRuns || cli.jobCreates != baselineCreates {
		t.Fatalf("existing Job path created workloads: dry-run %d->%d real %d->%d",
			baselineDryRuns, cli.dryRunCreates, baselineCreates, cli.jobCreates)
	}
}
