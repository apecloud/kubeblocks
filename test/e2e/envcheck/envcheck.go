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

package envcheck

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"

	. "github.com/apecloud/kubeblocks/test/e2e"
)

func EnvCheckTest() {

	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("Real Kubernetes Cluster", func() {
		It("All components' statuses (componentstatuses/v1 API) are healthy", func() {
			csList := &corev1.ComponentStatusList{}
			Expect(K8sClient.List(Ctx, csList)).To(Succeed())
			Expect(len(csList.Items)).ShouldNot(Equal(0))
			for _, cs := range csList.Items {
				for _, csCond := range cs.Conditions {
					if csCond.Type != corev1.ComponentHealthy {
						continue
					}
					Expect(csCond.Status).Should(Equal(corev1.ConditionTrue))
				}
			}
		})
	})
}

func EnvGotCleanedTest() {

	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("Real Kubernetes Cluster", func() {
		It("Check no KubeBlocks CRD installed", CheckNoKubeBlocksCRDs)
	})
}

func CheckNoKubeBlocksCRDs() {
	apiGroups := []string{
		dpv1alpha1.GroupVersion.Group,
		appsv1alpha1.GroupVersion.Group,
	}

	crdList := &apiextv1.CustomResourceDefinitionList{}
	Expect(K8sClient.List(Ctx, crdList)).To(Succeed())
	for _, crd := range crdList.Items {
		for _, g := range apiGroups {
			Expect(strings.Contains(crd.Spec.Group, g)).Should(BeFalse())
		}
	}
}
