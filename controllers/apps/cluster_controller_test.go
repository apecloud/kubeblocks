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

package apps

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sethvargo/go-password/password"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Cluster Controller", func() {
	const (
		clusterDefName        = "test-clusterdef"
		compDefName           = "test-compdef"
		compVersionName       = "test-compver"
		clusterName           = "test-cluster"
		defaultCompName       = "mysql"
		defaultServiceVersion = "8.0.31-r0"
		latestServiceVersion  = "8.0.31-r1"
		defaultShardCount     = 2
	)

	var (
		clusterDefObj   *appsv1.ClusterDefinition
		compDefObj      *appsv1.ComponentDefinition
		compVersionObj  *appsv1.ComponentVersion
		clusterObj      *appsv1.Cluster
		clusterKey      types.NamespacedName
		allSettings     map[string]interface{}
		defaultTopology = appsv1.ClusterTopology{
			Name:    "default",
			Default: true,
			Components: []appsv1.ClusterTopologyComponent{
				{
					Name:    defaultCompName,
					CompDef: compDefName, // prefix
				},
			},
		}
	)

	resetTestContext := func() {
		clusterDefObj = nil
		compDefObj = nil
		compVersionObj = nil
		clusterObj = nil
		if allSettings != nil {
			Expect(viper.MergeConfigMap(allSettings)).ShouldNot(HaveOccurred())
			allSettings = nil
		}
	}

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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.VolumeSnapshotSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ServiceSignature, true, inNS)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
		testapps.ClearResources(&testCtx, generics.ActionSetSignature, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		resetTestContext()
	}

	BeforeEach(func() {
		cleanEnv()
		allSettings = viper.AllSettings()
	})

	AfterEach(func() {
		cleanEnv()
	})

	randomStr := func() string {
		str, _ := password.Generate(6, 0, 0, true, false)
		return str
	}

	createAllDefinitionObjects := func() {
		By("Create a componentDefinition obj")
		compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
			WithRandomName().
			SetDefaultSpec().
			Create(&testCtx).
			GetObject()

		By("Create a componentVersion obj")
		compVersionObj = testapps.NewComponentVersionFactory(compVersionName).
			SetSpec(appsv1.ComponentVersionSpec{
				CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
					{
						CompDefs: []string{compDefName}, // prefix
						Releases: []string{"v0.1.0", "v0.2.0"},
					},
				},
				Releases: []appsv1.ComponentVersionRelease{
					{
						Name:           "v0.1.0",
						Changes:        "init release",
						ServiceVersion: defaultServiceVersion,
						Images: map[string]string{
							compDefObj.Spec.Runtime.Containers[0].Name: compDefObj.Spec.Runtime.Containers[0].Image + "-" + defaultServiceVersion,
						},
					},
					{
						Name:           "v0.2.0",
						Changes:        "new release",
						ServiceVersion: latestServiceVersion,
						Images: map[string]string{
							compDefObj.Spec.Runtime.Containers[0].Name: compDefObj.Spec.Runtime.Containers[0].Image + "-" + latestServiceVersion,
						},
					},
				},
			}).
			Create(&testCtx).
			GetObject()

		By("Create a clusterDefinition obj")
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddClusterTopology(defaultTopology).
			Create(&testCtx).
			GetObject()

		By("Create a bpt obj")
		createBackupPolicyTpl(compDefObj.Name)

		By("Wait objects available")
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compDefObj),
			func(g Gomega, compDef *appsv1.ComponentDefinition) {
				g.Expect(compDef.Status.ObservedGeneration).Should(Equal(compDef.Generation))
				g.Expect(compDef.Status.Phase).Should(Equal(appsv1.AvailablePhase))
			})).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(compVersionObj),
			func(g Gomega, compVersion *appsv1.ComponentVersion) {
				g.Expect(compVersion.Status.ObservedGeneration).Should(Equal(compVersion.Generation))
				g.Expect(compVersion.Status.Phase).Should(Equal(appsv1.AvailablePhase))
			})).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
			func(g Gomega, clusterDef *appsv1.ClusterDefinition) {
				g.Expect(clusterDef.Status.ObservedGeneration).Should(Equal(clusterDef.Generation))
				g.Expect(clusterDef.Status.Phase).Should(Equal(appsv1.AvailablePhase))
			})).Should(Succeed())
	}

	waitForCreatingResourceCompletely := func(clusterKey client.ObjectKey, compNames ...string) {
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		cluster := &appsv1.Cluster{}
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, cluster, true)).Should(Succeed())
		for _, compName := range compNames {
			compPhase := appsv1.CreatingClusterCompPhase
			for _, spec := range cluster.Spec.ComponentSpecs {
				if spec.Name == compName && spec.Replicas == 0 {
					compPhase = appsv1.StoppedClusterCompPhase
				}
			}
			Eventually(testapps.GetClusterComponentPhase(&testCtx, clusterKey, compName)).Should(Equal(compPhase))
		}
	}

	createClusterObjNoWait := func(clusterDefName string, processor ...func(*testapps.MockClusterFactory)) {
		f := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName).
			WithRandomName()
		for _, p := range processor {
			if p != nil {
				p(f)
			}
		}
		clusterObj = f.Create(&testCtx).GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
	}

	componentProcessorWrapper := func(compName, compDefName string, processor ...func(*testapps.MockClusterFactory)) func(f *testapps.MockClusterFactory) {
		return func(f *testapps.MockClusterFactory) {
			f.AddComponent(compName, compDefName).SetReplicas(1)
			for _, p := range processor {
				if p != nil {
					p(f)
				}
			}
		}
	}

	shardingComponentProcessorWrapper := func(compName, compDefName string, processor ...func(*testapps.MockClusterFactory)) func(f *testapps.MockClusterFactory) {
		return func(f *testapps.MockClusterFactory) {
			f.AddShardingSpec(compName, compDefName).SetShards(defaultShardCount)
			for _, p := range processor {
				if p != nil {
					p(f)
				}
			}
		}
	}

	multipleTemplateComponentProcessorWrapper := func(compName, compDefName string, processor ...func(*testapps.MockClusterFactory)) func(f *testapps.MockClusterFactory) {
		return func(f *testapps.MockClusterFactory) {
			f.AddMultipleTemplateComponent(compName, compDefName).SetReplicas(3)
			for _, p := range processor {
				if p != nil {
					p(f)
				}
			}
		}
	}

	createClusterObj := func(compName, compDefName string, processor func(*testapps.MockClusterFactory)) {
		By("Creating a cluster with new component definition")
		createClusterObjNoWait("", componentProcessorWrapper(compName, compDefName, processor))

		By("Waiting for the cluster enter Creating phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1.CreatingClusterPhase))

		By("Wait component created")
		compKey := types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      constant.GenerateClusterComponentName(clusterObj.Name, compName),
		}
		Eventually(testapps.CheckObjExists(&testCtx, compKey, &appsv1.Component{}, true)).Should(Succeed())
	}

	createClusterObjWithTopology := func(topology, compName string, processor func(*testapps.MockClusterFactory)) {
		By("Creating a cluster with new component definition")
		setTopology := func(f *testapps.MockClusterFactory) { f.SetTopology(topology) }
		createClusterObjNoWait(clusterDefObj.Name, componentProcessorWrapper(compName, "", setTopology, processor))

		By("Waiting for the cluster enter Creating phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1.CreatingClusterPhase))

		By("Wait components created")
		compKey := types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      constant.GenerateClusterComponentName(clusterObj.Name, compName),
		}
		Eventually(testapps.CheckObjExists(&testCtx, compKey, &appsv1.Component{}, true)).Should(Succeed())
	}

	createClusterObjWithSharding := func(compTplName, compDefName string, processor func(*testapps.MockClusterFactory)) {
		By("Creating a cluster with new component definition")
		createClusterObjNoWait("", shardingComponentProcessorWrapper(compTplName, compDefName, processor))

		By("Waiting for the cluster enter Creating phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1.CreatingClusterPhase))

		By("Wait component created")
		ml := client.MatchingLabels{
			constant.AppInstanceLabelKey:       clusterKey.Name,
			constant.KBAppShardingNameLabelKey: compTplName,
		}
		Eventually(testapps.List(&testCtx, generics.ComponentSignature,
			ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(defaultShardCount))

		By("checking backup policy")
		backupPolicyName := generateBackupPolicyName(clusterKey.Name, compTplName, "")
		backupPolicyKey := client.ObjectKey{Name: backupPolicyName, Namespace: clusterKey.Namespace}
		Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, bp *dpv1alpha1.BackupPolicy) {
			g.Expect(bp.Spec.Targets).Should(HaveLen(defaultShardCount))
		})).Should(Succeed())

		By("checking backup schedule")
		backupScheduleName := generateBackupScheduleName(clusterKey.Name, compTplName, "")
		backupScheduleKey := client.ObjectKey{Name: backupScheduleName, Namespace: clusterKey.Namespace}
		Eventually(testapps.CheckObjExists(&testCtx, backupScheduleKey,
			&dpv1alpha1.BackupSchedule{}, true)).Should(Succeed())
	}

	createClusterObjWithMultipleTemplates := func(compName, compDefName string, processor func(*testapps.MockClusterFactory)) {
		By("Creating a cluster with new component definition")
		createClusterObjNoWait("", multipleTemplateComponentProcessorWrapper(compName, compDefName, processor))

		By("Waiting for the cluster enter Creating phase")
		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1.CreatingClusterPhase))

		By("Wait component created")
		compKey := types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      constant.GenerateClusterComponentName(clusterObj.Name, compName),
		}
		compObj := &appsv1.Component{}
		Eventually(testapps.CheckObjExists(&testCtx, compKey, compObj, true)).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
			g.Expect(cluster.Spec.ComponentSpecs).Should(HaveLen(1))
			clusterJSON, err := json.Marshal(cluster.Spec.ComponentSpecs[0].Instances)
			g.Expect(err).Should(BeNil())
			itsJSON, err := json.Marshal(compObj.Spec.Instances)
			g.Expect(err).Should(BeNil())
			g.Expect(clusterJSON).Should(Equal(itsJSON))
		})).Should(Succeed())
	}

	testClusterComponent := func(compName, compDefName string, createObj func(string, string, func(*testapps.MockClusterFactory))) {
		createObj(compName, compDefName, nil)

		By("check component created")
		compKey := types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      constant.GenerateClusterComponentName(clusterObj.Name, compName),
		}
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1.Component) {
			g.Expect(comp.Generation).Should(BeEquivalentTo(1))
			for k, v := range constant.GetComponentWellKnownLabels(clusterObj.Name, compName) {
				g.Expect(comp.Labels).Should(HaveKeyWithValue(k, v))
			}
			if compDefName == compDefObj.Name {
				g.Expect(comp.Spec.CompDef).Should(Equal(compDefName))
			} else {
				g.Expect(comp.Spec.CompDef).Should(BeEmpty())
			}
		})).Should(Succeed())
	}

	testClusterComponentWithTopology := func(topology, compName string, processor func(*testapps.MockClusterFactory), expectedCompDef, expectedServiceVersion string) {
		createClusterObjWithTopology(topology, compName, processor)

		By("check cluster updated")
		Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
			if len(topology) == 0 {
				g.Expect(cluster.Spec.Topology).Should(Equal(defaultTopology.Name))
			} else {
				g.Expect(cluster.Spec.Topology).Should(Equal(topology))
			}
			g.Expect(cluster.Spec.ComponentSpecs).Should(HaveLen(len(defaultTopology.Components)))
			for i, comp := range defaultTopology.Components {
				g.Expect(cluster.Spec.ComponentSpecs[i].Name).Should(Equal(comp.Name))
				g.Expect(cluster.Spec.ComponentSpecs[i].ComponentDef).Should(Equal(expectedCompDef))
				g.Expect(cluster.Spec.ComponentSpecs[i].ServiceVersion).Should(Equal(expectedServiceVersion))
			}
		})).Should(Succeed())

		By("check component created")
		compKey := types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      constant.GenerateClusterComponentName(clusterObj.Name, compName),
		}
		Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1.Component) {
			g.Expect(comp.Spec.CompDef).Should(Equal(expectedCompDef))
			g.Expect(comp.Spec.ServiceVersion).Should(Equal(expectedServiceVersion))
		})).Should(Succeed())
	}

	testShardingClusterComponent := func(compName, compDefName string, createObj func(string, string, func(*testapps.MockClusterFactory)), shards int) {
		createObj(compName, compDefName, nil)

		By("check components created")
		ml := client.MatchingLabels{
			constant.AppInstanceLabelKey:       clusterKey.Name,
			constant.KBAppShardingNameLabelKey: compName,
		}
		Eventually(testapps.List(&testCtx, generics.ComponentSignature, ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(shards))
	}

	testClusterComponentScaleIn := func(compName, compDefName string) {
		otherCompName := fmt.Sprintf("%s-a", compName)

		By("creating and checking a cluster with multi component")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddComponent(compName, compDefName).SetReplicas(3).
			AddComponent(otherCompName, compDefName).SetReplicas(3).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("waiting for the cluster controller to create resources completely")
		waitForCreatingResourceCompletely(clusterKey, compName, otherCompName)

		By("scale in the target component")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			for i, compSpec := range cluster.Spec.ComponentSpecs {
				if compSpec.Name == compName {
					// delete the target component
					cluster.Spec.ComponentSpecs = append(cluster.Spec.ComponentSpecs[:i], cluster.Spec.ComponentSpecs[i+1:]...)
				}
			}
		})()).ShouldNot(HaveOccurred())

		By("check component deleted")
		compKey := types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      constant.GenerateClusterComponentName(clusterObj.Name, compName),
		}
		multiCompKey := types.NamespacedName{
			Namespace: clusterObj.Namespace,
			Name:      constant.GenerateClusterComponentName(clusterObj.Name, otherCompName),
		}
		Eventually(testapps.CheckObjExists(&testCtx, compKey, &appsv1.Component{}, false)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, multiCompKey, &appsv1.Component{}, true)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1.Cluster{}, true)).Should(Succeed())
	}

	testClusterShardingComponentScaleIn := func(compName, compDefName string, createObj func(string, string, func(*testapps.MockClusterFactory)), shards int) {
		By("creating and checking a cluster with sharding component")
		testShardingClusterComponent(compName, compDefName, createObj, shards)

		By("scale in the sharding component")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			for i := range cluster.Spec.ShardingSpecs {
				if cluster.Spec.ShardingSpecs[i].Name == compName {
					cluster.Spec.ShardingSpecs[i].Shards = int32(shards - 1)
				}
			}
		})()).ShouldNot(HaveOccurred())

		By("check sharding component scaled in")
		ml := client.MatchingLabels{
			constant.AppInstanceLabelKey:       clusterKey.Name,
			constant.KBAppShardingNameLabelKey: compName,
		}
		Eventually(testapps.List(&testCtx, generics.ComponentSignature, ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(shards - 1))
	}

	testClusterService := func(compName, compDefName string, createObj func(string, string, func(*testapps.MockClusterFactory))) {
		randClusterName := fmt.Sprintf("%s-%s", clusterName, randomStr())
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: randClusterName,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Protocol: corev1.ProtocolTCP,
						Port:     3306,
					},
				},
				Type: corev1.ServiceTypeLoadBalancer,
			},
		}
		createObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.SetName(randClusterName).
				AddAppManagedByLabel().
				AddAppInstanceLabel(randClusterName).
				AddService(appsv1.ClusterService{
					Service: appsv1.Service{
						Name:         service.Name,
						ServiceName:  service.Name,
						Spec:         service.Spec,
						RoleSelector: constant.Follower,
					},
					ComponentSelector: compName,
				})
		})

		By("check cluster service created")
		clusterSvcKey := types.NamespacedName{
			Namespace: clusterKey.Namespace,
			Name:      constant.GenerateClusterServiceName(clusterObj.Name, service.Name),
		}
		Eventually(testapps.CheckObj(&testCtx, clusterSvcKey, func(g Gomega, svc *corev1.Service) {
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterObj.Name))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.RoleLabelKey, constant.Follower))
			g.Expect(svc.Spec.ExternalTrafficPolicy).Should(BeEquivalentTo(corev1.ServiceExternalTrafficPolicyTypeLocal))
		})).Should(Succeed())

		By("check default component service created")
		compSvcKey := types.NamespacedName{
			Namespace: clusterKey.Namespace,
			Name:      constant.GenerateComponentServiceName(clusterObj.Name, compName, ""),
		}
		Eventually(testapps.CheckObj(&testCtx, compSvcKey, func(g Gomega, svc *corev1.Service) {
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterObj.Name))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
			g.Expect(svc.Spec.Selector).Should(HaveKey(constant.RoleLabelKey))
			g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.RoleLabelKey, constant.Leader))
		})).Should(Succeed())
	}

	type expectService struct {
		clusterIP string
		svcType   corev1.ServiceType
	}

	validateClusterServiceList := func(g Gomega, expectServices map[string]expectService, compName string, shardCount *int, enableShardOrdinal bool) {
		svcList := &corev1.ServiceList{}
		g.Expect(testCtx.Cli.List(testCtx.Ctx, svcList, client.MatchingLabels{
			constant.AppInstanceLabelKey: clusterKey.Name,
		}, client.InNamespace(clusterKey.Namespace))).Should(Succeed())

		// filter out default component services
		services := make([]*corev1.Service, 0)
		for i, svc := range svcList.Items {
			if _, ok := svc.Labels[constant.KBAppComponentLabelKey]; ok {
				continue
			}
			services = append(services, &svcList.Items[i])
		}

		validateSvc := func(svc *corev1.Service, svcSpec expectService) {
			g.Expect(svc.Spec.Type).Should(Equal(svcSpec.svcType))
			// g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.RoleLabelKey, "leader"))
			switch {
			case svc.Spec.Type == corev1.ServiceTypeLoadBalancer:
				g.Expect(svc.Spec.ExternalTrafficPolicy).Should(Equal(corev1.ServiceExternalTrafficPolicyTypeLocal))
			case svc.Spec.Type == corev1.ServiceTypeClusterIP && len(svcSpec.clusterIP) == 0:
				g.Expect(svc.Spec.ClusterIP).ShouldNot(Equal(corev1.ClusterIPNone))
			case svc.Spec.Type == corev1.ServiceTypeClusterIP && len(svcSpec.clusterIP) != 0:
				g.Expect(svc.Spec.ClusterIP).Should(Equal(corev1.ClusterIPNone))
				// for _, port := range getHeadlessSvcPorts(g, compDefName) {
				//	g.Expect(slices.Index(svc.Spec.Ports, port) >= 0).Should(BeTrue())
				// }
			}
		}

		if shardCount == nil {
			for svcName, svcSpec := range expectServices {
				idx := slices.IndexFunc(services, func(e *corev1.Service) bool {
					return e.Name == constant.GenerateClusterServiceName(clusterObj.Name, svcName)
				})
				g.Expect(idx >= 0).To(BeTrue())
				svc := services[idx]
				g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppComponentLabelKey, compName))
				validateSvc(svc, svcSpec)
			}
			g.Expect(len(expectServices)).Should(Equal(len(services)))
		} else {
			if enableShardOrdinal {
				g.Expect(len(expectServices) * *shardCount).Should(Equal(len(services)))
			} else {
				for svcName, svcSpec := range expectServices {
					idx := slices.IndexFunc(services, func(e *corev1.Service) bool {
						return e.Name == constant.GenerateClusterServiceName(clusterObj.Name, svcName)
					})
					g.Expect(idx >= 0).To(BeTrue())
					svc := services[idx]
					g.Expect(svc.Spec.Selector).Should(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, compName))
					validateSvc(svc, svcSpec)
				}
				g.Expect(len(expectServices)).Should(Equal(len(services)))
			}
		}
	}

	testClusterServiceCreateAndDelete := func(compName, compDefName string, createObj func(string, string, func(*testapps.MockClusterFactory))) {
		expectServices := map[string]expectService{
			testapps.ServiceDefaultName:  {"", corev1.ServiceTypeClusterIP},
			testapps.ServiceHeadlessName: {corev1.ClusterIPNone, corev1.ServiceTypeClusterIP},
			testapps.ServiceVPCName:      {"", corev1.ServiceTypeLoadBalancer},
			testapps.ServiceInternetName: {"", corev1.ServiceTypeLoadBalancer},
		}

		services := make([]appsv1.ClusterService, 0)
		for name, svc := range expectServices {
			services = append(services, appsv1.ClusterService{
				Service: appsv1.Service{
					Name:        name,
					ServiceName: name,
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Port: 3306},
						},
						Type:      svc.svcType,
						ClusterIP: svc.clusterIP,
					},
				},
				ComponentSelector: compName,
				// RoleSelector:      []string{"leader"},
			})
		}
		createObj(compName, compDefName, func(f *testapps.MockClusterFactory) {
			f.AddService(services[0]).
				AddService(services[1]).
				AddService(services[2])
		})

		deleteService := services[2]
		lastService := services[3]

		By("create last cluster service manually which will not owned by cluster")
		lastServiceName := constant.GenerateClusterServiceName(clusterObj.Name, lastService.ServiceName)
		svcObj := builder.NewServiceBuilder(clusterObj.Namespace, lastServiceName).
			AddLabelsInMap(constant.GetClusterWellKnownLabels(clusterObj.Name)).
			SetSpec(&lastService.Spec).
			AddSelector(constant.KBAppComponentLabelKey, lastService.ComponentSelector).
			// AddSelector(constant.RoleLabelKey, lastService.RoleSelector[0]).
			Optimize4ExternalTraffic().
			GetObject()
		Expect(testCtx.CheckedCreateObj(testCtx.Ctx, svcObj)).Should(Succeed())

		By("check all services created")
		Eventually(func(g Gomega) { validateClusterServiceList(g, expectServices, compName, nil, false) }).Should(Succeed())

		By("delete a cluster service")
		delete(expectServices, deleteService.Name)
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			var svcs []appsv1.ClusterService
			for _, item := range cluster.Spec.Services {
				if item.Name != deleteService.Name {
					svcs = append(svcs, item)
				}
			}
			cluster.Spec.Services = svcs
		})()).ShouldNot(HaveOccurred())

		By("check the service has been deleted, and the non-managed service has not been deleted")
		Eventually(func(g Gomega) { validateClusterServiceList(g, expectServices, compName, nil, false) }).Should(Succeed())

		By("add the deleted service back")
		expectServices[deleteService.Name] = expectService{deleteService.Spec.ClusterIP, deleteService.Spec.Type}
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			cluster.Spec.Services = append(cluster.Spec.Services, deleteService)
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) { validateClusterServiceList(g, expectServices, compName, nil, false) }).Should(Succeed())
	}

	testShardingClusterServiceCreateAndDelete := func(compTplName, compDefName string, createObj func(string, string, func(*testapps.MockClusterFactory))) {
		expectServices := map[string]expectService{
			testapps.ServiceDefaultName:  {"", corev1.ServiceTypeClusterIP},
			testapps.ServiceHeadlessName: {corev1.ClusterIPNone, corev1.ServiceTypeClusterIP},
		}

		services := make([]appsv1.ClusterService, 0)
		for name, svc := range expectServices {
			services = append(services, appsv1.ClusterService{
				Service: appsv1.Service{
					Name:        name,
					ServiceName: name,
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Port: 3306},
						},
						Type:      svc.svcType,
						ClusterIP: svc.clusterIP,
					},
				},
				ShardingSelector: compTplName,
			})
		}
		createObj(compTplName, compDefName, func(f *testapps.MockClusterFactory) {
			f.AddService(services[0]).AddService(services[1])
		})

		shards := defaultShardCount
		deleteService := services[0]

		By("check only one service created for each shard when ShardSvcAnnotationKey is not set")
		Eventually(func(g Gomega) { validateClusterServiceList(g, expectServices, compTplName, &shards, false) }).Should(Succeed())

		By("check shards number services were created for each shard when ShardSvcAnnotationKey is set")
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			if cluster.Annotations == nil {
				cluster.Annotations = map[string]string{}
			}
			cluster.Annotations[constant.ShardSvcAnnotationKey] = compTplName
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) { validateClusterServiceList(g, expectServices, compTplName, &shards, true) }).Should(Succeed())

		By("delete a cluster shard service")
		delete(expectServices, deleteService.Name)
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			var svcs []appsv1.ClusterService
			for _, item := range cluster.Spec.Services {
				if item.Name != deleteService.Name {
					svcs = append(svcs, item)
				}
			}
			cluster.Spec.Services = svcs
		})()).ShouldNot(HaveOccurred())

		By("check the service has been deleted, and the non-managed service has not been deleted")
		Eventually(func(g Gomega) { validateClusterServiceList(g, expectServices, compTplName, &shards, true) }).Should(Succeed())

		By("add the deleted service back")
		expectServices[deleteService.Name] = expectService{deleteService.Spec.ClusterIP, deleteService.Spec.Type}
		Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			cluster.Spec.Services = append(cluster.Spec.Services, deleteService)
		})()).ShouldNot(HaveOccurred())
		Eventually(func(g Gomega) { validateClusterServiceList(g, expectServices, compTplName, &shards, true) }).Should(Succeed())
	}

	testClusterFinalizer := func(compName string, createObj func(appsv1.TerminationPolicyType)) {
		createObj(appsv1.WipeOut)

		By("wait component created")
		compKey := types.NamespacedName{
			Namespace: clusterKey.Namespace,
			Name:      clusterKey.Name + "-" + compName,
		}
		Eventually(testapps.CheckObjExists(&testCtx, compKey, &appsv1.Component{}, true)).Should(Succeed())

		By("set finalizer for component to prevent it from deletion")
		finalizer := "test/finalizer"
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *appsv1.Component) {
			comp.Finalizers = append(comp.Finalizers, finalizer)
		})()).ShouldNot(HaveOccurred())

		By("delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1.Cluster{})

		By("check cluster keep existing")
		Consistently(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1.Cluster{}, true)).Should(Succeed())

		By("remove finalizer of component to get it deleted")
		Expect(testapps.GetAndChangeObj(&testCtx, compKey, func(comp *appsv1.Component) {
			comp.Finalizers = nil
		})()).ShouldNot(HaveOccurred())

		By("wait for the cluster and component to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, compKey, &appsv1.Component{}, false)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1.Cluster{}, false)).Should(Succeed())
	}

	testDeleteClusterWithDoNotTerminate := func(createObj func(appsv1.TerminationPolicyType)) {
		createObj(appsv1.DoNotTerminate)

		By("check all other resources deleted")
		transCtx := &clusterTransformContext{
			Context: testCtx.Ctx,
			Client:  testCtx.Cli,
		}
		namespacedKinds, clusteredKinds := kindsForWipeOut()
		allKinds := append(namespacedKinds, clusteredKinds...)
		createdObjs, err := getOwningNamespacedObjects(transCtx.Context, transCtx.Client, clusterObj.Namespace, getAppInstanceML(*clusterObj), allKinds)
		Expect(err).Should(Succeed())

		By("delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1.Cluster{})
		Consistently(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1.Cluster{}, true)).Should(Succeed())

		By("check all cluster resources again")
		objs, err := getOwningNamespacedObjects(transCtx.Context, transCtx.Client, clusterObj.Namespace, getAppInstanceML(*clusterObj), allKinds)
		Expect(err).Should(Succeed())
		// check all objects existed before cluster deletion still be there
		for key, obj := range createdObjs {
			Expect(objs).Should(HaveKey(key))
			Expect(obj.GetUID()).Should(BeEquivalentTo(objs[key].GetUID()))
		}
	}

	testDeleteClusterWithHalt := func(createObj func(appsv1.TerminationPolicyType)) {
		createObj(appsv1.Halt)

		transCtx := &clusterTransformContext{
			Context: testCtx.Ctx,
			Client:  testCtx.Cli,
		}
		preserveKinds := haltPreserveKinds()
		preserveObjs, err := getOwningNamespacedObjects(transCtx.Context, transCtx.Client, clusterObj.Namespace, getAppInstanceML(*clusterObj), preserveKinds)
		Expect(err).Should(Succeed())
		for _, obj := range preserveObjs {
			// Expect(obj.GetFinalizers()).Should(ContainElements(constant.DBClusterFinalizerName))
			Expect(obj.GetAnnotations()).ShouldNot(HaveKey(constant.LastAppliedClusterAnnotationKey))
		}

		By("delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1.Cluster{})

		By("wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1.Cluster{}, false)).Should(Succeed())

		By("check expected preserved objects")
		keptObjs, err := getOwningNamespacedObjects(transCtx.Context, transCtx.Client, clusterObj.Namespace, getAppInstanceML(*clusterObj), preserveKinds)
		Expect(err).Should(Succeed())
		for key, obj := range preserveObjs {
			Expect(keptObjs).Should(HaveKey(key))
			keptObj := keptObjs[key]
			Expect(obj.GetUID()).Should(BeEquivalentTo(keptObj.GetUID()))
			Expect(keptObj.GetFinalizers()).ShouldNot(ContainElements(constant.DBClusterFinalizerName))
			Expect(keptObj.GetAnnotations()).Should(HaveKey(constant.LastAppliedClusterAnnotationKey))
		}

		By("check all other resources deleted")
		namespacedKinds, clusteredKinds := kindsForHalt()
		kindsToDelete := append(namespacedKinds, clusteredKinds...)
		otherObjs, err := getOwningNamespacedObjects(transCtx.Context, transCtx.Client, clusterObj.Namespace, getAppInstanceML(*clusterObj), kindsToDelete)
		Expect(err).Should(Succeed())
		Expect(otherObjs).Should(HaveLen(0))
	}

	testClusterHaltNRecovery := func(createObj func(appsv1.TerminationPolicyType)) {
		// TODO(component)
	}

	deleteClusterWithBackup := func(terminationPolicy appsv1.TerminationPolicyType, backupRetainPolicy string) {
		By("mocking a retained backup")
		backupPolicyName := "test-backup-policy"
		backupName := "test-backup"
		backupMethod := "test-backup-method"
		backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, backupName).
			SetBackupPolicyName(backupPolicyName).
			SetBackupMethod(backupMethod).
			SetLabels(map[string]string{
				constant.AppManagedByLabelKey:     constant.AppName,
				constant.AppInstanceLabelKey:      clusterObj.Name,
				constant.BackupProtectionLabelKey: backupRetainPolicy,
			}).
			WithRandomName().
			Create(&testCtx).GetObject()
		backupKey := client.ObjectKeyFromObject(backup)
		Eventually(testapps.CheckObjExists(&testCtx, backupKey, &dpv1alpha1.Backup{}, true)).Should(Succeed())

		By("delete the cluster")
		testapps.DeleteObject(&testCtx, clusterKey, &appsv1.Cluster{})

		By("wait for the cluster to terminate")
		Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1.Cluster{}, false)).Should(Succeed())

		By(fmt.Sprintf("checking the backup with TerminationPolicyType=%s", terminationPolicy))
		if terminationPolicy == appsv1.WipeOut && backupRetainPolicy == constant.BackupDelete {
			Eventually(testapps.CheckObjExists(&testCtx, backupKey, &dpv1alpha1.Backup{}, false)).Should(Succeed())
		} else {
			Consistently(testapps.CheckObjExists(&testCtx, backupKey, &dpv1alpha1.Backup{}, true)).Should(Succeed())
		}

		By("check all other resources deleted")
		transCtx := &clusterTransformContext{
			Context: testCtx.Ctx,
			Client:  testCtx.Cli,
		}
		var namespacedKinds, clusteredKinds []client.ObjectList
		if terminationPolicy == appsv1.WipeOut && backupRetainPolicy == constant.BackupDelete {
			namespacedKinds, clusteredKinds = kindsForWipeOut()
		} else {
			namespacedKinds, clusteredKinds = kindsForDelete()
		}
		kindsToDelete := append(namespacedKinds, clusteredKinds...)
		otherObjs, err := getOwningNamespacedObjects(transCtx.Context, transCtx.Client, clusterObj.Namespace, getAppInstanceML(*clusterObj), kindsToDelete)
		Expect(err).Should(Succeed())
		Expect(otherObjs).Should(HaveLen(0))
	}

	testDeleteClusterWithDelete := func(createObj func(appsv1.TerminationPolicyType)) {
		createObj(appsv1.Delete)
		deleteClusterWithBackup(appsv1.Delete, constant.BackupRetain)
	}

	testDeleteClusterWithWipeOut := func(createObj func(appsv1.TerminationPolicyType), backupRetainPolicy string) {
		createObj(appsv1.WipeOut)
		deleteClusterWithBackup(appsv1.WipeOut, backupRetainPolicy)
	}

	Context("cluster provisioning", func() {
		BeforeEach(func() {
			createAllDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("create cluster", func() {
			testClusterComponent(defaultCompName, compDefObj.Name, createClusterObj)
		})

		It("create sharding cluster", func() {
			testShardingClusterComponent(defaultCompName, compDefObj.Name, createClusterObjWithSharding, defaultShardCount)
		})

		It("create cluster with default topology", func() {
			testClusterComponentWithTopology("", defaultCompName, nil, compDefObj.Name, latestServiceVersion)
		})

		It("create cluster with specified topology", func() {
			testClusterComponentWithTopology(defaultTopology.Name, defaultCompName, nil, compDefObj.Name, latestServiceVersion)
		})

		It("create cluster with specified service version", func() {
			setServiceVersion := func(f *testapps.MockClusterFactory) {
				f.SetServiceVersion(defaultServiceVersion)
			}
			testClusterComponentWithTopology(defaultTopology.Name, defaultCompName, setServiceVersion, compDefObj.Name, defaultServiceVersion)
		})

		It("create multiple templates cluster", func() {
			testClusterComponent(defaultCompName, compDefObj.Name, createClusterObjWithMultipleTemplates)
		})
	})

	Context("cluster component scale-in", func() {
		BeforeEach(func() {
			createAllDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("with cluster component scale-in", func() {
			testClusterComponentScaleIn(defaultCompName, compDefObj.Name)
		})

		It("with cluster sharding scale-in", func() {
			testClusterShardingComponentScaleIn(defaultCompName, compDefObj.Name, createClusterObjWithSharding, defaultShardCount)
		})
	})

	Context("cluster termination policy", func() {
		var (
			createObj = func(policyType appsv1.TerminationPolicyType) {
				createClusterObj(defaultCompName, compDefObj.Name, func(f *testapps.MockClusterFactory) {
					f.SetTerminationPolicy(policyType)
				})
			}
		)

		BeforeEach(func() {
			createAllDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("deleted after all the sub-resources", func() {
			testClusterFinalizer(defaultCompName, createObj)
		})

		It("delete cluster with terminationPolicy=DoNotTerminate", func() {
			testDeleteClusterWithDoNotTerminate(createObj)
		})

		It("delete cluster with terminationPolicy=Halt", func() {
			testDeleteClusterWithHalt(createObj)
		})

		It("cluster Halt and Recovery", func() {
			testClusterHaltNRecovery(createObj)
		})

		It("delete cluster with terminationPolicy=Delete", func() {
			testDeleteClusterWithDelete(createObj)
		})

		It("delete cluster with terminationPolicy=WipeOut and backupRetainPolicy=Delete", func() {
			testDeleteClusterWithWipeOut(createObj, constant.BackupDelete)
		})

		It("delete cluster with terminationPolicy=WipeOut and backupRetainPolicy=Retain", func() {
			testDeleteClusterWithWipeOut(createObj, constant.BackupRetain)
		})
	})

	Context("cluster status", func() {
		BeforeEach(func() {
			createAllDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("test cluster conditions when cluster definition non-exist", func() {
			By("create a cluster with cluster definition non-exist")
			mockCompDefName := fmt.Sprintf("%s-%s", compDefName, testCtx.GetRandomStr())
			createClusterObjNoWait(clusterDefObj.Name, componentProcessorWrapper(defaultCompName, mockCompDefName))

			By("check conditions")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Status.ObservedGeneration).Should(BeZero())
				condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeProvisioningStarted)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Reason).Should(BeEquivalentTo(ReasonPreCheckFailed))
			})).Should(Succeed())
		})
	})

	Context("cluster with backup", func() {
		const (
			backupRepoName   = "test-backup-repo"
			backupMethodName = "test-backup-method"
		)

		BeforeEach(func() {
			cleanEnv()
			createAllDefinitionObjects()
		})

		createClusterWithBackup := func(backup *appsv1.ClusterBackup) {
			By("Creating a cluster")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				WithRandomName().
				AddComponent(defaultCompName, compDefName).
				SetBackup(backup).
				Create(&testCtx).GetObject()
			clusterKey = client.ObjectKeyFromObject(clusterObj)

			By("Waiting for the cluster controller to create resources completely")
			waitForCreatingResourceCompletely(clusterKey)
		}

		It("Creating cluster without backup", func() {
			createClusterWithBackup(nil)
			Eventually(testapps.List(&testCtx, generics.BackupPolicySignature,
				client.MatchingLabels{
					constant.AppInstanceLabelKey: clusterKey.Name,
				}, client.InNamespace(clusterKey.Namespace))).ShouldNot(BeEmpty())
		})

		It("Creating cluster with backup", func() {
			var (
				boolTrue  = true
				boolFalse = false
				int64Ptr  = func(in int64) *int64 {
					return &in
				}
				retention = func(s string) dpv1alpha1.RetentionPeriod {
					return dpv1alpha1.RetentionPeriod(s)
				}
			)

			var testCases = []struct {
				desc   string
				backup *appsv1.ClusterBackup
			}{
				{
					desc: "backup with snapshot method",
					backup: &appsv1.ClusterBackup{
						Enabled:                 &boolTrue,
						RetentionPeriod:         retention("1d"),
						Method:                  vsBackupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						PITREnabled:             &boolTrue,
						RepoName:                backupRepoName,
					},
				},
				{
					desc: "disable backup",
					backup: &appsv1.ClusterBackup{
						Enabled:                 &boolFalse,
						RetentionPeriod:         retention("1d"),
						Method:                  vsBackupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						PITREnabled:             &boolTrue,
						RepoName:                backupRepoName,
					},
				},
				{
					desc: "backup with backup tool",
					backup: &appsv1.ClusterBackup{
						Enabled:                 &boolTrue,
						RetentionPeriod:         retention("2d"),
						Method:                  backupMethodName,
						CronExpression:          "*/1 * * * *",
						StartingDeadlineMinutes: int64Ptr(int64(10)),
						RepoName:                backupRepoName,
						PITREnabled:             &boolFalse,
					},
				},
				{
					desc:   "backup is nil",
					backup: nil,
				},
			}

			for _, t := range testCases {
				By(t.desc)
				backup := t.backup
				createClusterWithBackup(backup)

				checkSchedule := func(g Gomega, schedule *dpv1alpha1.BackupSchedule) {
					var policy *dpv1alpha1.SchedulePolicy
					enableOtherFullMethod := false
					for i, s := range schedule.Spec.Schedules {
						if s.BackupMethod == backup.Method {
							Expect(*s.Enabled).Should(BeEquivalentTo(*backup.Enabled))
							policy = &schedule.Spec.Schedules[i]
							if *backup.Enabled {
								enableOtherFullMethod = true
							}
							continue
						}
						if enableOtherFullMethod {
							// another full backup method should be disabled.
							Expect(*s.Enabled).Should(BeFalse())
						}
					}
					if backup.Enabled != nil && *backup.Enabled {
						Expect(policy).ShouldNot(BeNil())
						Expect(policy.RetentionPeriod).Should(BeEquivalentTo(backup.RetentionPeriod))
						Expect(policy.CronExpression).Should(BeEquivalentTo(backup.CronExpression))
					}
				}

				checkPolicy := func(g Gomega, policy *dpv1alpha1.BackupPolicy) {
					if backup != nil && backup.RepoName != "" {
						g.Expect(*policy.Spec.BackupRepoName).Should(BeEquivalentTo(backup.RepoName))
					}
					g.Expect(policy.Spec.BackupMethods).ShouldNot(BeEmpty())
				}

				By("checking backup policy")
				backupPolicyName := generateBackupPolicyName(clusterKey.Name, defaultCompName, "")
				backupPolicyKey := client.ObjectKey{Name: backupPolicyName, Namespace: clusterKey.Namespace}
				backupPolicy := &dpv1alpha1.BackupPolicy{}
				Eventually(testapps.CheckObjExists(&testCtx, backupPolicyKey, backupPolicy, true)).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, checkPolicy)).Should(Succeed())

				By("checking backup schedule")
				backupScheduleName := generateBackupScheduleName(clusterKey.Name, defaultCompName, "")
				backupScheduleKey := client.ObjectKey{Name: backupScheduleName, Namespace: clusterKey.Namespace}
				if backup == nil {
					Eventually(testapps.CheckObjExists(&testCtx, backupScheduleKey,
						&dpv1alpha1.BackupSchedule{}, true)).Should(Succeed())
					continue
				}
				Eventually(testapps.CheckObj(&testCtx, backupScheduleKey, checkSchedule)).Should(Succeed())
			}
		})
	})

	Context("cluster service", func() {
		BeforeEach(func() {
			createAllDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("with cluster service set", func() {
			testClusterService(defaultCompName, compDefObj.Name, createClusterObj)
		})

		It("should create and delete cluster service correctly", func() {
			testClusterServiceCreateAndDelete(defaultCompName, compDefObj.Name, createClusterObj)
		})

		It("should create and delete shard topology cluster service correctly", func() {
			testShardingClusterServiceCreateAndDelete(defaultCompName, compDefObj.Name, createClusterObjWithSharding)
		})
	})

	Context("cluster upgrade", func() {
		BeforeEach(func() {
			createAllDefinitionObjects()
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("upgrade service version", func() {
			setServiceVersion := func(f *testapps.MockClusterFactory) {
				f.SetServiceVersion(defaultServiceVersion)
			}
			testClusterComponentWithTopology(defaultTopology.Name, defaultCompName, setServiceVersion, compDefObj.Name, defaultServiceVersion)

			By("update cluster to upgrade service version")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
				cluster.Spec.ComponentSpecs[0].ServiceVersion = latestServiceVersion
			})()).ShouldNot(HaveOccurred())

			By("check cluster and component objects been upgraded")
			compKey := types.NamespacedName{
				Namespace: clusterObj.Namespace,
				Name:      constant.GenerateClusterComponentName(clusterObj.Name, defaultCompName),
			}
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDefObj.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(latestServiceVersion))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1.Component) {
				g.Expect(comp.Spec.CompDef).Should(Equal(compDefObj.Name))
				g.Expect(comp.Spec.ServiceVersion).Should(Equal(latestServiceVersion))
			})).Should(Succeed())
		})

		It("upgrade component definition", func() {
			setServiceVersion := func(f *testapps.MockClusterFactory) {
				f.SetServiceVersion(defaultServiceVersion)
			}
			testClusterComponentWithTopology(defaultTopology.Name, defaultCompName, setServiceVersion, compDefObj.Name, defaultServiceVersion)

			By("publish a new component definition obj")
			newCompDefObj := testapps.NewComponentDefinitionFactory(compDefObj.Name+"-r100").
				SetDefaultSpec().
				AddEnv(compDefObj.Spec.Runtime.Containers[0].Name, corev1.EnvVar{Name: "key", Value: "value"}).
				Create(&testCtx).
				GetObject()
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(newCompDefObj), func(g Gomega, compDef *appsv1.ComponentDefinition) {
				g.Expect(compDef.Status.ObservedGeneration).Should(Equal(compDef.Generation))
				g.Expect(compDef.Status.Phase).Should(Equal(appsv1.AvailablePhase))
			})).Should(Succeed())

			By("check cluster and component objects stay in original version before upgrading")
			compKey := types.NamespacedName{
				Namespace: clusterObj.Namespace,
				Name:      constant.GenerateClusterComponentName(clusterObj.Name, defaultCompName),
			}
			Consistently(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(compDefObj.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(defaultServiceVersion))
			})).Should(Succeed())
			Consistently(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1.Component) {
				g.Expect(comp.Spec.CompDef).Should(Equal(compDefObj.Name))
				g.Expect(comp.Spec.ServiceVersion).Should(Equal(defaultServiceVersion))
			})).Should(Succeed())

			By("update cluster to upgrade component definition")
			Expect(testapps.GetAndChangeObj(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
				cluster.Spec.ComponentSpecs[0].ComponentDef = ""
			})()).ShouldNot(HaveOccurred())

			By("check cluster and component objects been upgraded")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ComponentDef).Should(Equal(newCompDefObj.Name))
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(defaultServiceVersion))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, compKey, func(g Gomega, comp *appsv1.Component) {
				g.Expect(comp.Spec.CompDef).Should(Equal(newCompDefObj.Name))
				g.Expect(comp.Spec.ServiceVersion).Should(Equal(defaultServiceVersion))
			})).Should(Succeed())
		})
	})
})

func createBackupPolicyTpl(compDef string) {
	By("create actionSet")
	fakeActionSet("")

	By("Creating a BackupPolicyTemplate")
	bpt := testapps.NewBackupPolicyTemplateFactory(backupPolicyTPLName).
		AddLabels(compDef, compDef)

	ttl := "7d"
	bpt = bpt.AddBackupPolicy(compDef).
		AddBackupMethod(backupMethodName, false, actionSetName).
		SetBackupMethodVolumeMounts("data", "/data").
		AddBackupMethod(vsBackupMethodName, true, "").
		SetBackupMethodVolumes([]string{"data"}).
		AddSchedule(backupMethodName, "0 0 * * *", ttl, true).
		AddSchedule(vsBackupMethodName, "0 0 * * *", ttl, true)
	bpt.Create(&testCtx)
}
