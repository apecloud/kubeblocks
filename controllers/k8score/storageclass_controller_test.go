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

package k8score

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("StorageClass Controller", func() {

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.StorageClassSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	createStorageClassObj := func(storageClassName string, allowVolumeExpansion bool) *storagev1.StorageClass {
		return testdbaas.CreateCustomizedObj(&testCtx, "operations/storageclass.yaml",
			&storagev1.StorageClass{}, testdbaas.CustomizeObjYAML(storageClassName, allowVolumeExpansion))
	}

	handleStorageClass := func(reqCtx intctrlutil.RequestCtx, cli client.Client, storageClass *storagev1.StorageClass) error {
		patch := client.MergeFrom(storageClass.DeepCopy())
		storageClass.Annotations["kubeblocks.io/test"] = "test"
		return cli.Patch(ctx, storageClass, patch)
	}

	Context("test storageClass controller", func() {
		It("should handle it properly", func() {
			By("create a storageClass and register a storageClassHandler")
			StorageClassHandlerMap["test-controller"] = handleStorageClass
			storageClassName := fmt.Sprintf("standard-%s", testCtx.GetRandomStr())
			sc := createStorageClassObj(storageClassName, true)

			By("test storageClass changes")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(sc), func(tmpSc *storagev1.StorageClass) {
				allowVolumeExpansion := true
				tmpSc.AllowVolumeExpansion = &allowVolumeExpansion
			})()).Should(Succeed())

			// wait until storageClass patched
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(sc), func(g Gomega, tempStorageClass *storagev1.StorageClass) {
				g.Expect(tempStorageClass.Annotations["kubeblocks.io/test"] == "test").Should(BeTrue())
			})).Should(Succeed())
		})
	})
})
