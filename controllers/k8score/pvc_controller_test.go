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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("PersistentVolumeClaim Controller", func() {
	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.PersistentVolumeClaimSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	createPVC := func(pvcName string) *corev1.PersistentVolumeClaim {
		By("By assure an default storageClass")
		return testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, "apecloud-mysql",
			"consensus", "data").SetStorage("2Gi").
			SetStorageClass("csi-hostpath-sc").Create(&testCtx).GetObject()
	}

	handlePersistentVolumeClaim := func(reqCtx intctrlutil.RequestCtx, cli client.Client, pvc *corev1.PersistentVolumeClaim) error {
		patch := client.MergeFrom(pvc.DeepCopy())
		pvc.Annotations["kubeblocks.io/test"] = "test_pvc"
		return cli.Patch(ctx, pvc, patch)
	}

	Context("test creating PersistentVolumeClaim", func() {
		It("should handle it properly", func() {
			By("register an pvcHandler for testing")
			PersistentVolumeClaimHandlerMap["pvc-controller"] = handlePersistentVolumeClaim

			By("test PersistentVolumeClaim changes")
			pvcName := fmt.Sprintf("pvc-%s", testCtx.GetRandomStr())
			pvc := createPVC(pvcName)
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(pvc), func(tmpPvc *corev1.PersistentVolumeClaim) {
				pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("4Gi")
			})()).Should(Succeed())

			// wait until pvc patched the annotation by storageClass controller.
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pvc), func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				g.Expect(tmpPVC.Annotations["kubeblocks.io/test"] == "test_pvc").Should(BeTrue())
			})).Should(Succeed())
		})
	})
})
