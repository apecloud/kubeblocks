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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Backup Deleter Test", func() {
	const (
		backupRepoPVCName  = "backup-repo-pvc"
		backupPath         = "/backup/test-backup"
		backupVSName       = "backup-vs"
		backupPVCName      = "backup-pvc"
		workerSAName       = "dp-worker"
		deleteJobNamespace = "delete-job-ns"
	)

	buildDeleter := func() *Deleter {
		return &Deleter{
			RequestCtx: ctrlutil.RequestCtx{
				Log:      logger,
				Ctx:      testCtx.Ctx,
				Recorder: recorder,
			},
			Scheme:               testEnv.Scheme,
			Client:               testCtx.Cli,
			WorkerServiceAccount: workerSAName,
		}
	}

	cleanEnv := func() {
		By("clean resources")
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		deleteJobNS := client.InNamespace(deleteJobNamespace)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, deleteJobNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		testapps.ClearResources(&testCtx, generics.VolumeSnapshotSignature, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true)
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
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, fetched *batchv1.Job) {
				g.Expect(fetched.Spec.Template.Spec.ServiceAccountName).Should(Equal(workerSAName))
				// In-namespace delete Jobs are scoped to the Backup CR via an
				// ownerReference and are GC'd by Kubernetes when the Backup is
				// deleted. They must not carry the external-only TTL — the
				// scoping decision for case-003 (only delete Jobs created in
				// a controller-side namespace need TTL fallback).
				g.Expect(fetched.Spec.TTLSecondsAfterFinished).Should(BeNil())
				g.Expect(fetched.OwnerReferences).ShouldNot(BeEmpty())
			})).Should(Succeed())

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

		It("should not get worker service account when deletion job already exists", func() {
			By("mock backup repo PVC")
			backupRepoPVC := testdp.NewFakePVC(&testCtx, backupRepoPVCName)

			By("create a deletion job")
			backup.Status.PersistentVolumeClaimName = backupRepoPVC.Name
			backup.Status.Path = backupPath
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusDeleting))

			key := BuildDeleteBackupFilesJobKey(backup, false)
			testdp.ReplaceK8sJobStatus(&testCtx, key, batchv1.JobComplete)

			By("delete backup with job succeed without resolving a worker service account")
			deleter.WorkerServiceAccount = ""
			workerFuncCalled := false
			deleter.WorkerServiceAccountFunc = func() (string, error) {
				workerFuncCalled = true
				return "", errors.New("worker service account should not be requested")
			}
			status, err = deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusSucceeded))
			Expect(workerFuncCalled).Should(BeFalse())
		})

		It("should get worker service account lazily when creating a deletion job", func() {
			By("mock backup repo PVC")
			backupRepoPVC := testdp.NewFakePVC(&testCtx, backupRepoPVCName)

			By("delete backup file")
			backup.Status.PersistentVolumeClaimName = backupRepoPVC.Name
			backup.Status.Path = backupPath
			deleter.WorkerServiceAccount = ""
			workerFuncCalled := 0
			deleter.WorkerServiceAccountFunc = func() (string, error) {
				workerFuncCalled++
				return "lazy-worker", nil
			}
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusDeleting))
			Expect(workerFuncCalled).Should(Equal(1))

			By("check job service account")
			key := BuildDeleteBackupFilesJobKey(backup, false)
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, fetched *batchv1.Job) {
				g.Expect(fetched.Spec.Template.Spec.ServiceAccountName).Should(Equal("lazy-worker"))
			})).Should(Succeed())
		})

		It("should create and clean up tool delete job in configured delete job namespace", func() {
			By("mock delete job namespace and tool BackupRepo")
			Expect(client.IgnoreAlreadyExists(testCtx.CreateObj(testCtx.Ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: deleteJobNamespace},
			}))).Should(Succeed())
			backupRepo := &dpv1alpha1.BackupRepo{
				ObjectMeta: metav1.ObjectMeta{Name: testdp.BackupRepoName},
				Spec: dpv1alpha1.BackupRepoSpec{
					StorageProviderRef: testdp.StorageProviderName,
					AccessMethod:       dpv1alpha1.AccessMethodTool,
					PVReclaimPolicy:    corev1.PersistentVolumeReclaimRetain,
				},
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, backupRepo)).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, backupRepo, func() {
				backupRepo.Status.ToolConfigSecretName = "backup-repo-tool-config"
			})).Should(Succeed())

			By("delete backup file through the controller namespace")
			backup.Status.BackupRepoName = backupRepo.Name
			backup.Status.Path = backupPath
			deleter.WorkerServiceAccount = ""
			deleter.DeleteJobNamespace = deleteJobNamespace
			var workerNamespace, preparedNamespace, recordedNamespace string
			deleter.WorkerServiceAccountForNamespaceFunc = func(namespace string) (string, error) {
				workerNamespace = namespace
				return workerSAName, nil
			}
			deleter.PrepareDeleteJobBackupRepoFunc = func(repo *dpv1alpha1.BackupRepo, namespace string) error {
				preparedNamespace = namespace
				return nil
			}
			deleter.RecordCleanupJobNamespaceFunc = func(b *dpv1alpha1.Backup, namespace string) error {
				recordedNamespace = namespace
				b.Status.CleanupJobNamespace = namespace
				return nil
			}
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusDeleting))
			Expect(workerNamespace).Should(Equal(deleteJobNamespace))
			Expect(preparedNamespace).Should(Equal(deleteJobNamespace))
			Expect(recordedNamespace).Should(Equal(deleteJobNamespace))
			Expect(backup.Status.CleanupJobNamespace).Should(Equal(deleteJobNamespace))

			By("check external delete job contract")
			key := BuildDeleteBackupFilesJobKey(backup, false)
			key.Namespace = deleteJobNamespace
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, fetched *batchv1.Job) {
				g.Expect(fetched.OwnerReferences).Should(BeEmpty())
				g.Expect(fetched.Labels[constant.AppManagedByLabelKey]).Should(Equal(dptypes.AppName))
				g.Expect(fetched.Labels[dptypes.BackupNameLabelKey]).Should(Equal(backup.Name))
				g.Expect(fetched.Labels[dptypes.BackupNamespaceLabelKey]).Should(Equal(backup.Namespace))
				g.Expect(fetched.Labels[DeleteBackupFilesJobLabelKey]).Should(Equal("true"))
				// External delete Jobs cannot use a cross-namespace
				// ownerReference, so a TTL bounds how long the completed
				// Job + Pod linger even when the best-effort
				// BackgroundDeleteObject path does not propagate after
				// the Backup CR finalizer is removed.
				g.Expect(fetched.Spec.TTLSecondsAfterFinished).ShouldNot(BeNil())
				g.Expect(*fetched.Spec.TTLSecondsAfterFinished).Should(Equal(externalDeleteJobTTLSecondsAfterFinished))
				g.Expect(fetched.Spec.Template.Spec.ServiceAccountName).Should(Equal(workerSAName))
				g.Expect(fetched.Spec.Template.Spec.Volumes).Should(ContainElement(WithTransform(func(v corev1.Volume) string {
					if v.Secret == nil {
						return ""
					}
					return v.Secret.SecretName
				}, Equal(backupRepo.Status.ToolConfigSecretName))))
			})).Should(Succeed())

			By("delete external job after it succeeds")
			testdp.ReplaceK8sJobStatus(&testCtx, key, batchv1.JobComplete)
			status, err = deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusSucceeded))
			Eventually(testapps.CheckObjExists(&testCtx, key, &batchv1.Job{}, false)).Should(Succeed())
		})

		It("should wait for existing external delete job when BackupRepo is missing", func() {
			By("mock delete job namespace and existing external delete job")
			Expect(client.IgnoreAlreadyExists(testCtx.CreateObj(testCtx.Ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: deleteJobNamespace},
			}))).Should(Succeed())
			backup.Status.BackupRepoName = "missing-repo"
			backup.Status.Path = backupPath
			deleter.DeleteJobNamespace = deleteJobNamespace
			key := BuildDeleteBackupFilesJobKey(backup, false)
			key.Namespace = deleteJobNamespace
			Expect(testCtx.CreateObj(testCtx.Ctx, &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: key.Namespace,
					Name:      key.Name,
					Labels: map[string]string{
						DeleteBackupFilesJobLabelKey: "true",
					},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: deleteContainerName, Image: testdp.KBToolImage}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			})).Should(Succeed())

			By("delete backup with external job still running")
			status, err := deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusDeleting))

			By("delete backup after external job succeeds")
			testdp.ReplaceK8sJobStatus(&testCtx, key, batchv1.JobComplete)
			status, err = deleter.DeleteBackupFiles(backup)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status).Should(Equal(DeletionStatusSucceeded))
		})

		Context("when BackupRepo is deleted before cleanup completes", func() {
			const (
				externalNS    = deleteJobNamespace
				wrongDefault  = "wrong-default-ns"
				missingRepo   = "missing-repo"
				cleanupLabel  = DeleteBackupFilesJobLabelKey
			)

			ensureNS := func(ns string) {
				Expect(client.IgnoreAlreadyExists(testCtx.CreateObj(testCtx.Ctx, &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: ns},
				}))).Should(Succeed())
			}

			makeCleanupJob := func(key client.ObjectKey) {
				Expect(testCtx.CreateObj(testCtx.Ctx, &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: key.Namespace,
						Name:      key.Name,
						Labels:    map[string]string{cleanupLabel: "true"},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers:    []corev1.Container{{Name: deleteContainerName, Image: testdp.KBToolImage}},
								RestartPolicy: corev1.RestartPolicyNever,
							},
						},
					},
				})).Should(Succeed())
			}

			BeforeEach(func() {
				ensureNS(externalNS)
				ensureNS(wrongDefault)
				backup.Status.BackupRepoName = missingRepo
				backup.Status.Path = backupPath
				backup.Status.CleanupJobNamespace = externalNS
			})

			// Case 1: recorded namespace + delete Job Running → Deleting.
			It("returns Deleting when recorded namespace holds a Running delete Job", func() {
				key := BuildDeleteBackupFilesJobKey(backup, false)
				key.Namespace = externalNS
				makeCleanupJob(key)

				status, err := deleter.DeleteBackupFiles(backup)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusDeleting))
			})

			// Case 2: recorded namespace + delete Job Failed → Failed. The
			// finalizer-non-release contract is asserted at controller level;
			// here we assert the returned status which the controller's
			// switch maps onto a non-finalizer-removing branch.
			It("returns Failed when recorded namespace holds a Failed delete Job", func() {
				key := BuildDeleteBackupFilesJobKey(backup, false)
				key.Namespace = externalNS
				makeCleanupJob(key)
				testdp.ReplaceK8sJobStatus(&testCtx, key, batchv1.JobFailed)

				status, err := deleter.DeleteBackupFiles(backup)
				Expect(err).Should(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusFailed))
			})

			// Case 3: recorded namespace + delete Job Complete → Succeeded
			// and the external Job is best-effort cleaned up.
			It("returns Succeeded and cleans up when recorded namespace holds a Completed delete Job", func() {
				key := BuildDeleteBackupFilesJobKey(backup, false)
				key.Namespace = externalNS
				makeCleanupJob(key)
				testdp.ReplaceK8sJobStatus(&testCtx, key, batchv1.JobComplete)

				status, err := deleter.DeleteBackupFiles(backup)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusSucceeded))
				Eventually(testapps.CheckObjExists(&testCtx, key, &batchv1.Job{}, false)).Should(Succeed())
			})

			// Case 4: recorded namespace authoritative even when d.DeleteJobNamespace
			// is set to a wrong default. The lookup must find the Job in the
			// recorded namespace and must not fall back to the wrong default.
			It("uses recorded namespace as authoritative and does not fall back to a wrong default", func() {
				deleter.DeleteJobNamespace = wrongDefault
				key := BuildDeleteBackupFilesJobKey(backup, false)
				key.Namespace = externalNS
				makeCleanupJob(key)

				status, err := deleter.DeleteBackupFiles(backup)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusDeleting))
			})

			// Case 5: recorded namespace + preDelete Running, no delete Job →
			// Deleting. The BackupRepo-NotFound early return must not skip the
			// preDelete Job lookup.
			It("returns Deleting when recorded namespace holds a Running preDelete Job", func() {
				preKey := BuildDeleteBackupFilesJobKey(backup, true)
				preKey.Namespace = externalNS
				makeCleanupJob(preKey)

				status, err := deleter.DeleteBackupFiles(backup)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusDeleting))
			})

			// Case 6: empty recorded namespace + BackupRepo NotFound +
			// no Job in any candidate namespace → legacy Succeeded
			// semantic is preserved.
			It("preserves legacy Succeeded for backups without a recorded namespace", func() {
				backup.Status.CleanupJobNamespace = ""

				status, err := deleter.DeleteBackupFiles(backup)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusSucceeded))
			})

			// Case 7: recorded namespace + preDelete Complete + delete Job
			// absent → Unknown. A successful preDelete alone is not enough
			// to prove the backup artifact was deleted.
			It("returns Unknown when only the preDelete Job is Complete and the delete Job is absent", func() {
				preKey := BuildDeleteBackupFilesJobKey(backup, true)
				preKey.Namespace = externalNS
				makeCleanupJob(preKey)
				testdp.ReplaceK8sJobStatus(&testCtx, preKey, batchv1.JobComplete)

				status, err := deleter.DeleteBackupFiles(backup)
				Expect(err).Should(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusUnknown))
			})

			// Case 8: recorded namespace non-empty + no cleanup Job in any
			// candidate namespace → Unknown rather than Succeeded.
			It("returns Unknown when recorded namespace is set but no cleanup Job exists anywhere", func() {
				status, err := deleter.DeleteBackupFiles(backup)
				Expect(err).Should(HaveOccurred())
				Expect(status).Should(Equal(DeletionStatusUnknown))
			})
		})

		Context("RecordCleanupJobNamespaceFunc callback contract", func() {
			It("is a no-op when invoked with the already-recorded namespace", func() {
				called := 0
				deleter.RecordCleanupJobNamespaceFunc = func(b *dpv1alpha1.Backup, ns string) error {
					called++
					if b.Status.CleanupJobNamespace == ns {
						return nil
					}
					return errors.New("unexpected overwrite")
				}
				backup.Status.CleanupJobNamespace = deleteJobNamespace
				Expect(deleter.recordCleanupJobNamespace(backup, client.ObjectKey{
					Namespace: deleteJobNamespace, Name: "any-cleanup-job",
				})).Should(Succeed())
				Expect(called).Should(Equal(1))
			})

			It("propagates callback error when namespace already differs", func() {
				deleter.RecordCleanupJobNamespaceFunc = func(b *dpv1alpha1.Backup, ns string) error {
					if b.Status.CleanupJobNamespace == "" || b.Status.CleanupJobNamespace == ns {
						return nil
					}
					return errors.New("namespace already recorded as " + b.Status.CleanupJobNamespace)
				}
				backup.Status.CleanupJobNamespace = "previously-recorded-ns"
				err := deleter.recordCleanupJobNamespace(backup, client.ObjectKey{
					Namespace: deleteJobNamespace, Name: "any-cleanup-job",
				})
				Expect(err).Should(MatchError(ContainSubstring("previously-recorded-ns")))
			})

			It("refuses to create an external cleanup Job when no callback is wired", func() {
				deleter.RecordCleanupJobNamespaceFunc = nil
				err := deleter.recordCleanupJobNamespace(backup, client.ObjectKey{
					Namespace: deleteJobNamespace, Name: "any-cleanup-job",
				})
				Expect(err).Should(MatchError(ContainSubstring("RecordCleanupJobNamespaceFunc is not set")))
			})

			It("is a no-op for in-namespace cleanup Jobs", func() {
				called := false
				deleter.RecordCleanupJobNamespaceFunc = func(_ *dpv1alpha1.Backup, _ string) error {
					called = true
					return nil
				}
				Expect(deleter.recordCleanupJobNamespace(backup, client.ObjectKey{
					Namespace: backup.Namespace, Name: "in-namespace-cleanup-job",
				})).Should(Succeed())
				Expect(called).Should(BeFalse())
			})
		})

		It("should reject an external delete job for non-tool BackupRepo", func() {
			mountRepo := &dpv1alpha1.BackupRepo{
				ObjectMeta: metav1.ObjectMeta{Name: "mount-repo"},
				Spec: dpv1alpha1.BackupRepoSpec{
					AccessMethod: dpv1alpha1.AccessMethodMount,
				},
			}

			err := deleter.createDeleteJob(corev1.Container{Name: deleteContainerName},
				client.ObjectKey{Namespace: deleteJobNamespace, Name: "delete-mount-backup"},
				backup, mountRepo, backupRepoPVCName)
			Expect(err).Should(MatchError(ContainSubstring("requires a tool-access BackupRepo")))
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
