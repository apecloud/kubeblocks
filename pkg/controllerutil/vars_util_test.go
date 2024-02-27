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

package controllerutil

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("vars util test", func() {

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("vars util test", func() {

		It("test functions in vars_util.go", func() {
			cmEnvKey := "CM_ENV_1"
			envKey := "ENV_1"
			envValue := "value_1"
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: testCtx.DefaultNamespace,
				},
				Data: map[string]string{
					cmEnvKey: "CM_VALUE_1",
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())

			pod := testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod").AddContainer(corev1.Container{
				Name:    "c1",
				Image:   "test:latest",
				Command: []string{"sh", "-c", "echo 1"},
				Env: []corev1.EnvVar{
					{
						Name:  envKey,
						Value: envValue,
					},
					{
						Name: "ENV_2",
						ValueFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: cm.Name,
								},
								Key: cmEnvKey,
							},
						},
					},
				},
				EnvFrom: []corev1.EnvFromSource{
					{
						ConfigMapRef: &corev1.ConfigMapEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cm.Name,
							},
						},
					},
				},
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 9090,
						Name:          "p1",
					},
				},
			}).Create(&testCtx).GetObject()

			By("test GetEnvVarsFromEnvFrom function")
			envFromMap, _ := GetEnvVarsFromEnvFrom(ctx, k8sClient, pod.Namespace, &pod.Spec.Containers[0])
			Expect(len(envFromMap)).Should(Equal(1))

			By("test GetEnvVarsFromEnvFrom function")
			envVar := BuildVarWithEnv(pod, &pod.Spec.Containers[0], envKey)
			Expect(envVar.Value).Should(Equal(envValue))

			By("test BuildVarWithFieldPath function")
			envVar, _ = BuildVarWithFieldPath(pod, ".spec.containers[0].ports[0].containerPort")
			Expect(envVar.Value).Should(Equal(strconv.Itoa(9090)))
		})
	})
})
