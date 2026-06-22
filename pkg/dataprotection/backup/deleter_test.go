/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package backup

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestDeleterDoPreDeleteActionCreatesAndReusesJob(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, batchv1.AddToScheme(scheme))
	assert.NoError(t, dpv1alpha1.AddToScheme(scheme))

	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns", UID: types.UID("backup-uid")},
		Status:     dpv1alpha1.BackupStatus{BackupMethod: &dpv1alpha1.BackupMethod{Env: []corev1.EnvVar{{Name: "IMAGE_TAG", Value: "1.0"}}}},
	}
	repo := &dpv1alpha1.BackupRepo{Spec: dpv1alpha1.BackupRepoSpec{}, Status: dpv1alpha1.BackupRepoStatus{BackupPVCName: "repo-pvc"}}
	deleter := &Deleter{
		RequestCtx:           ctrlutil.RequestCtx{Ctx: context.Background()},
		Client:               cli,
		Scheme:               scheme,
		WorkerServiceAccount: "worker",
		actionSet:            &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{Env: []corev1.EnvVar{{Name: "ACTION_ENV", Value: "set"}}}},
	}

	job, err := deleter.doPreDeleteAction(backup, repo, &dpv1alpha1.BaseJobActionSpec{Image: "deleter:$(IMAGE_TAG)", Command: []string{"delete"}}, "", "/backup/path")
	assert.NoError(t, err)
	assert.Empty(t, job.Name)

	got := &batchv1.Job{}
	assert.NoError(t, cli.Get(context.Background(), BuildDeleteBackupFilesJobKey(backup, true), got))
	assert.Contains(t, got.Spec.Template.Spec.Containers[0].Image, "deleter:1.0")
	assert.Equal(t, "worker", got.Spec.Template.Spec.ServiceAccountName)
	envMap := map[string]string{}
	for _, env := range got.Spec.Template.Spec.Containers[0].Env {
		envMap[env.Name] = env.Value
	}
	assert.Equal(t, "/backup/path", envMap[dptypes.DPBackupBasePath])
	assert.Equal(t, "set", envMap["ACTION_ENV"])

	job, err = deleter.doPreDeleteAction(backup, repo, &dpv1alpha1.BaseJobActionSpec{Image: "deleter:$(IMAGE_TAG)", Command: []string{"delete"}}, "", "/backup/path")
	assert.NoError(t, err)
	assert.Equal(t, got.Name, job.Name)
}

var _ = Describe("Backup Deleter Test", func() {
	const (
		backupRepoPVCName = "backup-repo-pvc"
		backupPath        = "/backup/test-backup"
		backupVSName      = "backup-vs"
		backupPVCName     = "backup-pvc"
	)

	buildDeleter := func() *Deleter {
		return &Deleter{
			RequestCtx: ctrlutil.RequestCtx{
				Log:      logger,
				Ctx:      testCtx.Ctx,
				Recorder: recorder,
			},
			Scheme: testEnv.Scheme,
			Client: testCtx.Cli,
		}
	}

	cleanEnv := func() {
		By("clean resources")
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		testapps.ClearResources(&testCtx, generics.VolumeSnapshotSignature, inNS)
	}

	BeforeEach(func() {
		cleanEnv()
		viper.Set(constant.KBToolsImage, testdp.KBToolImage)
	})

	AfterEach(func() {
		cleanEnv()
		viper.Set(constant.KBToolsImage, "")
	})

	Context("delete backup file", func() {
		var (
			backup  *dpv1alpha1.Backup
			deleter *Deleter
		)

		BeforeEach(func() {
			backup = testdp.NewFakeBackup(&testCtx, nil)
			deleter = buildDeleter()
		})

		It("should success when backup status PVC is empty", func() {
			Expect(backup.Status.PersistentVolumeClaimName).Should(Equal(""))
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusSucceeded))
		})

		It("should success when backup status path is empty", func() {
			backup.Status.PersistentVolumeClaimName = backupRepoPVCName
			Expect(backup.Status.Path).Should(Equal(""))
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusSucceeded))
		})

		It("should success when PVC does not exist", func() {
			backup.Status.PersistentVolumeClaimName = backupRepoPVCName
			backup.Status.Path = backupPath
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusSucceeded))
		})

		It("should ensure worker service account only when creating a deletion job", func() {
			ensureWorkerServiceAccountCalls := 0
			deleter.EnsureWorkerServiceAccount = func() (string, error) {
				ensureWorkerServiceAccountCalls++
				return "worker-sa", nil
			}

			By("skipping worker service account creation for snapshot backups")
			backup.Status.BackupMethod = &dpv1alpha1.BackupMethod{
				SnapshotVolumes: pointer.Bool(true),
			}
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusSucceeded))
			Expect(ensureWorkerServiceAccountCalls).Should(Equal(0))

			By("creating worker service account when a deletion job is needed")
			backup.Status.BackupMethod = nil
			backupRepoPVC := testdp.NewFakePVC(&testCtx, backupRepoPVCName)
			backup.Status.PersistentVolumeClaimName = backupRepoPVC.Name
			backup.Status.Path = backupPath
			status, err = deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusDeleting))
			Expect(ensureWorkerServiceAccountCalls).Should(Equal(1))

			key := BuildDeleteBackupFilesJobKey(backup, false)
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, fetched *batchv1.Job) {
				g.Expect(fetched.Spec.Template.Spec.ServiceAccountName).Should(Equal("worker-sa"))
			})).Should(Succeed())
		})

		It("should create job to delete backup file", func() {
			By("mock backup repo PVC")
			backupRepoPVC := testdp.NewFakePVC(&testCtx, backupRepoPVCName)

			By("delete backup file")
			backup.Status.PersistentVolumeClaimName = backupRepoPVC.Name
			backup.Status.Path = backupPath
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusDeleting))

			By("check job exist")
			job := &batchv1.Job{}
			key := BuildDeleteBackupFilesJobKey(backup, false)
			Eventually(testapps.CheckObjExists(&testCtx, key, job, true)).Should(Succeed())

			By("delete backup with job running")
			backupKey := client.ObjectKeyFromObject(backup)
			Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
				status, err := deleter.DeleteBackupFiles(fetched)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusDeleting))
			})).Should(Succeed())

			By("delete backup with job succeed")
			testdp.ReplaceK8sJobStatus(&testCtx, key, batchv1.JobComplete)
			Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
				status, err := deleter.DeleteBackupFiles(fetched)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusSucceeded))
			})).Should(Succeed())

			By("delete backup with job failed")
			testdp.ReplaceK8sJobStatus(&testCtx, key, batchv1.JobFailed)
			Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
				status, err := deleter.DeleteBackupFiles(fetched)
				Expect(err).Should(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusFailed))
			})).Should(Succeed())
		})

		It("delete backup with backup repo", func() {
			backup.Status.BackupRepoName = testdp.BackupRepoName
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusSucceeded))
		})
	})

	Context("delete volume snapshots", func() {
		var (
			backup  *dpv1alpha1.Backup
			deleter *Deleter
		)

		BeforeEach(func() {
			backup = testdp.NewFakeBackup(&testCtx, nil)
			deleter = buildDeleter()
		})

		It("should success when volume snapshot does not exist", func() {
			Expect(deleter.DeleteVolumeSnapshots(backup)).Should(Succeed())
		})

		It("should success when volume snapshot exist", func() {
			By("mock volume snapshot")
			vs := testdp.NewVolumeSnapshotFactory(testCtx.DefaultNamespace, backupVSName).
				SetSourcePVCName(backupPVCName).
				AddLabelsInMap(BuildBackupWorkloadLabels(backup)).
				Create(&testCtx).GetObject()
			Eventually(testapps.CheckObjExists(&testCtx,
				client.ObjectKeyFromObject(vs), vs, true)).Should(Succeed())

			By("delete volume snapshot")
			Expect(deleter.DeleteVolumeSnapshots(backup)).Should(Succeed())

			By("check volume snapshot deleted")
			Eventually(testapps.CheckObjExists(&testCtx,
				client.ObjectKeyFromObject(vs), vs, false)).Should(Succeed())
		})
	})
})
