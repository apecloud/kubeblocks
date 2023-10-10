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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Component", func() {
	const (
		statelessCompName      = "stateless"
		statelessCompDefName   = "stateless"
		statefulCompName       = "stateful"
		statefulCompDefName    = "stateful"
		replicationCompName    = "replication"
		replicationCompDefName = "replication"
		consensusCompName      = "consensus"
		consensusCompDefName   = "consensus"
	)

	var (
		random         = testCtx.GetRandomStr()
		clusterDefName = "test-clusterdef-" + random
		clusterVerName = "test-clusterver-" + random
		clusterName    = "test-cluster" + random
		clusterDefObj  *appsv1alpha1.ClusterDefinition
		clusterVerObj  *appsv1alpha1.ClusterVersion
		clusterObj     *appsv1alpha1.Cluster
		reqCtx         intctrlutil.RequestCtx
		dag            *graph.DAG
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResources(&testCtx, generics.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	setup := func() {
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatelessNginxComponent, statelessCompDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
			AddComponentDef(testapps.ReplicationRedisComponent, replicationCompDefName).
			AddComponentDef(testapps.ConsensusMySQLComponent, consensusCompDefName).
			GetObject()
		clusterVerObj = testapps.NewClusterVersionFactory(clusterVerName, clusterDefObj.GetName()).
			AddComponentVersion(statelessCompDefName).AddContainerShort("nginx", testapps.NginxImage).
			AddComponentVersion(statefulCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(replicationCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(consensusCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			GetObject()

		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVerObj.Name).
			AddComponent(statelessCompName, statelessCompDefName).
			SetReplicas(1).
			AddComponent(statefulCompName, statefulCompDefName).
			SetReplicas(2).
			AddVolumeClaimTemplate("data", testapps.NewPVCSpec("2Gi")).
			AddComponent(replicationCompName, replicationCompDefName).
			SetReplicas(2).
			AddVolumeClaimTemplate("data", testapps.NewPVCSpec("2Gi")).
			AddComponent(consensusCompName, consensusCompDefName).
			SetReplicas(3).
			AddVolumeClaimTemplate("data", testapps.NewPVCSpec("2Gi")).
			GetObject()

		reqCtx = intctrlutil.RequestCtx{Ctx: ctx, Log: logger, Recorder: recorder}
		dag = graph.NewDAG()
	}

	resetDag := func(comp Component) {
		Expect(comp).ShouldNot(BeNil())
		rsmComp, ok := comp.(*rsmComponent)
		Expect(ok).Should(BeTrue())
		dag = graph.NewDAG()
		rsmComp.dag = dag
	}

	submitChanges := func(ctx context.Context, cli client.Client, dag *graph.DAG) error {
		walking := func(v graph.Vertex) error {
			node, ok := v.(*ictrltypes.LifecycleVertex)
			Expect(ok).Should(BeTrue())

			_, ok = node.Obj.(*appsv1alpha1.Cluster)
			Expect(ok).ShouldNot(BeTrue())

			switch *node.Action {
			case ictrltypes.CREATE:
				err := cli.Create(ctx, node.Obj)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return err
				}
			case ictrltypes.UPDATE:
				if node.Immutable {
					return nil
				}
				err := cli.Update(ctx, node.Obj)
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			case ictrltypes.PATCH:
				patch := client.MergeFrom(node.ObjCopy)
				if err := cli.Patch(ctx, node.Obj, patch); err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			case ictrltypes.DELETE:
				if controllerutil.RemoveFinalizer(node.Obj, constant.DBClusterFinalizerName) {
					err := cli.Update(ctx, node.Obj)
					if err != nil && !apierrors.IsNotFound(err) {
						return err
					}
				}
				if _, ok := node.Obj.(*appsv1alpha1.Cluster); !ok {
					err := cli.Delete(ctx, node.Obj)
					if err != nil && !apierrors.IsNotFound(err) {
						return err
					}
				}
			case ictrltypes.STATUS:
				patch := client.MergeFrom(node.ObjCopy)
				if err := cli.Status().Patch(ctx, node.Obj, patch); err != nil {
					return err
				}
			case ictrltypes.NOOP:
				// nothing
			}
			return nil
		}
		return dag.WalkReverseTopoOrder(walking, nil)
	}

	newComponent := func(compName string) Component {
		comp, err := NewComponent(reqCtx, testCtx.Cli, clusterDefObj, clusterVerObj, clusterObj, compName, dag)
		Expect(comp).ShouldNot(BeNil())
		Expect(err).Should(Succeed())
		return comp
	}

	createComponent := func(comp Component) error {
		Expect(comp.Create(reqCtx, testCtx.Cli)).Should(Succeed())
		return submitChanges(testCtx.Ctx, testCtx.Cli, dag)
	}

	deleteComponent := func(comp Component) error {
		resetDag(comp)
		Expect(comp.Update(reqCtx, testCtx.Cli)).Should(Succeed())
		return submitChanges(testCtx.Ctx, testCtx.Cli, dag)
	}

	updateComponent := func(comp Component) error {
		resetDag(comp)
		Expect(comp.Update(reqCtx, testCtx.Cli)).Should(Succeed())
		return submitChanges(testCtx.Ctx, testCtx.Cli, dag)
	}

	statusComponent := func(comp Component) error {
		resetDag(comp)
		Expect(comp.Status(reqCtx, testCtx.Cli)).Should(Succeed())
		return submitChanges(testCtx.Ctx, testCtx.Cli, dag)
	}

	Context("new component object", func() {
		BeforeEach(func() {
			setup()
		})

		It("normally", func() {
			By("create cluster component normally")
			comp := newComponent(statefulCompName)
			Expect(comp.GetNamespace()).Should(Equal(clusterObj.GetNamespace()))
			Expect(comp.GetClusterName()).Should(Equal(clusterObj.GetName()))
			Expect(comp.GetName()).Should(Equal(statefulCompName))
			Expect(comp.GetCluster()).Should(Equal(clusterObj))
			Expect(comp.GetClusterVersion()).Should(Equal(clusterVerObj))
			Expect(comp.GetSynthesizedComponent()).ShouldNot(BeNil())
		})

		It("w/o component definition", func() {
			By("create cluster without component definition")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVerObj.Name).
				AddComponent(statefulCompName, statefulCompDefName+random). // with a random component def name
				GetObject()
			_, err := NewComponent(reqCtx, testCtx.Cli, clusterDefObj, clusterVerObj, clusterObj, statefulCompName, dag)
			Expect(err).ShouldNot(Succeed())
		})

		It("w/o component definition and spec", func() {
			By("create cluster without component definition and spec")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVerObj.Name).
				AddComponent(statefulCompName+random, statefulCompDefName+random). // with a random component spec and def name
				GetObject()
			comp, err := NewComponent(reqCtx, testCtx.Cli, clusterDefObj, clusterVerObj, clusterObj, statefulCompName, dag)
			Expect(comp).Should(BeNil())
			Expect(err).Should(BeNil())
		})
	})

	Context("create and delete component", func() {
		var (
			comp   Component
			labels client.MatchingLabels
		)

		BeforeEach(func() {
			setup()

			comp = newComponent(statefulCompName)
			Expect(createComponent(comp)).Should(Succeed())

			labels = client.MatchingLabels{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    comp.GetClusterName(),
				constant.KBAppComponentLabelKey: comp.GetName(),
			}
		})

		It("create component resources", func() {
			By("check workload resources created")
			Eventually(testapps.List(&testCtx, generics.StatefulSetSignature, labels)).Should(HaveLen(1))
		})

		It("delete component doesn't affect resources", func() {
			By("delete the component")
			Expect(deleteComponent(comp)).Should(Succeed())

			By("check workload resources still exist")
			Eventually(testapps.List(&testCtx, generics.StatefulSetSignature, labels)).Should(HaveLen(1))
		})
	})

	Context("update component", func() {
		var (
			comp   Component
			labels client.MatchingLabels
		)

		spec := func() *appsv1alpha1.ClusterComponentSpec {
			return clusterObj.Spec.GetComponentByName(comp.GetName())
		}

		BeforeEach(func() {
			setup()

			comp = newComponent(statefulCompName)
			Expect(createComponent(comp)).Should(Succeed())

			labels = client.MatchingLabels{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    comp.GetClusterName(),
				constant.KBAppComponentLabelKey: comp.GetName(),
			}
		})

		It("update w/o changes", func() {
			By("update without change")
			Expect(updateComponent(comp)).Should(Succeed())

			By("check the workload not updated")
			stsKey := types.NamespacedName{
				Namespace: comp.GetNamespace(),
				Name:      fmt.Sprintf("%s-%s", comp.GetClusterName(), comp.GetName()),
			}
			Eventually(testapps.CheckObj(&testCtx, stsKey, func(g Gomega, sts *appsv1.StatefulSet) {
				g.Expect(sts.GetGeneration()).Should(Equal(int64(1)))
			})).Should(Succeed())
		})

		It("scale out", func() {
			By("scale out replicas with 1")
			spec().Replicas = spec().Replicas + 1
			Expect(updateComponent(comp)).Should(Succeed())

			By("check the workload updated")
			stsKey := types.NamespacedName{
				Namespace: comp.GetNamespace(),
				Name:      fmt.Sprintf("%s-%s", comp.GetClusterName(), comp.GetName()),
			}
			Eventually(testapps.CheckObj(&testCtx, stsKey, func(g Gomega, sts *appsv1.StatefulSet) {
				g.Expect(sts.GetGeneration()).Should(Equal(int64(2)))
				g.Expect(*sts.Spec.Replicas).Should(Equal(spec().Replicas))
			})).Should(Succeed())
			Eventually(testapps.List(&testCtx, generics.PodSignature, labels)).Should(HaveLen(int(spec().Replicas)))
			Eventually(testapps.List(&testCtx, generics.PersistentVolumeClaimSignature, labels)).Should(HaveLen(int(spec().Replicas)))
		})

		It("scale out - TODO", func() {
		})

		It("scale in", func() {
			By("scale in replicas with 1")
			Expect(spec().Replicas > 1).Should(BeTrue())
			spec().Replicas = spec().Replicas - 1
			Expect(updateComponent(comp)).Should(Succeed())

			By("check the workload updated")
			stsKey := types.NamespacedName{
				Namespace: comp.GetNamespace(),
				Name:      fmt.Sprintf("%s-%s", comp.GetClusterName(), comp.GetName()),
			}
			Eventually(testapps.CheckObj(&testCtx, stsKey, func(g Gomega, sts *appsv1.StatefulSet) {
				g.Expect(sts.GetGeneration()).Should(Equal(int64(2)))
				g.Expect(*sts.Spec.Replicas).Should(Equal(spec().Replicas))
			})).Should(Succeed())
			Eventually(testapps.List(&testCtx, generics.PodSignature, labels)).Should(HaveLen(int(spec().Replicas)))

			By("check the PVC deleted")
			Eventually(testapps.List(&testCtx, generics.PersistentVolumeClaimSignature, labels)).Should(HaveLen(int(spec().Replicas)))
		})

		It("scale in to zero", func() {
			By("scale in replicas to 0")
			replicas := spec().Replicas
			spec().Replicas = 0
			Expect(updateComponent(comp)).Should(Succeed())

			By("check the workload updated")
			stsKey := types.NamespacedName{
				Namespace: comp.GetNamespace(),
				Name:      fmt.Sprintf("%s-%s", comp.GetClusterName(), comp.GetName()),
			}
			Eventually(testapps.CheckObjExists(&testCtx, stsKey, &appsv1.StatefulSet{}, false)).Should(Succeed())

			By("check all the PVCs kept")
			Eventually(testapps.List(&testCtx, generics.PersistentVolumeClaimSignature, labels)).Should(HaveLen(int(replicas)))
		})

		It("scale up", func() {
			By("scale up cpu & mem")
			// spec().Resources
			Expect(updateComponent(comp)).Should(Succeed())

			By("check the workload updated")
			stsKey := types.NamespacedName{
				Namespace: comp.GetNamespace(),
				Name:      fmt.Sprintf("%s-%s", comp.GetClusterName(), comp.GetName()),
			}
			Eventually(testapps.CheckObj(&testCtx, stsKey, func(g Gomega, sts *appsv1.StatefulSet) {
				g.Expect(sts.GetGeneration()).Should(Equal(int64(2)))
				// TODO
			})).Should(Succeed())
		})

		It("scale up out-of-limit", func() {
			Expect(updateComponent(comp)).Should(Succeed())
		})

		It("scale down", func() {
			Expect(updateComponent(comp)).Should(Succeed())
		})

		It("volume expansion", func() {
			By("up the volume size with 1Gi")
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Add(apiresource.MustParse("1Gi"))
			Expect(updateComponent(comp)).Should(Succeed())

			// TODO: check the volume
		})

		It("volume expansion out-of-limit", func() {
			By("set the volume size as out-of-limit")
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Add(apiresource.MustParse("128Ti"))
			Expect(updateComponent(comp)).Should(HaveOccurred())

			// TODO: check the volume as unchanged
		})

		It("volume shrink", func() {
			By("down the volume size with 1Gi")
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Sub(apiresource.MustParse("1Gi"))
			Expect(updateComponent(comp)).Should(HaveOccurred())

			// TODO: check the volume as unchanged
		})

		It("general workload update", func() {
			Expect(updateComponent(comp)).Should(Succeed())
		})

		It("kb system images update", func() {
			Expect(updateComponent(comp)).Should(Succeed())
		})

		It("update strategy", func() {
			Expect(updateComponent(comp)).Should(Succeed())
		})
	})

	Context("status component", func() {
		var (
			comp Component
			// labels client.MatchingLabels
		)

		BeforeEach(func() {
			setup()

			comp = newComponent(statefulCompName)
			Expect(createComponent(comp)).Should(Succeed())

			// labels = client.MatchingLabels{
			//	constant.AppManagedByLabelKey:   constant.AppName,
			//	constant.AppInstanceLabelKey:    comp.GetClusterName(),
			//	constant.KBAppComponentLabelKey: comp.GetName(),
			// }
		})

		It("provisioning", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("provisioning with temporary error", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("pods not ready", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("pods ready, has no role", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("pods ready, role probe timed-out", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("all pods are ok", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("updating", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("updating with conditions", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("updating with temporary error", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})

		It("passively error", func() {
			Expect(statusComponent(comp)).Should(Succeed())
		})
	})
})
