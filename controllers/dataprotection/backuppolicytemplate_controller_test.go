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

package dataprotection

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("BackupPolicyTemplate Controller", func() {

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.BackupToolSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	assureBackupPolicyTemplateObj := func(backupTool string) *dataprotectionv1alpha1.BackupPolicyTemplate {
		By("By assure an backupPolicyTemplate obj")
		return testapps.NewBackupPolicyTemplateFactory("backup-policy-template-").
			WithRandomName().
			SetBackupToolName(backupTool).
			SetSchedule("0 3 * * *").
			SetTTL("168h0m0s").
			Create(&testCtx).GetObject()
	}

	assureBackupToolObj := func() *dataprotectionv1alpha1.BackupTool {
		By("By assure an backupTool obj")
		return testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
			&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName())
	}

	Context("When creating backupPolicyTemplate", func() {
		It("Should success with no error", func() {

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicyTemplate from backupTool: " + backupTool.Name)
			toCreate := assureBackupPolicyTemplateObj(backupTool.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}
			result := &dataprotectionv1alpha1.BackupPolicyTemplate{}
			Eventually(k8sClient.Get(ctx, key, result)).Should(Succeed())
		})
	})

})
