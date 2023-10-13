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
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Component", func() {
	const (
		statefulCompName    = "stateful"
		statefulCompDefName = "stateful"
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

		defaultStorageClass *storagev1.StorageClass

		defaultReplicas       = 2
		defaultVolumeSize     = "2Gi"
		defaultVolumeQuantity = apiresource.MustParse(defaultVolumeSize)
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RSMSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	newDAGWithPlaceholder := func(namespace, clusterName, compName string) *graph.DAG {
		root := builder.NewReplicatedStateMachineBuilder(namespace, fmt.Sprintf("%s-%s", clusterName, compName)).GetObject()
		dag := graph.NewDAG()
		model.NewGraphClient(nil).Root(dag, nil, root, nil)
		return dag
	}

	setup := func() {
		defaultStorageClass = testk8s.CreateMockStorageClass(&testCtx, testk8s.DefaultStorageClassName)
		Expect(*defaultStorageClass.AllowVolumeExpansion).Should(BeTrue())

		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
			GetObject()

		clusterVerObj = testapps.NewClusterVersionFactory(clusterVerName, clusterDefObj.GetName()).
			AddComponentVersion(statefulCompDefName).AddContainerShort(testapps.DefaultMySQLContainerName, testapps.ApeCloudMySQLImage).
			GetObject()

		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVerObj.Name).
			AddComponent(statefulCompName, statefulCompDefName).
			SetReplicas(int32(defaultReplicas)).
			AddVolumeClaimTemplate(testapps.LogVolumeName, testapps.NewPVCSpec(defaultVolumeSize)).
			AddVolumeClaimTemplate(testapps.DataVolumeName, testapps.NewPVCSpec(defaultVolumeSize)).
			GetObject()

		reqCtx = intctrlutil.RequestCtx{Ctx: ctx, Log: logger, Recorder: recorder}
		dag = newDAGWithPlaceholder(clusterObj.Namespace, clusterName, statefulCompName)
	}

	resetDag := func(comp Component) {
		Expect(comp).ShouldNot(BeNil())
		rsmComp, ok := comp.(*rsmComponent)
		Expect(ok).Should(BeTrue())
		rsmComp.dag = newDAGWithPlaceholder(clusterObj.Namespace, clusterName, rsmComp.GetName())
	}

	submitChanges := func(ctx context.Context, cli client.Client, dag *graph.DAG) error {
		walking := func(v graph.Vertex) error {
			node, ok := v.(*model.ObjectVertex)
			Expect(ok).Should(BeTrue())

			_, ok = node.Obj.(*appsv1alpha1.Cluster)
			Expect(!ok || *node.Action == model.NOOP).Should(BeTrue())

			switch *node.Action {
			case model.CREATE:
				err := cli.Create(ctx, node.Obj)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return err
				}
			case model.UPDATE:
				err := cli.Update(ctx, node.Obj)
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			case model.PATCH:
				patch := client.MergeFrom(node.OriObj)
				err := cli.Patch(ctx, node.Obj, patch)
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			case model.DELETE:
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
			case model.STATUS:
				patch := client.MergeFrom(node.OriObj)
				if err := cli.Status().Patch(ctx, node.Obj, patch); err != nil {
					return err
				}
			case model.NOOP:
				// nothing
			}
			return nil
		}
		if dag.Root() != nil {
			return dag.WalkReverseTopoOrder(walking, nil)
		} else {
			withRoot := graph.NewDAG()
			model.NewGraphClient(cli).Root(withRoot, nil, &appsv1alpha1.Cluster{}, model.ActionNoopPtr())
			withRoot.Merge(dag)
			return withRoot.WalkReverseTopoOrder(walking, nil)
		}
	}

	newComponent := func(compName string) Component {
		comp, err := NewComponent(reqCtx, testCtx.Cli, clusterDefObj, clusterVerObj, clusterObj, compName, dag)
		Expect(comp).ShouldNot(BeNil())
		Expect(err).Should(Succeed())
		return comp
	}

	createComponent := func(comp Component) error {
		if err := comp.Create(reqCtx, testCtx.Cli); err != nil {
			return err
		}
		return submitChanges(testCtx.Ctx, testCtx.Cli, dag)
	}

	deleteComponent := func(comp Component) error {
		resetDag(comp)
		if err := comp.Delete(reqCtx, testCtx.Cli); err != nil {
			return err
		}
		return submitChanges(testCtx.Ctx, testCtx.Cli, dag)
	}

	updateComponent := func(comp Component) error {
		resetDag(comp)
		comp.GetCluster().Generation = comp.GetCluster().Status.ObservedGeneration + 1
		if err := comp.Update(reqCtx, testCtx.Cli); err != nil {
			return err
		}
		return submitChanges(testCtx.Ctx, testCtx.Cli, dag)
	}

	pvcKey := func(clusterName, compName, vctName string, ordinal int) types.NamespacedName {
		return types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      fmt.Sprintf("%s-%s-%s-%d", vctName, clusterName, compName, ordinal),
		}
	}

	pvKey := func(clusterName, compName, vctName string, ordinal int) types.NamespacedName {
		return types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      fmt.Sprintf("pvc-%s-%s-%s-%d", clusterName, compName, vctName, ordinal),
		}
	}

	createPVCs := func(spec *appsv1alpha1.ClusterComponentSpec, labels client.MatchingLabels) {
		for _, vct := range spec.VolumeClaimTemplates {
			for i := 0; i < int(spec.Replicas); i++ {
				var (
					pvcName = pvcKey(clusterName, spec.Name, vct.Name, i).Name
					pvName  = pvKey(clusterName, spec.Name, vct.Name, i).Name
				)
				pvc := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName, spec.Name, vct.Name).
					AddLabelsInMap(labels).
					SetStorageClass(defaultStorageClass.Name).
					SetStorage(defaultVolumeSize).
					SetVolumeName(pvName).
					CheckedCreate(&testCtx).
					GetObject()
				testapps.NewPersistentVolumeFactory(testCtx.DefaultNamespace, pvName, pvcName).
					SetStorage(defaultVolumeSize).
					SetClaimRef(pvc).
					CheckedCreate(&testCtx)
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey(clusterName, spec.Name, vct.Name, i),
					func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Status.Phase = corev1.ClaimBound
						if pvc.Status.Capacity == nil {
							pvc.Status.Capacity = corev1.ResourceList{}
						}
						pvc.Status.Capacity[corev1.ResourceStorage] = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
					})).Should(Succeed())
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvKey(clusterName, spec.Name, vct.Name, i),
					func(pv *corev1.PersistentVolume) {
						pvc.Status.Phase = corev1.ClaimBound
					})).Should(Succeed())
			}
		}
	}

	Context("new component object", func() {
		BeforeEach(func() {
			setup()
		})

		It("ok", func() {
			By("new cluster component ok")
			comp := newComponent(statefulCompName)
			Expect(comp.GetNamespace()).Should(Equal(clusterObj.GetNamespace()))
			Expect(comp.GetClusterName()).Should(Equal(clusterObj.GetName()))
			Expect(comp.GetName()).Should(Equal(statefulCompName))
			Expect(comp.GetCluster()).Should(Equal(clusterObj))
			Expect(comp.GetClusterVersion()).Should(Equal(clusterVerObj))
			Expect(comp.GetSynthesizedComponent()).ShouldNot(BeNil())
		})

		It("w/o component definition", func() {
			By("new cluster component without component definition")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVerObj.Name).
				AddComponent(statefulCompName, statefulCompDefName+random). // with a random component def name
				GetObject()
			_, err := NewComponent(reqCtx, testCtx.Cli, clusterDefObj, clusterVerObj, clusterObj, statefulCompName, dag)
			Expect(err).ShouldNot(Succeed())
		})

		It("w/o component definition and spec", func() {
			By("new cluster component without component definition and spec")
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
			Eventually(testapps.List(&testCtx, generics.RSMSignature, labels)).Should(HaveLen(1))
		})

		It("delete component doesn't affect resources", func() {
			By("delete the component")
			Expect(deleteComponent(comp)).Should(Succeed())

			By("check workload resources still exist")
			Eventually(testapps.List(&testCtx, generics.RSMSignature, labels)).Should(HaveLen(1))
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

		// rsmKey := func() types.NamespacedName {
		//	return types.NamespacedName{
		//		Namespace: comp.GetNamespace(),
		//		Name:      fmt.Sprintf("%s-%s", clusterObj.GetName(), comp.GetName()),
		//	}
		// }

		BeforeEach(func() {
			setup()

			comp = newComponent(statefulCompName)
			Expect(createComponent(comp)).Should(Succeed())

			labels = client.MatchingLabels{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    comp.GetClusterName(),
				constant.KBAppComponentLabelKey: comp.GetName(),
			}

			// create all PVCs ann PVs
			createPVCs(spec(), labels)
		})

		It("expand volume", func() {
			By("up the log volume size with 1Gi")
			vct := spec().VolumeClaimTemplates[0]
			quantity := vct.Spec.Resources.Requests.Storage()
			quantity.Add(apiresource.MustParse("1Gi"))
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = *quantity
			Expect(updateComponent(comp)).Should(Succeed())

			By("check all the log PVCs updated")
			Eventually(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli, generics.PersistentVolumeClaimSignature, comp.GetNamespace(), labels)
				g.Expect(err).Should(Succeed())
				g.Expect(objs).Should(HaveLen(int(spec().Replicas) * len(spec().VolumeClaimTemplates)))
				for _, pvc := range objs {
					if strings.HasPrefix(pvc.GetName(), vct.Name) {
						g.Expect(pvc.Spec.Resources.Requests.Storage().Cmp(defaultVolumeQuantity)).Should(Equal(1))
					} else {
						g.Expect(pvc.Spec.Resources.Requests.Storage().Cmp(defaultVolumeQuantity)).Should(Equal(0))
					}
				}
			}).Should(Succeed())
		})

		It("shrink volume", func() {
			By("shrink the log volume with 1Gi")
			quantity := spec().VolumeClaimTemplates[0].Spec.Resources.Requests.Storage()
			quantity.Sub(apiresource.MustParse("1Gi"))
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = *quantity
			Expect(updateComponent(comp)).Should(HaveOccurred())

			By("check all the PVCs unchanged")
			Consistently(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli, generics.PersistentVolumeClaimSignature, comp.GetNamespace(), labels)
				g.Expect(err).Should(Succeed())
				g.Expect(objs).Should(HaveLen(int(spec().Replicas) * len(spec().VolumeClaimTemplates)))
				for _, pvc := range objs {
					g.Expect(pvc.Spec.Resources.Requests.Storage().Cmp(defaultVolumeQuantity)).Should(Equal(0))
				}
			}).Should(Succeed())
		})

		It("recover volume size during expansion", func() {
			By("up the log volume size with 1Gi")
			vct := spec().VolumeClaimTemplates[0]
			quantity := vct.Spec.Resources.Requests.Storage()
			quantity.Add(apiresource.MustParse("1Gi"))
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = *quantity
			Expect(updateComponent(comp)).Should(Succeed())

			By("check all the log PVCs updating")
			Eventually(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli, generics.PersistentVolumeClaimSignature, comp.GetNamespace(), labels)
				g.Expect(err).Should(Succeed())
				g.Expect(objs).Should(HaveLen(int(spec().Replicas) * len(spec().VolumeClaimTemplates)))
				for _, pvc := range objs {
					if strings.HasPrefix(pvc.GetName(), vct.Name) {
						g.Expect(pvc.Spec.Resources.Requests.Storage().Cmp(*pvc.Status.Capacity.Storage())).Should(Equal(1))
					} else {
						g.Expect(pvc.Spec.Resources.Requests.Storage().Cmp(*pvc.Status.Capacity.Storage())).Should(Equal(0))
					}
				}
			}).Should(Succeed())

			By("reset the log volumes as original size")
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = defaultVolumeQuantity
			Expect(updateComponent(comp)).Should(Succeed())

			By("check all the PVCs rolled-back")
			Eventually(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli, generics.PersistentVolumeClaimSignature, comp.GetNamespace(), labels)
				g.Expect(err).Should(Succeed())
				g.Expect(objs).Should(HaveLen(int(spec().Replicas) * len(spec().VolumeClaimTemplates)))
				for _, pvc := range objs {
					g.Expect(pvc.Spec.Resources.Requests.Storage().Cmp(defaultVolumeQuantity)).Should(Equal(0))
				}
			}).Should(Succeed())
		})
	})
})
