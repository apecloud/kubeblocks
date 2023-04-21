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

package envcheck

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"

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
		dataprotectionv1alpha1.GroupVersion.Group,
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
