package envcheck

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/apecloud/kubeblocks/test/e2e"

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
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
		dbaasv1alpha1.GroupVersion.Group,
	}

	crdList := &apiextv1.CustomResourceDefinitionList{}
	Expect(K8sClient.List(Ctx, crdList)).To(Succeed())
	for _, crd := range crdList.Items {
		for _, g := range apiGroups {
			Expect(strings.Contains(crd.Spec.Group, g)).Should(BeFalse())
		}
	}
}
