/*
Copyright ApeCloud, Inc.

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

package v1alpha1

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("clusterVersion webhook", func() {
	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "webhook-mysql-definition-" + randomStr
		clusterVersionName    = "webhook-mysql-clusterversion-" + randomStr
		timeout               = time.Second * 10
		interval              = time.Second
	)
	cleanupObjects := func() {
		// Add any setup steps that needs to be executed before each test
		err := k8sClient.DeleteAllOf(ctx, &ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
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
	Context("When clusterVersion create and update", func() {
		It("Should webhook validate passed", func() {
			By("By testing create a new clusterVersion when clusterDefinition not exist")
			clusterVersion := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName)
			Expect(testCtx.CreateObj(ctx, clusterVersion)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(testCtx.CreateObj(ctx, clusterDef)).Should(Succeed())

			Eventually(func() bool {
				By("By testing component name is not found in cluserDefinition")
				clusterVersion.Spec.ComponentVersions[1].ComponentDefRef = "proxy1"
				Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).ShouldNot(Succeed())

				By("By creating an clusterVersion")
				clusterVersion.Spec.ComponentVersions[1].ComponentDefRef = "proxy"
				err := testCtx.CheckedCreateObj(ctx, clusterVersion)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By testing create a new clusterVersion with invalid config template")
			clusterVersionDup := createTestClusterVersionObj(clusterDefinitionName, clusterVersionName+"-for-config")
			clusterVersionDup.Spec.ComponentVersions[0].ConfigSpecs = []ComponentConfigSpec{
				{
					ComponentTemplateSpec: ComponentTemplateSpec{
						Name:        "tpl1",
						TemplateRef: "cm1",
						VolumeName:  "volume1",
					},
					ConfigConstraintRef: "constraint1",
				},
				{
					ComponentTemplateSpec: ComponentTemplateSpec{
						Name:        "tpl2",
						TemplateRef: "cm2",
						VolumeName:  "volume1",
					},
					ConfigConstraintRef: "constraint2",
				},
			}
			err := testCtx.CreateObj(ctx, clusterVersionDup)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("volume[volume1] already existed."))

			By("By testing update clusterVersion.status")
			patch := client.MergeFrom(clusterVersion.DeepCopy())
			clusterVersion.Status.Message = "Hello, kubeblocks!"
			Expect(k8sClient.Status().Patch(ctx, clusterVersion, patch)).Should(Succeed())

			By("By testing update clusterVersion.spec")
			patch = client.MergeFrom(clusterVersion.DeepCopy())
			clusterVersion.Spec.ClusterDefinitionRef = "test1"
			Expect(k8sClient.Patch(ctx, clusterVersion, patch)).ShouldNot(Succeed())
		})
	})
})

func createTestClusterVersionObj(clusterDefinitionName, clusterVersionName string) *ClusterVersion {
	clusterVersion := &ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterVersionName,
			Namespace: "default",
		},
		Spec: ClusterVersionSpec{
			ClusterDefinitionRef: clusterDefinitionName,
			ComponentVersions: []ClusterComponentVersion{
				{
					ComponentDefRef: "replicasets",
					VersionsCtx: VersionsContext{Containers: []corev1.Container{
						{Name: "main"},
					}}},
				{
					ComponentDefRef: "proxy",
					VersionsCtx: VersionsContext{Containers: []corev1.Container{
						{Name: "main"},
					}}},
			},
		},
		Status: ClusterVersionStatus{},
	}
	return clusterVersion
}

// createTestReplicationSetClusterVersionObj create a replication clusterVersion object, other webhook_test called this function, carefully for modifying the function.
func createTestReplicationSetClusterVersionObj(clusterDefinitionName, clusterVersionName string) *ClusterVersion {
	clusterVersion := &ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterVersionName,
			Namespace: "default",
		},
		Spec: ClusterVersionSpec{
			ClusterDefinitionRef: clusterDefinitionName,
			ComponentVersions: []ClusterComponentVersion{
				{
					ComponentDefRef: "redis",
					VersionsCtx: VersionsContext{
						Containers: []corev1.Container{
							{Name: "main"},
						}}},
			},
		},
		Status: ClusterVersionStatus{},
	}
	return clusterVersion
}
