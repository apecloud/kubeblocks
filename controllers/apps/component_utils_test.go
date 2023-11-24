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

package apps

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

func TestIsProbeTimeout(t *testing.T) {
	podsReadyTime := &metav1.Time{Time: time.Now().Add(-10 * time.Minute)}
	compDef := &appsv1alpha1.ClusterComponentDefinition{
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleProbe:                      &appsv1alpha1.ClusterDefinitionProbe{},
			RoleProbeTimeoutAfterPodsReady: appsv1alpha1.DefaultRoleProbeTimeoutAfterPodsReady,
		},
	}
	if !IsProbeTimeout(compDef.Probes, podsReadyTime) {
		t.Error("probe timed out should be true")
	}
}

var _ = Describe("Component Utils", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "mysql-clusterdef-" + randomStr
		clusterVersionName = "mysql-clusterversion-" + randomStr
		clusterName        = "mysql-" + randomStr
	)

	const (
		consensusCompDefRef = "consensus"
		consensusCompName   = "consensus"
		statelessCompName   = "stateless"
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

	Context("Component test", func() {
		It("Component test", func() {
			By(" init cluster, statefulSet, pods")
			_, _, cluster := testapps.InitClusterWithHybridComps(&testCtx, clusterDefName,
				clusterVersionName, clusterName, statelessCompName, "stateful", consensusCompName)
			sts := testapps.MockConsensusComponentStatefulSet(&testCtx, clusterName, consensusCompName)
			testapps.MockStatelessComponentDeploy(&testCtx, clusterName, statelessCompName)
			_ = testapps.MockConsensusComponentPods(&testCtx, sts, clusterName, consensusCompName)

			By("test GetClusterByObject function")
			newCluster, _ := GetClusterByObject(ctx, k8sClient, sts)
			Expect(newCluster != nil).Should(BeTrue())

			By("test consensusSet initClusterComponentStatusIfNeed function")
			err := initClusterComponentStatusIfNeed(cluster, consensusCompName, appsv1alpha1.Consensus)
			Expect(err).Should(Succeed())
			Expect(cluster.Status.Components[consensusCompName].ConsensusSetStatus).ShouldNot(BeNil())
			Expect(cluster.Status.Components[consensusCompName].ConsensusSetStatus.Leader.Pod).Should(Equal(constant.ComponentStatusDefaultPodName))

			By("test replicationSet initClusterComponentStatusIfNeed function")
			err = initClusterComponentStatusIfNeed(cluster, consensusCompName, appsv1alpha1.Replication)
			Expect(err).Should(Succeed())
			Expect(cluster.Status.Components[consensusCompName].ReplicationSetStatus).ShouldNot(BeNil())
			Expect(cluster.Status.Components[consensusCompName].ReplicationSetStatus.Primary.Pod).Should(Equal(constant.ComponentStatusDefaultPodName))

			By("test getObjectListByComponentName function")
			stsList := &appsv1.StatefulSetList{}
			_ = component.GetObjectListByComponentName(ctx, k8sClient, *cluster, stsList, consensusCompName)
			Expect(len(stsList.Items) > 0).Should(BeTrue())

			By("test getObjectListByCustomLabels function")
			stsList = &appsv1.StatefulSetList{}
			matchLabel := constant.GetComponentWellKnownLabels(cluster.Name, consensusCompName)
			_ = getObjectListByCustomLabels(ctx, k8sClient, *cluster, stsList, client.MatchingLabels(matchLabel))
			Expect(len(stsList.Items) > 0).Should(BeTrue())

			By("test getClusterComponentSpecByName function")
			clusterComp := getClusterComponentSpecByName(*cluster, consensusCompName)
			Expect(clusterComp).ShouldNot(BeNil())

			By("test GetComponentStsMinReadySeconds")
			minReadySeconds, _ := component.GetComponentWorkloadMinReadySeconds(ctx, k8sClient, *cluster,
				appsv1alpha1.Stateless, statelessCompName)
			Expect(minReadySeconds).To(Equal(int32(10)))
			minReadySeconds, _ = component.GetComponentWorkloadMinReadySeconds(ctx, k8sClient, *cluster,
				appsv1alpha1.Consensus, statelessCompName)
			Expect(minReadySeconds).To(Equal(int32(0)))

			By("test getCompRelatedObjectList function")
			stsList = &appsv1.StatefulSetList{}
			podList, _ := getCompRelatedObjectList(ctx, k8sClient, *cluster, consensusCompName, stsList)
			Expect(len(stsList.Items) > 0 && len(podList.Items) > 0).Should(BeTrue())
		})
	})

	Context("Custom Label test", func() {
		Context("parseCustomLabelPattern func", func() {
			It("should parse pattern well", func() {
				pattern := "v1/Pod"
				gvk, err := parseCustomLabelPattern(pattern)
				Expect(err).Should(BeNil())
				Expect(gvk.Group).Should(BeEmpty())
				Expect(gvk.Version).Should(Equal("v1"))
				Expect(gvk.Kind).Should(Equal("Pod"))
				pattern = "apps/v1/StatefulSet"
				gvk, err = parseCustomLabelPattern(pattern)
				Expect(err).Should(BeNil())
				Expect(gvk.Group).Should(Equal("apps"))
				Expect(gvk.Version).Should(Equal("v1"))
				Expect(gvk.Kind).Should(Equal("StatefulSet"))
			})
		})

		// TODO(xingran: add test case for updateCustomLabelToObjs func
		Context("updateCustomLabelToPods func", func() {
			It("should work well", func() {
				_, _, cluster := testapps.InitClusterWithHybridComps(&testCtx, clusterDefName,
					clusterVersionName, clusterName, statelessCompName, "stateful", consensusCompName)
				sts := testapps.MockConsensusComponentStatefulSet(&testCtx, clusterName, consensusCompName)
				pods := testapps.MockConsensusComponentPods(&testCtx, sts, clusterName, consensusCompName)
				mockLabelKey := "mock-label-key"
				mockLabelPlaceHolderValue := "$(KB_CLUSTER_NAME)-$(KB_COMP_NAME)"
				customLabels := map[string]appsv1alpha1.BuiltInString{
					mockLabelKey: appsv1alpha1.BuiltInString(mockLabelPlaceHolderValue),
				}
				comp := &component.SynthesizedComponent{
					Name:   consensusCompName,
					Labels: customLabels,
				}

				dag := graph.NewDAG()
				dag.AddVertex(&model.ObjectVertex{Obj: pods[0], Action: model.ActionUpdatePtr()})
				Expect(UpdateCustomLabelToPods(testCtx.Ctx, k8sClient, cluster, comp, dag)).Should(Succeed())
				graphCli := model.NewGraphClient(k8sClient)
				podList := graphCli.FindAll(dag, &corev1.Pod{})
				Expect(podList).Should(HaveLen(3))
				for _, pod := range podList {
					Expect(pod.GetLabels()).ShouldNot(BeNil())
					Expect(pod.GetLabels()[mockLabelKey]).Should(Equal(fmt.Sprintf("%s-%s", clusterName, comp.Name)))
				}
			})
		})
	})

	Context("test mergeServiceAnnotations", func() {
		It("test sync pod spec default values set by k8s", func() {
			var (
				clusterName = "cluster"
				compName    = "component"
				podName     = "pod"
				role        = "leader"
				mode        = "ReadWrite"
			)
			pod := testapps.MockConsensusComponentStsPod(&testCtx, nil, clusterName, compName, podName, role, mode)
			ppod := testapps.NewPodFactory(testCtx.DefaultNamespace, "pod").
				SetOwnerReferences("apps/v1", constant.StatefulSetKind, nil).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(compName).
				AddAppManagedByLabel().
				AddRoleLabel(role).
				AddConsensusSetAccessModeLabel(mode).
				AddControllerRevisionHashLabel("").
				AddContainer(corev1.Container{
					Name:  testapps.DefaultMySQLContainerName,
					Image: testapps.ApeCloudMySQLImage,
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/hello",
								Port: intstr.FromInt(1024),
							},
						},
						TimeoutSeconds:   1,
						PeriodSeconds:    1,
						FailureThreshold: 1,
					},
					StartupProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt(1024),
							},
						},
					},
				}).
				GetObject()
			ResolvePodSpecDefaultFields(pod.Spec, &ppod.Spec)
			Expect(reflect.DeepEqual(pod.Spec, ppod.Spec)).Should(BeTrue())
		})
	})
})
