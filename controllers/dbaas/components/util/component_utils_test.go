/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

func checkCompletedPhase(t *testing.T, phase dbaasv1alpha1.Phase) {
	isComplete := IsCompleted(phase)
	if !isComplete {
		t.Errorf("%s status is the completed status", phase)
	}
}

func TestIsCompleted(t *testing.T) {
	checkCompletedPhase(t, dbaasv1alpha1.FailedPhase)
	checkCompletedPhase(t, dbaasv1alpha1.RunningPhase)
	checkCompletedPhase(t, dbaasv1alpha1.AbnormalPhase)
}

func TestIsFailedOrAbnormal(t *testing.T) {
	if !IsFailedOrAbnormal(dbaasv1alpha1.AbnormalPhase) {
		t.Error("isAbnormal should be true")
	}
}

func TestIsProbeTimeout(t *testing.T) {
	podsReadyTime := &metav1.Time{Time: time.Now().Add(-2 * time.Minute)}
	if !IsProbeTimeout(podsReadyTime) {
		t.Error("probe timed out should be true")
	}
}

func TestCalculateComponentPhase(t *testing.T) {
	var (
		isFailed   = true
		isAbnormal = true
	)
	status := CalculateComponentPhase(isFailed, isAbnormal)
	if status != dbaasv1alpha1.FailedPhase {
		t.Error("function CalculateComponentPhase should return Failed")
	}
	isFailed = false
	status = CalculateComponentPhase(isFailed, isAbnormal)
	if status != dbaasv1alpha1.AbnormalPhase {
		t.Error("function CalculateComponentPhase should return Abnormal")
	}
	isAbnormal = false
	status = CalculateComponentPhase(isFailed, isAbnormal)
	if status != "" {
		t.Error(`function CalculateComponentPhase should return ""`)
	}
}

func TestGetComponentOrTypeName(t *testing.T) {
	var (
		componentType = "mysqlType"
		componentName = "mysql"
	)
	cluster := dbaasv1alpha1.Cluster{
		Spec: dbaasv1alpha1.ClusterSpec{
			Components: []dbaasv1alpha1.ClusterComponent{
				{Name: componentName, Type: componentType},
			},
		},
	}
	typeName := GetComponentTypeName(cluster, componentName)
	if typeName != componentType {
		t.Errorf(`function GetComponentTypeName should return %s`, componentType)
	}
	component := GetComponentByName(&cluster, componentName)
	if component == nil {
		t.Errorf("function GetComponentByName should not return nil")
	}
	componentName = "mysql1"
	typeName = GetComponentTypeName(cluster, componentName)
	if typeName != componentName {
		t.Errorf(`function GetComponentTypeName should return %s`, componentName)
	}
	component = GetComponentByName(&cluster, componentName)
	if component != nil {
		t.Error("function GetComponentByName should return nil")
	}
}

func TestGetComponentDefFromClusterDefinition(t *testing.T) {
	componentType := "mysqlType"
	clusterDef := &dbaasv1alpha1.ClusterDefinition{
		Spec: dbaasv1alpha1.ClusterDefinitionSpec{
			Components: []dbaasv1alpha1.ClusterDefinitionComponent{
				{
					TypeName: componentType,
				},
			},
		},
	}
	if GetComponentDefFromClusterDefinition(clusterDef, componentType) == nil {
		t.Error("function GetComponentTypeName should not return nil")
	}
	componentType = "test"
	if GetComponentDefFromClusterDefinition(clusterDef, componentType) != nil {
		t.Error("function GetComponentTypeName should return nil")
	}
}

var _ = Describe("Consensus Component", func() {
	var (
		randomStr          = testCtx.GetRandomStr()
		clusterDefName     = "mysql-clusterdef-" + randomStr
		clusterVersionName = "mysql-clusterversion-" + randomStr
		clusterName        = "mysql-" + randomStr
		timeout            = 10 * time.Second
		interval           = time.Second
		consensusCompType  = "consensus"
		consensusCompName  = "consensus"
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey},
			client.GracePeriodSeconds(0))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	Context("Consensus Component test", func() {
		It("Consensus Component test", func() {
			By(" init cluster, statefulSet, pods")
			_, _, cluster := testdbaas.InitConsensusMysql(ctx, testCtx, clusterDefName,
				clusterVersionName, clusterName, consensusCompName)
			sts := testdbaas.MockConsensusComponentStatefulSet(ctx, testCtx, clusterName, consensusCompName)
			if !testCtx.UsingExistingCluster() {
				_ = testdbaas.MockConsensusComponentPods(ctx, testCtx, clusterName, consensusCompName)
			} else {
				timeout = 3 * timeout
			}

			By("test GetComponentDeftByCluster function")
			componentDef, _ := GetComponentDeftByCluster(ctx, k8sClient, cluster, consensusCompType)
			Expect(componentDef != nil).Should(BeTrue())

			By("test GetClusterByObject function")
			newCluster, _ := GetClusterByObject(ctx, k8sClient, sts)
			Expect(newCluster != nil).Should(BeTrue())

			By("test GetComponentPodList function")
			Eventually(func() bool {
				podList, _ := GetComponentPodList(ctx, k8sClient, cluster, consensusCompName)
				return len(podList.Items) > 0
			}, timeout, interval).Should(BeTrue())

			By("test GetObjectListByComponentName function")
			stsList := &appsv1.StatefulSetList{}
			_ = GetObjectListByComponentName(ctx, k8sClient, cluster, stsList, consensusCompName)
			Expect(len(stsList.Items) > 0).Should(BeTrue())

			By("test CheckRelatedPodIsTerminating function")
			isTerminating, _ := CheckRelatedPodIsTerminating(ctx, k8sClient, cluster, consensusCompName)
			Expect(isTerminating).Should(BeFalse())

			By("test GetStatusComponentMessageKey function")
			Expect(GetStatusComponentMessageKey("Pod", "mysql-01")).To(Equal("Pod/mysql-01"))

			By("test GetComponentReplicas function")
			component := GetComponentByName(cluster, consensusCompName)
			Expect(GetComponentReplicas(component, componentDef)).To(Equal(int32(3)))
		})
	})
})
