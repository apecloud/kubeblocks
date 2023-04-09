package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("", func() {

	var componentClassDefinition *v1alpha1.ComponentClassDefinition

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResources(&testCtx, intctrlutil.ClassFamilySignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ComponentClassDefinitionSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	It("Class should exist in status", func() {
		var (
			className = "test"
		)
		class := v1alpha1.ComponentClass{
			Name:   className,
			CPU:    "1",
			Memory: "2Gi",
		}
		componentClassDefinition = testapps.NewComponentClassDefinitionFactory("custom", "apecloud-mysql", "mysql").
			AddClass(class).Create(&testCtx).GetObject()
		key := client.ObjectKeyFromObject(componentClassDefinition)
		Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, pobj *v1alpha1.ComponentClassDefinition) {
			g.Expect(pobj.Status.Classes).ShouldNot(BeEmpty())
			g.Expect(pobj.Status.Classes[0].Name).Should(Equal(className))
		})).Should(Succeed())
	})
})
