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

package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("", func() {
	var (
		BackupPolicyTemplateName = "test-bpt"
		ClusterDefName           = "test-cd"
		BackupPolicyName         = "test-bp"
		BackupMethod             = "test-bm"
		ActionSetName            = "test-as"
		VsBackupMethodName       = "test-vs-bm"
		VsActionSetName          = "test-vs-as"
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

	Context("create a backuppolicytemplate", func() {
		It("should be available", func() {
			bpt := testapps.NewBackupPolicyTemplateFactory(BackupPolicyTemplateName).
				SetClusterDefRef(ClusterDefName).
				AddBackupPolicy(BackupPolicyName).
				AddBackupMethod(BackupMethod, false, ActionSetName).
				SetBackupMethodVolumeMounts("data", "/data").
				AddBackupMethod(VsBackupMethodName, true, VsActionSetName).
				SetBackupMethodVolumeMounts("data", "/data").
				AddSchedule(BackupMethod, "0 0 * * *", true).
				AddSchedule(VsBackupMethodName, "0 0 * * *", true).
				Create(&testCtx).GetObject()
			key := client.ObjectKeyFromObject(bpt)
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, pobj *v1alpha1.BackupPolicyTemplate) {
				g.Expect(pobj.GetLabels()[constant.ClusterDefLabelKey]).To(Equal(bpt.Spec.ClusterDefRef))
			})).Should(Succeed())
		})
	})

})
