package apps

import (
	. "github.com/onsi/ginkgo/v2"

	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("", func() {

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("", func() {
		It("", func() {

		})
	})
})
