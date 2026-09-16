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

package cluster

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

func TestClusterReconcilePreservesConcurrentScaling(t *testing.T) {
	model.AddScheme(appsv1.AddToScheme)
	for _, sharded := range []bool{false, true} {
		name := "component replicas"
		if sharded {
			name = "sharding shards"
		}
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			g.Expect(appsv1.AddToScheme(scheme)).To(Succeed())
			compDef := testapps.NewComponentDefinitionFactory("mysql-v1").SetServiceVersion("1.0").GetObject()
			compDef.Status.Phase = appsv1.AvailablePhase
			shardingDef := testapps.NewShardingDefinitionFactory("mysql-sharding", compDef.Name).
				AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).GetObject()
			shardingDef.Status.Phase = appsv1.AvailablePhase
			factory := testapps.NewClusterFactory("default", "concurrent-scaling", "")
			if sharded {
				factory.AddSharding("mysql", shardingDef.Name, "").SetShards(2)
			} else {
				factory.AddComponent("mysql", compDef.Name).SetReplicas(2)
			}
			cluster := factory.GetObject()
			cluster.UID = "concurrent-scaling"
			cluster.Generation = 1
			key := client.ObjectKeyFromObject(cluster)
			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(&appsv1.Cluster{}, &appsv1.Component{}).
				WithObjects(cluster, compDef, shardingDef).Build()
			components := &appsv1.ComponentList{}
			injected, conflicted := false, false
			racingClient := interceptor.NewClient(cli, interceptor.Funcs{
				Patch: func(ctx context.Context, cli client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					if _, ok := obj.(*appsv1.Cluster); ok && !injected {
						injected = true
						// Components are already persisted, but normalized Cluster fields are not.
						g.Expect(cli.List(ctx, components)).To(Succeed())
						expected := 1
						if sharded {
							expected = 2
						}
						g.Expect(components.Items).To(HaveLen(expected))
						live := &appsv1.Cluster{}
						g.Expect(cli.Get(ctx, key, live)).To(Succeed())
						if sharded {
							live.Spec.Shardings[0].Shards = 3
						} else {
							live.Spec.ComponentSpecs[0].Replicas = 3
						}
						live.Finalizers = append(live.Finalizers, "example.com/concurrent-owner")
						g.Expect(cli.Update(ctx, live)).To(Succeed())
					}
					err := cli.Patch(ctx, obj, patch, opts...)
					conflicted = conflicted || apierrors.IsConflict(err)
					return err
				},
			})
			reconciler := &ClusterReconciler{Client: racingClient, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(injected).To(BeTrue())
			g.Expect(cli.Get(ctx, key, cluster)).To(Succeed())
			if sharded {
				g.Expect(cluster.Spec.Shardings[0].Shards).To(Equal(int32(3)))
			} else {
				g.Expect(cluster.Spec.ComponentSpecs[0].Replicas).To(Equal(int32(3)))
			}
			g.Expect(cluster.Finalizers).To(ContainElement("example.com/concurrent-owner"))
			g.Expect(conflicted).To(BeTrue())
			g.Expect(result.Requeue).To(BeTrue())

			// A fresh reconciler must recover from the partial writes using current resources.
			reconciler = &ClusterReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}
			_, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cli.Get(ctx, key, cluster)).To(Succeed())
			g.Expect(cluster.Finalizers).To(ContainElements("example.com/concurrent-owner", constant.DBClusterFinalizerName))
			g.Expect(cli.List(ctx, components)).To(Succeed())
			if sharded {
				g.Expect(cluster.Spec.Shardings[0].Shards).To(Equal(int32(3)))
				g.Expect(cluster.Spec.Shardings[0].Template.ServiceVersion).To(Equal("1.0"))
				g.Expect(components.Items).To(HaveLen(3))
			} else {
				g.Expect(cluster.Spec.ComponentSpecs[0].Replicas).To(Equal(int32(3)))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).To(Equal("1.0"))
				g.Expect(components.Items).To(HaveLen(1))
				g.Expect(components.Items[0].Spec.Replicas).To(Equal(int32(3)))
			}
		})
	}
}

var _ = Describe("cluster plan builder test", func() {
	const (
		clusterDefName = "test-clusterdef"
		clusterName    = "test-cluster" // this become cluster prefix name if used with testapps.NewClusterFactory().WithRandomName()
	)

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.VolumeSnapshotSignature, true, inNS)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.ActionSetSignature, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("test init", func() {
		It("should init successfully", func() {
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName).
				WithRandomName().
				GetObject()
			Expect(testCtx.Cli.Create(testCtx.Ctx, clusterObj)).Should(Succeed())
			clusterKey := client.ObjectKeyFromObject(clusterObj)
			Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1.Cluster{}, true)).Should(Succeed())
			req := ctrl.Request{
				NamespacedName: clusterKey,
			}
			reqCtx := intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Req: req,
				Log: log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
			}
			planBuilder := newClusterPlanBuilder(reqCtx, testCtx.Cli)
			Expect(planBuilder.Init()).Should(Succeed())
		})
	})
})
