/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package v1alpha1

import (
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
				By("By testing component name is not found in clusterDefinition")
				clusterVersion.Spec.ComponentVersions[1].ComponentDefRef = "proxy1"
				Expect(testCtx.CheckedCreateObj(ctx, clusterVersion)).ShouldNot(Succeed())

				By("By creating an clusterVersion")
				clusterVersion.Spec.ComponentVersions[1].ComponentDefRef = "proxy"
				err := testCtx.CheckedCreateObj(ctx, clusterVersion)
				return err == nil
			}).Should(BeTrue())

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
