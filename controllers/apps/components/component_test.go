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
	"fmt"
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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
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
	}

	labels := func() client.MatchingLabels {
		return client.MatchingLabels{
			constant.AppManagedByLabelKey:   constant.AppName,
			constant.AppInstanceLabelKey:    clusterObj.Name,
			constant.KBAppComponentLabelKey: statefulCompName,
		}
	}

	spec := func() *appsv1alpha1.ClusterComponentSpec {
		for i, v := range clusterObj.Spec.ComponentSpecs {
			if v.Name == statefulCompName {
				return &clusterObj.Spec.ComponentSpecs[i]
			}
		}
		return nil
	}

	status := func() appsv1alpha1.ClusterComponentStatus {
		return clusterObj.Status.Components[statefulCompName]
	}

	rsmKey := func() types.NamespacedName {
		return types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      fmt.Sprintf("%s-%s", clusterObj.GetName(), statefulCompName),
		}
	}

	applyChanges := func(dag *graph.DAG) error {
		walking := func(v graph.Vertex) error {
			node, ok := v.(*ictrltypes.LifecycleVertex)
			Expect(ok).Should(BeTrue())

			_, ok = node.Obj.(*appsv1alpha1.Cluster)
			Expect(!ok || *node.Action == ictrltypes.NOOP).Should(BeTrue())

			switch *node.Action {
			case ictrltypes.CREATE:
				controllerutil.AddFinalizer(node.Obj, constant.DBClusterFinalizerName)
				err := testCtx.Create(ctx, node.Obj)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					return err
				}
			case ictrltypes.UPDATE:
				if node.Immutable {
					return nil
				}
				err := testCtx.Cli.Update(ctx, node.Obj)
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			case ictrltypes.PATCH:
				patch := client.MergeFrom(node.ObjCopy)
				err := testCtx.Cli.Patch(ctx, node.Obj, patch)
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			case ictrltypes.DELETE:
				if controllerutil.RemoveFinalizer(node.Obj, constant.DBClusterFinalizerName) {
					err := testCtx.Cli.Update(ctx, node.Obj)
					if err != nil && !apierrors.IsNotFound(err) {
						return err
					}
				}
				if _, ok := node.Obj.(*appsv1alpha1.Cluster); !ok {
					err := testCtx.Cli.Delete(ctx, node.Obj)
					if err != nil && !apierrors.IsNotFound(err) {
						return err
					}
				}
			case ictrltypes.STATUS:
				patch := client.MergeFrom(node.ObjCopy)
				if err := testCtx.Cli.Status().Patch(ctx, node.Obj, patch); err != nil {
					return err
				}
			case ictrltypes.NOOP:
				// nothing
			}
			return nil
		}
		if dag.Root() != nil {
			return dag.WalkReverseTopoOrder(walking, nil)
		} else {
			withRoot := graph.NewDAG()
			ictrltypes.LifecycleObjectNoop(withRoot, &appsv1alpha1.Cluster{}, nil)
			withRoot.Merge(dag)
			return withRoot.WalkReverseTopoOrder(walking, nil)
		}
	}

	newComponent := func(compName string) (Component, *graph.DAG) {
		dag := graph.NewDAG()
		comp, err := NewComponent(reqCtx, testCtx.Cli, clusterDefObj, clusterVerObj, clusterObj, compName, dag)
		Expect(comp).ShouldNot(BeNil())
		Expect(err).Should(Succeed())
		return comp, dag
	}

	createComponent := func() error {
		comp, dag := newComponent(statefulCompName)
		if err := comp.Create(reqCtx, testCtx.Cli); err != nil {
			return err
		}
		return applyChanges(dag)
	}

	deleteComponent := func() error {
		comp, dag := newComponent(statefulCompName)
		if err := comp.Delete(reqCtx, testCtx.Cli); err != nil {
			return err
		}
		return applyChanges(dag)
	}

	updateComponent := func() error {
		comp, dag := newComponent(statefulCompName)
		comp.GetCluster().Generation = comp.GetCluster().Status.ObservedGeneration + 1
		if err := comp.Update(reqCtx, testCtx.Cli); err != nil {
			return err
		}
		return applyChanges(dag)
	}

	retryUpdateComponent := func() error {
		comp, dag := newComponent(statefulCompName)
		// don't update the cluster generation
		if err := comp.Update(reqCtx, testCtx.Cli); err != nil {
			return err
		}
		return applyChanges(dag)
	}

	statusComponent := func() error {
		comp, dag := newComponent(statefulCompName)
		comp.GetCluster().Status.ObservedGeneration = comp.GetCluster().Generation
		if err := comp.Status(reqCtx, testCtx.Cli); err != nil {
			return err
		}
		return applyChanges(dag)
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

	createPVCs := func() {
		for _, vct := range spec().VolumeClaimTemplates {
			for i := 0; i < int(spec().Replicas); i++ {
				var (
					pvcName = pvcKey(clusterName, statefulCompName, vct.Name, i).Name
					pvName  = pvKey(clusterName, statefulCompName, vct.Name, i).Name
				)
				pvc := testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName, statefulCompName, vct.Name).
					AddLabelsInMap(labels()).
					SetStorageClass(defaultStorageClass.Name).
					SetStorage(defaultVolumeSize).
					SetVolumeName(pvName).
					CheckedCreate(&testCtx).
					GetObject()
				testapps.NewPersistentVolumeFactory(testCtx.DefaultNamespace, pvName, pvcName).
					SetStorage(defaultVolumeSize).
					SetClaimRef(pvc).
					CheckedCreate(&testCtx)
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey(clusterName, statefulCompName, vct.Name, i),
					func(pvc *corev1.PersistentVolumeClaim) {
						pvc.Status.Phase = corev1.ClaimBound
						if pvc.Status.Capacity == nil {
							pvc.Status.Capacity = corev1.ResourceList{}
						}
						pvc.Status.Capacity[corev1.ResourceStorage] = pvc.Spec.Resources.Requests[corev1.ResourceStorage]
					})).Should(Succeed())
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvKey(clusterName, statefulCompName, vct.Name, i),
					func(pv *corev1.PersistentVolume) {
						pvc.Status.Phase = corev1.ClaimBound
					})).Should(Succeed())
			}
		}
	}

	createPods := func() {
		for i := 0; i < int(spec().Replicas); i++ {
			// TODO
		}
	}

	Context("new component object", func() {
		BeforeEach(func() {
			setup()
		})

		It("ok", func() {
			By("new cluster component ok")
			comp, _ := newComponent(statefulCompName)
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
			_, err := NewComponent(reqCtx, testCtx.Cli, clusterDefObj, clusterVerObj, clusterObj, statefulCompName, graph.NewDAG())
			Expect(err).ShouldNot(Succeed())
		})

		It("w/o component definition and spec", func() {
			By("new cluster component without component definition and spec")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefObj.Name, clusterVerObj.Name).
				AddComponent(statefulCompName+random, statefulCompDefName+random). // with a random component spec and def name
				GetObject()
			comp, err := NewComponent(reqCtx, testCtx.Cli, clusterDefObj, clusterVerObj, clusterObj, statefulCompName, graph.NewDAG())
			Expect(comp).Should(BeNil())
			Expect(err).Should(BeNil())
		})
	})

	Context("create and delete component", func() {
		BeforeEach(func() {
			setup()
			Expect(createComponent()).Should(Succeed())
		})

		It("create component resources", func() {
			By("check workload resources created")
			Eventually(testapps.List(&testCtx, generics.RSMSignature, labels())).Should(HaveLen(1))
		})

		It("delete component doesn't affect resources", func() {
			By("delete the component")
			Expect(deleteComponent()).Should(Succeed())

			By("check workload resources still exist")
			Eventually(testapps.List(&testCtx, generics.RSMSignature, labels())).Should(HaveLen(1))
		})
	})

	Context("update component", func() {
		BeforeEach(func() {
			setup()

			Expect(createComponent()).Should(Succeed())

			// create all PVCs ann PVs
			createPVCs()
		})

		It("update w/o changes", func() {
			By("update without change")
			Expect(updateComponent()).Should(Succeed())

			By("check the workload not updated")
			Consistently(testapps.CheckObj(&testCtx, rsmKey(), func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
				g.Expect(rsm.GetGeneration()).Should(Equal(int64(1)))
			})).Should(Succeed())
		})

		It("scale out", func() {
			By("scale out replicas with 1")
			replicas := spec().Replicas
			spec().Replicas = spec().Replicas + 1

			By("update to create new PVCs, the workload not updated")
			// since we don't set backup policy, the dummy clone policy will be used
			Expect(updateComponent()).Should(Succeed())
			Consistently(testapps.CheckObj(&testCtx, rsmKey(), func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
				g.Expect(rsm.GetGeneration()).Should(Equal(int64(1)))
				g.Expect(*rsm.Spec.Replicas).Should(Equal(replicas))
			})).Should(Succeed())
			expectedCnt := int(spec().Replicas) * len(spec().VolumeClaimTemplates)
			Eventually(testapps.List(&testCtx, generics.PersistentVolumeClaimSignature, labels())).Should(HaveLen(expectedCnt))

			By("update again to apply changes to the workload")
			Expect(retryUpdateComponent()).Should(Succeed())

			By("check the workload updated")
			Eventually(testapps.CheckObj(&testCtx, rsmKey(), func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
				g.Expect(rsm.GetGeneration()).Should(Equal(int64(2)))
				g.Expect(*rsm.Spec.Replicas).Should(Equal(spec().Replicas))
			})).Should(Succeed())
		})

		It("TODO - scale out out-of-range", func() {
		})

		It("scale in", func() {
			By("scale in replicas with 1")
			Expect(spec().Replicas > 1).Should(BeTrue())
			replicas := spec().Replicas
			spec().Replicas = spec().Replicas - 1
			Expect(updateComponent()).Should(Succeed())

			By("check the workload updated")
			Eventually(testapps.CheckObj(&testCtx, rsmKey(), func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
				g.Expect(rsm.GetGeneration()).Should(Equal(int64(2)))
				g.Expect(*rsm.Spec.Replicas).Should(Equal(spec().Replicas))
			})).Should(Succeed())

			By("check the PVC logically deleted")
			Eventually(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli,
					generics.PersistentVolumeClaimSignature, testCtx.DefaultNamespace, labels())
				g.Expect(err).Should(Succeed())
				g.Expect(objs).Should(HaveLen(int(replicas) * len(spec().VolumeClaimTemplates)))
				for _, pvc := range objs {
					if strings.HasSuffix(pvc.GetName(), fmt.Sprintf("-%d", replicas-1)) {
						g.Expect(pvc.GetDeletionTimestamp()).ShouldNot(BeNil())
					} else {
						g.Expect(pvc.GetDeletionTimestamp()).Should(BeNil())
					}
				}
			}).Should(Succeed())
		})

		It("scale in to zero", func() {
			By("scale in replicas to 0")
			replicas := spec().Replicas
			spec().Replicas = 0
			Expect(updateComponent()).Should(Succeed())

			By("check the workload updated")
			Eventually(testapps.CheckObj(&testCtx, rsmKey(), func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
				g.Expect(rsm.GetGeneration()).Should(Equal(int64(2)))
				g.Expect(*rsm.Spec.Replicas).Should(Equal(spec().Replicas))
			})).Should(Succeed())

			By("check all the PVCs unchanged")
			Consistently(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli,
					generics.PersistentVolumeClaimSignature, testCtx.DefaultNamespace, labels())
				g.Expect(err).Should(Succeed())
				g.Expect(objs).Should(HaveLen(int(replicas) * len(spec().VolumeClaimTemplates)))
				for _, pvc := range objs {
					g.Expect(pvc.GetDeletionTimestamp()).Should(BeNil())
				}
			}).Should(Succeed())
		})

		It("TODO - scale up", func() {
		})

		It("TODO - scale up out-of-limit", func() {
		})

		It("TODO - scale down", func() {
		})

		It("expand volume", func() {
			By("up the log volume size with 1Gi")
			vct := spec().VolumeClaimTemplates[0]
			quantity := vct.Spec.Resources.Requests.Storage()
			quantity.Add(apiresource.MustParse("1Gi"))
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = *quantity
			Expect(updateComponent()).Should(Succeed())

			By("check all the log PVCs updated")
			Eventually(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli,
					generics.PersistentVolumeClaimSignature, testCtx.DefaultNamespace, labels())
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
			Expect(updateComponent()).Should(HaveOccurred())

			By("check all the PVCs unchanged")
			Consistently(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli,
					generics.PersistentVolumeClaimSignature, testCtx.DefaultNamespace, labels())
				g.Expect(err).Should(Succeed())
				g.Expect(objs).Should(HaveLen(int(spec().Replicas) * len(spec().VolumeClaimTemplates)))
				for _, pvc := range objs {
					g.Expect(pvc.Spec.Resources.Requests.Storage().Cmp(defaultVolumeQuantity)).Should(Equal(0))
				}
			}).Should(Succeed())
		})

		It("rollback volume size during expansion", func() {
			By("up the log volume size with 1Gi")
			vct := spec().VolumeClaimTemplates[0]
			quantity := vct.Spec.Resources.Requests.Storage()
			quantity.Add(apiresource.MustParse("1Gi"))
			spec().VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = *quantity
			Expect(updateComponent()).Should(Succeed())

			By("check all the log PVCs updating")
			Eventually(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli,
					generics.PersistentVolumeClaimSignature, testCtx.DefaultNamespace, labels())
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
			Expect(updateComponent()).Should(Succeed())

			By("check all the PVCs rolled-back")
			Eventually(func(g Gomega) {
				objs, err := listObjWithLabelsInNamespace(testCtx.Ctx, testCtx.Cli,
					generics.PersistentVolumeClaimSignature, testCtx.DefaultNamespace, labels())
				g.Expect(err).Should(Succeed())
				g.Expect(objs).Should(HaveLen(int(spec().Replicas) * len(spec().VolumeClaimTemplates)))
				for _, pvc := range objs {
					g.Expect(pvc.Spec.Resources.Requests.Storage().Cmp(defaultVolumeQuantity)).Should(Equal(0))
				}
			}).Should(Succeed())
		})

		It("TODO- rollback volume size during expansion - recreate PVC error", func() {
		})

		It("TODO- general workload update", func() {
			Expect(updateComponent()).Should(Succeed())
		})

		It("TODO- KB system images update", func() {
			Expect(updateComponent()).Should(Succeed())
		})

		It("TODO- update strategy", func() {
			Expect(updateComponent()).Should(Succeed())
		})
	})

	Context("status component", func() {
		BeforeEach(func() {
			setup()

			Expect(createComponent()).Should(Succeed())

			// create all PVCs ann PVs
			createPVCs()

			// create all Pods
			createPods()
		})

		It("provisioning", func() {
			By("status component")
			Expect(statusComponent()).Should(Succeed())

			By("check component status as CREATING")
			Expect(status().Phase).Should(Equal(appsv1alpha1.CreatingClusterCompPhase))
		})

		It("TODO - provisioning with temporary error", func() {
			By("some pods have temporary failure")

			By("status component")
			Expect(statusComponent()).Should(Succeed())

			By("check component status as CREATING")
			Expect(status().Phase).Should(Equal(appsv1alpha1.CreatingClusterCompPhase))
		})

		It("TODO - pods not ready", func() {
			Expect(statusComponent()).Should(Succeed())
		})

		It("pods ready, has no role", func() {
			Expect(statusComponent()).Should(Succeed())
		})

		It("TODO - pods ready, role probe timed-out", func() {
			Expect(statusComponent()).Should(Succeed())
		})

		It("TODO - all pods are ok", func() {
			Expect(statusComponent()).Should(Succeed())
		})

		It("TODO - updating", func() {
			Expect(statusComponent()).Should(Succeed())
		})

		It("TODO - updating with conditions", func() {
			Expect(statusComponent()).Should(Succeed())
		})

		It("TODO - updating with temporary error", func() {
			Expect(statusComponent()).Should(Succeed())
		})

		It("deleting", func() {
			By("delete underlying workload w/o removing finalizers")
			rsm := &workloads.ReplicatedStateMachine{}
			Expect(testCtx.Cli.Get(testCtx.Ctx, rsmKey(), rsm)).Should(Succeed())
			Expect(testCtx.Cli.Delete(testCtx.Ctx, rsm)).Should(Succeed())

			By("status component")
			Expect(statusComponent()).Should(Succeed())

			By("check component status as DELETING")
			Expect(status().Phase).Should(Equal(appsv1alpha1.DeletingClusterCompPhase))
		})

		It("TODO - stopping", func() {
			Expect(statusComponent()).Should(Succeed())
		})

		It("stopped", func() {
			By("set replicas as 0 (stop or scale-in to 0)")
			spec().Replicas = 0
			Expect(updateComponent()).Should(Succeed())

			By("check the workload updated")
			Eventually(testapps.CheckObj(&testCtx, rsmKey(), func(g Gomega, rsm *workloads.ReplicatedStateMachine) {
				g.Expect(rsm.GetGeneration()).Should(Equal(int64(2)))
				g.Expect(*rsm.Spec.Replicas).Should(Equal(spec().Replicas))
			})).Should(Succeed())
			Eventually(testapps.List(&testCtx, generics.PodSignature, labels())).Should(HaveLen(0))

			By("status component")
			Expect(statusComponent()).Should(Succeed())

			By("check component status as STOPPED")
			Expect(status().Phase).Should(Equal(appsv1alpha1.StoppedClusterCompPhase))
		})
	})
})
