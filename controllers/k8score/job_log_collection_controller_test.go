/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package k8score

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dpbackup "github.com/apecloud/kubeblocks/pkg/dataprotection/backup"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("Log Collection Controller", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
		testapps.ClearResources(&testCtx, generics.VolumeSnapshotClassSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
		_ = testdp.NewFakeCluster(&testCtx)
	})

	AfterEach(cleanEnv)

	When("test log collection when backup and restore jobs are failed", func() {

		BeforeEach(func() {

			By("creating an actionSet")
			actionSet := testdp.NewFakeActionSet(&testCtx)

			By("creating storage provider")
			_ = testdp.NewFakeStorageProvider(&testCtx, nil)

			By("creating backup repo")
			_, _ = testdp.NewFakeBackupRepo(&testCtx, nil)

			By("creating a backupPolicy from actionSet: " + actionSet.Name)
			testdp.NewFakeBackupPolicy(&testCtx, nil)
		})

		Context("test log collection", func() {
			var (
				backup *dpv1alpha1.Backup
			)
			BeforeEach(func() {
				backup = testdp.NewFakeBackup(&testCtx, nil)
			})

			It("test when backup job is failed", func() {
				jobKey := client.ObjectKey{
					Name:      dpbackup.GenerateBackupJobName(backup, dpbackup.BackupDataJobNamePrefix+"-0"),
					Namespace: backup.Namespace,
				}
				Eventually(testapps.CheckObj(&testCtx, jobKey, func(g Gomega, fetched *batchv1.Job) {
					// image should be expanded by env
					g.Expect(fetched.OwnerReferences[0].Name).Should(Equal(backup.Name))
				})).Should(Succeed())

				job := &batchv1.Job{}
				Expect(k8sClient.Get(ctx, jobKey, job)).ShouldNot(HaveOccurred())

				testapps.NewPodFactory(testCtx.DefaultNamespace, jobKey.Name+"-rsdjrk").
					AddContainer(corev1.Container{Name: "test", Image: testdp.ImageTag}).
					AddLabels("job-name", jobKey.Name).
					SetOwnerReferences("batch/v1", constant.JobKind, job).Create(&testCtx)

				// patch jobs failed
				testdp.PatchK8sJobStatus(&testCtx, jobKey, batchv1.JobFailed)

				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, fetched *dpv1alpha1.Backup) {
					// NOTE: env test can not create a real pod, so FailureReason will always empty
					g.Expect(fetched.Status.FailureReason).Should(BeEmpty())
				})).Should(Succeed())
			})
		})
	})
})
