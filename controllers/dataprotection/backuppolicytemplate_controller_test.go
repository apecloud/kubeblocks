/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package dataprotection

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("", func() {
	var (
		BackupPolicyTemplateName = "test-bpt"
		BackupMethod             = "test-bm"
		VsBackupMethodName       = "test-vs-bm"
		ttl                      = "7d"
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.BackupPolicyTemplateSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("create a BackupPolicyTemplate", func() {
		It("test BackupPolicyTemplate", func() {
			bpt := testdp.NewBackupPolicyTemplateFactory(BackupPolicyTemplateName).
				AddBackupMethod(BackupMethod, false, testdp.ActionSetName).
				SetBackupMethodVolumeMounts("data", "/data").
				AddBackupMethod(VsBackupMethodName, true, "").
				SetBackupMethodVolumeMounts("data", "/data").
				AddSchedule(BackupMethod, "0 0 * * *", ttl, true).
				AddSchedule(VsBackupMethodName, "0 0 * * *", ttl, true).
				Create(&testCtx).GetObject()
			key := client.ObjectKeyFromObject(bpt)

			By("should be unavailable")
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, pobj *dpv1alpha1.BackupPolicyTemplate) {
				g.Expect(pobj.Status.ObservedGeneration).To(Equal(bpt.Generation))
				g.Expect(pobj.Status.Phase).To(Equal(dpv1alpha1.UnavailablePhase))
				g.Expect(pobj.Status.Message).To(ContainSubstring(fmt.Sprintf(`ActionSet "%s" not found`, testdp.ActionSetName)))
			})).Should(Succeed())

			By("should be available")
			testdp.NewFakeActionSet(&testCtx)
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, pobj *dpv1alpha1.BackupPolicyTemplate) {
				g.Expect(pobj.Status.ObservedGeneration).To(Equal(bpt.Generation))
				g.Expect(pobj.Status.Phase).To(Equal(dpv1alpha1.AvailablePhase))
				g.Expect(pobj.Status.Message).To(BeEmpty())
			})).Should(Succeed())
		})
	})

})
