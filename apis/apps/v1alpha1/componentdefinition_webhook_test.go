/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("componentDefinition webhook", func() {
	var (
		randomStr               = testCtx.GetRandomStr()
		componentDefinitionName = "webhook-compdef-" + randomStr
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
	Context("When create and update", func() {
		It("Should webhook validate passed", func() {
			By("By creating a new componentDefinition")
			compDef := createTestComponentDefObj(componentDefinitionName)
			Expect(testCtx.CreateObj(ctx, compDef)).Should(Succeed())

			// TODO: add more test cases
		})
	})
})

func createTestComponentDefObj(compDefName string) *ComponentDefinition {
	compDef := &ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      compDefName,
			Namespace: "default",
		},
		Spec: ComponentDefinitionSpec{
			ServiceKind:    "test",
			ServiceVersion: "test-version",
			Runtime: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "foo",
						Image: "bar",
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "for_test",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/tmp",
							},
						},
					},
				},
			},
			Volumes: []ComponentVolume{
				{
					Name:          "for_test",
					NeedSnapshot:  true,
					HighWatermark: 80,
				},
			},
		},
		Status: ComponentDefinitionStatus{},
	}
	return compDef
}
