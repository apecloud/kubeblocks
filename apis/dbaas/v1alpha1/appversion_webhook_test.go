/*
Copyright 2022 The KubeBlocks Authors

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
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("appVersion webhook", func() {
	var (
		clusterDefinitionName = "appversion-webhook-mysql-definition"
		appVersionName        = "appversion-webhook-mysql-appversion"
		timeout               = time.Second * 10
		interval              = time.Second
	)
	Context("When appVersion create and update", func() {
		It("Should webhook validate passed", func() {
			By("By testing create a new appVersion when clusterDefinition not exist")
			appVersion := createTestAppVersionObj(clusterDefinitionName, appVersionName)
			Expect(k8sClient.Create(ctx, appVersion)).ShouldNot(Succeed())

			By("By creating a new clusterDefinition")
			clusterDef, _ := createTestClusterDefinitionObj(clusterDefinitionName)
			Expect(k8sClient.Create(ctx, clusterDef)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), client.ObjectKey{Name: clusterDef.Name}, clusterDef)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By testing component type is not found in cluserDefinition")
			appVersion.Spec.Components[1].Type = "proxy1"
			Expect(k8sClient.Create(ctx, appVersion)).ShouldNot(Succeed())

			By("By creating an appVersion")
			appVersion.Spec.Components[1].Type = "proxy"
			Expect(k8sClient.Create(ctx, appVersion)).Should(Succeed())

			By("By testing update appVersion.status")
			appVersion.Status.ClusterDefSyncStatus = OutOfSyncStatus
			Expect(k8sClient.Update(ctx, appVersion)).Should(Succeed())

			By("By testing update appVersion.spec ")
			appVersion.Spec.ClusterDefinitionRef = "test"
			Expect(k8sClient.Update(ctx, appVersion)).ShouldNot(Succeed())

		})
	})
})

func createTestAppVersionObj(clusterDefinitionName, appVersionName string) *AppVersion {
	appVersion := &AppVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersion,
			Kind:       AppVersionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      appVersionName,
			Namespace: "default",
		},
		Spec: AppVersionSpec{
			ClusterDefinitionRef: clusterDefinitionName,
			Components: []AppVersionComponent{
				{Type: "replicaSets", PodSpec: &corev1.PodSpec{Containers: []corev1.Container{
					{Name: "main"},
				}}},
				{Type: "proxy", PodSpec: &corev1.PodSpec{Containers: []corev1.Container{
					{Name: "main"},
				}}},
			},
		},
		Status: AppVersionStatus{},
	}
	return appVersion
}
