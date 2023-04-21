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

package apps

import (
	"strings"

	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/generics"
	"github.com/apecloud/kubeblocks/internal/testutil"
)

// ClearBackups clears all Backup resources satisfying the input ListOptions.
func ClearBackups(testCtx *testutil.TestContext, opts ...client.DeleteAllOfOption) {
	ClearResourcesWithCleaner(testCtx, generics.BackupSignature,
		func(g gomega.Gomega, backup *dataprotectionv1alpha1.Backup) {
			jobList := batchv1.JobList{}
			g.Expect(testCtx.Cli.List(testCtx.Ctx, &jobList,
				client.InNamespace(testCtx.DefaultNamespace),
				client.HasLabels{testCtx.TestObjLabelKey})).Should(gomega.Succeed())
			for _, job := range jobList.Items {
				// When deleting a Backup object, it will create a Job to delete the corresponding backup file.
				// In unit tests, the Job cannot enter the completed state, causing the Backup object to
				// never be deleted. Therefore, here we manually set the Job to the completed state.
				if strings.HasSuffix(job.Name, backup.Name) {
					completed := false
					for _, cond := range job.Status.Conditions {
						if cond.Type == batchv1.JobComplete {
							completed = true
						}
					}
					if !completed {
						g.Expect(ChangeObjStatus(testCtx, &job, func() {
							job.Status.Conditions = append(job.Status.Conditions,
								batchv1.JobCondition{
									Type: batchv1.JobComplete,
								})
						})).Should(gomega.Succeed())
					}
				}
			}
		}, opts...)
}

// ClearBackupResources clears all Backup and other related resources satisfying the input ListOptions.
func ClearBackupResources(testCtx *testutil.TestContext, opts ...client.DeleteAllOfOption) {
	ClearBackups(testCtx, opts...)
	ClearResources(testCtx, generics.BackupPolicySignature, opts...)
}
