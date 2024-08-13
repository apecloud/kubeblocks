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

package dataprotection

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dpbackup "github.com/apecloud/kubeblocks/pkg/dataprotection/backup"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Backup Controller test", func() {
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

		// wait all backups to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
		testapps.ClearResources(&testCtx, generics.VolumeSnapshotClassSignature, ml)
	}

	var clusterInfo *testdp.BackupClusterInfo

	BeforeEach(func() {
		cleanEnv()
		clusterInfo = testdp.NewFakeCluster(&testCtx)
	})

	AfterEach(func() {
		cleanEnv()
	})

	When("with default settings", func() {
		var (
			backupPolicy *dpv1alpha1.BackupPolicy
			repoPVCName  string
			cluster      *appsv1alpha1.Cluster
			pvcName      string
			targetPod    *corev1.Pod
		)

		BeforeEach(func() {
			By("creating an actionSet")
			actionSet := testdp.NewFakeActionSet(&testCtx)

			By("creating storage provider")
			_ = testdp.NewFakeStorageProvider(&testCtx, nil)

			By("creating backup repo")
			_, repoPVCName = testdp.NewFakeBackupRepo(&testCtx, nil)

			By("creating a backupPolicy from actionSet: " + actionSet.Name)
			backupPolicy = testdp.NewFakeBackupPolicy(&testCtx, nil)

			cluster = clusterInfo.Cluster
			pvcName = clusterInfo.TargetPVC
			targetPod = clusterInfo.TargetPod
		})

		Context("creates a backup", func() {
			var (
				backupKey types.NamespacedName
				backup    *dpv1alpha1.Backup
			)

			getJobKey := func() client.ObjectKey {
				return client.ObjectKey{
					Name:      dpbackup.GenerateBackupJobName(backup, dpbackup.BackupDataJobNamePrefix+"-0"),
					Namespace: backup.Namespace,
				}
			}

			BeforeEach(func() {
				By("creating a backup from backupPolicy " + testdp.BackupPolicyName) //nolint:goconst
				backup = testdp.NewFakeBackup(&testCtx, nil)
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should succeed after job completes", func() {
				By("check backup status")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.PersistentVolumeClaimName).Should(Equal(repoPVCName))
					g.Expect(fetched.Status.Path).Should(Equal(dpbackup.BuildBaseBackupPath(fetched, "", backupPolicy.Spec.PathPrefix)))
					g.Expect(fetched.Status.Phase).Should(Equal(dpv1alpha1.BackupPhaseRunning))
					g.Expect(fetched.Annotations[constant.EncryptedSystemAccountsAnnotationKey]).ShouldNot(BeEmpty())
				})).Should(Succeed())

				By("check backup job's nodeName equals pod's nodeName")
				Eventually(testapps.CheckObj(&testCtx, getJobKey(), func(g Gomega, fetched *batchv1.Job) {
					g.Expect(fetched.Spec.Template.Spec.NodeSelector[corev1.LabelHostname]).To(Equal(targetPod.Spec.NodeName))
					// image should be expanded by env
					g.Expect(fetched.Spec.Template.Spec.Containers[0].Image).Should(ContainSubstring(testdp.ImageTag))
					g.Expect(fetched.Spec.Template.Spec.ServiceAccountName).Should(Equal(viper.GetString(dptypes.CfgKeyWorkerServiceAccountName)))
				})).Should(Succeed())

				testdp.PatchK8sJobStatus(&testCtx, getJobKey(), batchv1.JobComplete)

				By("backup job should have completed")
				Eventually(testapps.CheckObj(&testCtx, getJobKey(), func(g Gomega, fetched *batchv1.Job) {
					_, finishedType, _ := dputils.IsJobFinished(fetched)
					g.Expect(fetched.Labels[constant.AppManagedByLabelKey]).Should(Equal(dptypes.AppName))
					g.Expect(finishedType).To(Equal(batchv1.JobComplete))
				})).Should(Succeed())

				By("backup should have completed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
					g.Expect(fetched.Labels[dptypes.ClusterUIDLabelKey]).Should(Equal(string(cluster.UID)))
					g.Expect(fetched.Labels[constant.AppInstanceLabelKey]).Should(Equal(testdp.ClusterName))
					g.Expect(fetched.Labels[constant.KBAppComponentLabelKey]).Should(Equal(testdp.ComponentName))
					g.Expect(fetched.Labels[constant.AppManagedByLabelKey]).Should(Equal(dptypes.AppName))
					g.Expect(fetched.Annotations[constant.ClusterSnapshotAnnotationKey]).ShouldNot(BeEmpty())
				})).Should(Succeed())

				By("backup job should be deleted after backup completed")
				Eventually(testapps.CheckObjExists(&testCtx, getJobKey(), &batchv1.Job{}, false)).Should(Succeed())
			})

			It("should fail after job fails", func() {
				testdp.PatchK8sJobStatus(&testCtx, getJobKey(), batchv1.JobFailed)

				By("check backup job failed")
				Eventually(testapps.CheckObj(&testCtx, getJobKey(), func(g Gomega, fetched *batchv1.Job) {
					_, finishedType, _ := dputils.IsJobFinished(fetched)
					g.Expect(finishedType).To(Equal(batchv1.JobFailed))
				})).Should(Succeed())

				By("check backup failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})
		})

		Context("create an invalid backup", func() {
			It("should fail if backupPolicy is not found", func() {
				By("creating a backup using a not found backupPolicy")
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.BackupPolicyName = "not-found"
				})
				backupKey := client.ObjectKeyFromObject(backup)

				By("check backup failed and its expiration when retentionPeriod is not set")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
					g.Expect(fetched.Status.Expiration).Should(BeNil())
				})).Should(Succeed())
			})
		})

		Context("creates a backup with retentionPeriod", func() {
			It("create a valid backup", func() {
				By("creating a backup from backupPolicy " + testdp.BackupPolicyName)
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.RetentionPeriod = "1h"
				})
				backupKey := client.ObjectKeyFromObject(backup)

				getJobKey := func() client.ObjectKey {
					return client.ObjectKey{
						Name:      dpbackup.GenerateBackupJobName(backup, dpbackup.BackupDataJobNamePrefix+"-0"),
						Namespace: backup.Namespace,
					}
				}

				By("check backup expiration is set by start time when backup is running")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).Should(Equal(dpv1alpha1.BackupPhaseRunning))
					g.Expect(fetched.Status.Expiration.Second()).Should(Equal(fetched.Status.StartTimestamp.Add(time.Hour).Second()))
				})).Should(Succeed())

				testdp.PatchK8sJobStatus(&testCtx, getJobKey(), batchv1.JobComplete)

				By("check backup expiration is updated by completion time when backup is completed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
					g.Expect(fetched.Status.CompletionTimestamp).ShouldNot(BeNil())
					g.Expect(fetched.Status.Expiration.Second()).Should(Equal(fetched.Status.CompletionTimestamp.Add(time.Hour).Second()))
				})).Should(Succeed())
			})

			It("create an invalid backup", func() {
				By("creating a backup using a not found backupPolicy")
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.BackupPolicyName = "not-found"
					backup.Spec.RetentionPeriod = "1h"
				})
				backupKey := client.ObjectKeyFromObject(backup)

				By("check backup failed and its expiration is set")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
					g.Expect(fetched.Status.Expiration).ShouldNot(BeNil())
				})).Should(Succeed())
			})

			It("create a backup with backupMethod and target", func() {
				By("Set backupMethod's target")
				Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(bp *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.BackupMethods[0].Target = &dpv1alpha1.BackupTarget{
						PodSelector: &dpv1alpha1.PodSelector{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									constant.AppInstanceLabelKey:    testdp.ClusterName,
									constant.KBAppComponentLabelKey: testdp.ComponentName,
									constant.RoleLabelKey:           constant.Follower,
								},
							},
						},
					}
				})).Should(Succeed())
				By("check targets pod")
				reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
				targets, err := GetTargetPods(reqCtx, k8sClient, nil, backupPolicy, backupPolicy.Spec.BackupMethods[0].Target, dpv1alpha1.BackupTypeFull)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(targets).Should(HaveLen(1))
				Expect(targets[0].Name).Should(Equal(testdp.ClusterName + "-" + testdp.ComponentName + "-1"))
			})

			It("create a backup with backupMethod and podSelection strategy is All", func() {
				By("Set backupMethod's target and podSelection strategy to All")
				Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(bp *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.BackupMethods[0].Target = &dpv1alpha1.BackupTarget{
						PodSelector: &dpv1alpha1.PodSelector{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									constant.AppInstanceLabelKey:    testdp.ClusterName,
									constant.KBAppComponentLabelKey: testdp.ComponentName,
								},
							},
							Strategy: dpv1alpha1.PodSelectionStrategyAll,
						},
					}
				})).Should(Succeed())
				By("check targets pod")
				reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
				targets, err := GetTargetPods(reqCtx, k8sClient, nil, backupPolicy, backupPolicy.Spec.BackupMethods[0].Target, dpv1alpha1.BackupTypeFull)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(targets).Should(HaveLen(2))
				By("create a backup")
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.RetentionPeriod = "1h"
				})
				getJobKey := func(index int) client.ObjectKey {
					return client.ObjectKey{
						Name:      dpbackup.GenerateBackupJobName(backup, fmt.Sprintf("%s-%d", dpbackup.BackupDataJobNamePrefix, index)),
						Namespace: backup.Namespace,
					}
				}
				By("mock jobs are completed and backup should be completed")
				testdp.PatchK8sJobStatus(&testCtx, getJobKey(0), batchv1.JobComplete)
				testdp.PatchK8sJobStatus(&testCtx, getJobKey(1), batchv1.JobComplete)
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
					g.Expect(fetched.Status.CompletionTimestamp).ShouldNot(BeNil())
					g.Expect(fetched.Status.Expiration.Second()).Should(Equal(fetched.Status.CompletionTimestamp.Add(time.Hour).Second()))
				})).Should(Succeed())
			})
		})

		It("create a backup with backupMethod and multi targets", func() {
			By("Set backupMethod's targets")
			Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(bp *dpv1alpha1.BackupPolicy) {
				podSelector := &dpv1alpha1.PodSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constant.AppInstanceLabelKey:    testdp.ClusterName,
							constant.KBAppComponentLabelKey: testdp.ComponentName,
						},
					},
					Strategy: dpv1alpha1.PodSelectionStrategyAny,
				}
				backupPolicy.Spec.BackupMethods[0].Targets = []dpv1alpha1.BackupTarget{
					{Name: testdp.ComponentName + "-0", PodSelector: podSelector},
					{Name: testdp.ComponentName + "-1", PodSelector: podSelector},
				}
			})).Should(Succeed())
			By("check targets pod")
			targets := backupPolicy.Spec.BackupMethods[0].Targets
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			targetPods, err := GetTargetPods(reqCtx, k8sClient, nil, backupPolicy, &targets[0], dpv1alpha1.BackupTypeFull)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(targetPods).Should(HaveLen(1))
			By("create a backup")
			backup := testdp.NewFakeBackup(&testCtx, nil)
			getJobKey := func(targetName string) client.ObjectKey {
				return client.ObjectKey{
					Name:      dpbackup.GenerateBackupJobName(backup, fmt.Sprintf("%s-%s-0", dpbackup.BackupDataJobNamePrefix, targetName)),
					Namespace: backup.Namespace,
				}
			}
			By("mock backup jobs to completed and backup should be completed")
			testdp.PatchK8sJobStatus(&testCtx, getJobKey(targets[0].Name), batchv1.JobComplete)
			testdp.PatchK8sJobStatus(&testCtx, getJobKey(targets[1].Name), batchv1.JobComplete)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, fetched *dpv1alpha1.Backup) {
				g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
			})).Should(Succeed())
		})

		It("create an backup using fallbackLabelSelector", func() {
			podFactory := func(name string) *testapps.MockPodFactory {
				return testapps.NewPodFactory(testCtx.DefaultNamespace, name).
					AddAppInstanceLabel(testdp.ClusterName).
					AddAppComponentLabel(testdp.ComponentName).
					AddContainer(corev1.Container{Name: testdp.ContainerName, Image: testapps.ApeCloudMySQLImage})
			}
			podName := "fallback" + testdp.ClusterName + "-" + testdp.ComponentName
			By("mock a primary pod that is available ")
			pod0 := podFactory(podName + "-0").
				AddRoleLabel("primary").
				Create(&testCtx).GetObject()
			Expect(testapps.ChangeObjStatus(&testCtx, pod0, func() {
				pod0.Status.Phase = corev1.PodRunning
				testk8s.MockPodAvailable(pod0, metav1.Now())
			})).Should(Succeed())
			By("mock a secondary pod that is unavailable")
			pod1 := podFactory(podName + "-1").
				AddRoleLabel("secondary").
				Create(&testCtx).GetObject()
			Expect(testapps.ChangeObjStatus(&testCtx, pod1, func() {
				pod1.Status.Phase = corev1.PodFailed
				testk8s.MockPodIsFailed(context.Background(), testCtx, pod1)
			})).Should(Succeed())

			By("Set backupPolicy's target with fallbackLabelSelector")
			Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(bp *dpv1alpha1.BackupPolicy) {
				podSelector := &dpv1alpha1.PodSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constant.AppInstanceLabelKey:    testdp.ClusterName,
							constant.KBAppComponentLabelKey: testdp.ComponentName,
							constant.RoleLabelKey:           "secondary",
						},
					},
					FallbackLabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constant.AppInstanceLabelKey:    testdp.ClusterName,
							constant.KBAppComponentLabelKey: testdp.ComponentName,
							constant.RoleLabelKey:           "primary",
						},
					},
					Strategy: dpv1alpha1.PodSelectionStrategyAny,
				}
				backupPolicy.Spec.Target = &dpv1alpha1.BackupTarget{
					Name: testdp.ComponentName + "-0", PodSelector: podSelector,
				}
			})).Should(Succeed())

			By("check targets pod")
			target := backupPolicy.Spec.Target
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			targetPods, err := GetTargetPods(reqCtx, k8sClient, nil, backupPolicy, target, dpv1alpha1.BackupTypeFull)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(targetPods).Should(HaveLen(1))
			Expect(targetPods[0].Name).Should(Equal(pod0.Name))

			By("create a backup")
			backup := testdp.NewFakeBackup(&testCtx, nil)
			getJobKey := func(targetName string) client.ObjectKey {
				return client.ObjectKey{
					Name:      dpbackup.GenerateBackupJobName(backup, fmt.Sprintf("%s-%s-0", dpbackup.BackupDataJobNamePrefix, targetName)),
					Namespace: backup.Namespace,
				}
			}
			By("mock backup jobs to completed and backup should be completed")
			testdp.PatchK8sJobStatus(&testCtx, getJobKey(target.Name), batchv1.JobComplete)
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, fetched *dpv1alpha1.Backup) {
				g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
			})).Should(Succeed())
		})

		It("create a backup with backupMethod and specify the port by name", func() {
			By("Set backupMethod's targets with containerPort")
			Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(bp *dpv1alpha1.BackupPolicy) {
				podSelector := &dpv1alpha1.PodSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constant.AppInstanceLabelKey:    testdp.ClusterName,
							constant.KBAppComponentLabelKey: testdp.ComponentName,
						},
					},
					Strategy: dpv1alpha1.PodSelectionStrategyAny,
				}
				containerPort := &dpv1alpha1.ContainerPort{
					ContainerName: testdp.ContainerName + "-1",
					PortName:      testdp.PortName,
				}
				backupPolicy.Spec.BackupMethods[0].Target = &dpv1alpha1.BackupTarget{
					Name: testdp.ComponentName, PodSelector: podSelector, ContainerPort: containerPort,
				}
				backupPolicy.Spec.Target.ConnectionCredential = nil
			})).Should(Succeed())

			By("create a backup")
			backup := testdp.NewFakeBackup(&testCtx, nil)
			getJobKey := func(targetName string) client.ObjectKey {
				return client.ObjectKey{
					Name:      dpbackup.GenerateBackupJobName(backup, fmt.Sprintf("%s-%s-0", dpbackup.BackupDataJobNamePrefix, targetName)),
					Namespace: backup.Namespace,
				}
			}

			getDPDBPortEnv := func(container *corev1.Container) corev1.EnvVar {
				for _, env := range container.Env {
					if env.Name == dptypes.DPDBPort {
						return env
					}
				}
				return corev1.EnvVar{}
			}

			By("check backup job's port env")
			Eventually(testapps.CheckObj(&testCtx, getJobKey(backupPolicy.Spec.BackupMethods[0].Target.Name), func(g Gomega, fetched *batchv1.Job) {
				// image should be expanded by env
				g.Expect(getDPDBPortEnv(&fetched.Spec.Template.Spec.Containers[0]).Value).Should(Equal(strconv.Itoa(testdp.PortNum)))
			})).Should(Succeed())
		})
		Context("creates a backup with encryption", func() {
			const (
				encryptionKeySecretName = "backup-encryption"
				keyName                 = "password"
			)
			It("should fail if encryption key secret is not present", func() {
				By("set encryptionConfig")
				Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(bp *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.EncryptionConfig = &dpv1alpha1.EncryptionConfig{
						Algorithm: "AES-256-CFB",
						PassPhraseSecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: encryptionKeySecretName,
							},
							Key: keyName,
						},
					}
				})).Should(Succeed())

				By("create a backup")
				backup := testdp.NewFakeBackup(&testCtx, nil)

				By("check the backup, and it should be failed")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})

			It("should run the backup with encryption envs", func() {
				By("set encryptionConfig")
				Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(bp *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.EncryptionConfig = &dpv1alpha1.EncryptionConfig{
						Algorithm: "AES-256-CFB",
						PassPhraseSecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: encryptionKeySecretName,
							},
							Key: keyName,
						},
					}
				})).Should(Succeed())

				By("create the encryption key secret")
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      encryptionKeySecretName,
						Namespace: testCtx.DefaultNamespace,
					},
					StringData: map[string]string{
						keyName: "whatever",
					},
				}
				testapps.CreateK8sResource(&testCtx, secret)

				By("create a backup")
				backup := testdp.NewFakeBackup(&testCtx, nil)

				By("check the backup")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseRunning))
					g.Expect(fetched.Status.EncryptionConfig).ShouldNot(BeNil())
				})).Should(Succeed())

				By("check the backup job")
				getJobKey := func(index int) client.ObjectKey {
					return client.ObjectKey{
						Name:      dpbackup.GenerateBackupJobName(backup, fmt.Sprintf("%s-%d", dpbackup.BackupDataJobNamePrefix, index)),
						Namespace: backup.Namespace,
					}
				}
				Eventually(testapps.CheckObj(&testCtx, getJobKey(0), func(g Gomega, job *batchv1.Job) {
					g.Expect(len(job.Spec.Template.Spec.Containers)).ShouldNot(BeZero())
					expectedEnvs := []string{
						dptypes.DPDatasafedEncryptionAlgorithm,
						dptypes.DPDatasafedEncryptionPassPhrase,
					}
					for _, c := range job.Spec.Template.Spec.Containers {
						count := 0
						for _, env := range c.Env {
							if slices.Contains(expectedEnvs, env.Name) {
								count++
							}
						}
						g.Expect(count).To(BeEquivalentTo(len(expectedEnvs)))
					}
				})).Should(Succeed())
			})
		})

		Context("deletes a backup", func() {
			var (
				backupKey types.NamespacedName
				backup    *dpv1alpha1.Backup
			)
			BeforeEach(func() {
				By("creating a backup from backupPolicy " + testdp.BackupPolicyName)
				backup = testdp.NewFakeBackup(&testCtx, nil)
				backupKey = client.ObjectKeyFromObject(backup)

				By("waiting for backup status to be running")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseRunning))
				})).Should(Succeed())
			})

			It("should create a Job for deleting backup files", func() {
				By("deleting a backup object")
				testapps.DeleteObject(&testCtx, backupKey, &dpv1alpha1.Backup{})

				By("checking new created Job")
				jobKey := dpbackup.BuildDeleteBackupFilesJobKey(backup, false)
				job := &batchv1.Job{}
				Eventually(testapps.CheckObjExists(&testCtx, jobKey, job, true)).Should(Succeed())
				volumeName := "dp-backup-data"
				Eventually(testapps.CheckObj(&testCtx, jobKey, func(g Gomega, job *batchv1.Job) {
					Expect(job.Labels[constant.AppManagedByLabelKey]).Should(Equal(dptypes.AppName))
					Expect(job.Spec.Template.Spec.Volumes).
						Should(ContainElement(corev1.Volume{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: repoPVCName,
								},
							},
						}))
					Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).
						Should(ContainElement(corev1.VolumeMount{
							Name:      volumeName,
							MountPath: dpbackup.RepoVolumeMountPath,
						}))
					Expect(job.Spec.Template.Spec.ServiceAccountName).Should(Equal(viper.GetString(dptypes.CfgKeyWorkerServiceAccountName)))
				})).Should(Succeed())

				By("checking backup object, it should not be deleted")
				Eventually(testapps.CheckObjExists(&testCtx, backupKey,
					&dpv1alpha1.Backup{}, true)).Should(Succeed())

				By("mock job for deletion to failed, backup should not be deleted")
				testdp.ReplaceK8sJobStatus(&testCtx, jobKey, batchv1.JobFailed)
				Eventually(testapps.CheckObjExists(&testCtx, backupKey,
					&dpv1alpha1.Backup{}, true)).Should(Succeed())

				By("mock job for deletion to completed, backup should be deleted")
				testdp.ReplaceK8sJobStatus(&testCtx, jobKey, batchv1.JobComplete)

				By("check deletion backup file job completed")
				Eventually(testapps.CheckObj(&testCtx, jobKey, func(g Gomega, fetched *batchv1.Job) {
					_, finishedType, _ := dputils.IsJobFinished(fetched)
					g.Expect(finishedType).To(Equal(batchv1.JobComplete))
				})).Should(Succeed())

				By("check backup deleted")
				Eventually(testapps.CheckObjExists(&testCtx, backupKey,
					&dpv1alpha1.Backup{}, false)).Should(Succeed())

				// TODO: add delete backup test case with the pvc not exists
			})
		})

		Context("creates a snapshot backup", func() {
			var (
				backupKey types.NamespacedName
				backup    *dpv1alpha1.Backup
				vsKey     client.ObjectKey
			)

			BeforeEach(func() {
				// mock VolumeSnapshotClass for volume snapshot
				testk8s.CreateVolumeSnapshotClass(&testCtx, testutil.DefaultCSIDriver)

				By("create a backup from backupPolicy " + testdp.BackupPolicyName)
				backup = testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.BackupMethod = testdp.VSBackupMethodName
				})
				backupKey = client.ObjectKeyFromObject(backup)
				vsKey = client.ObjectKey{
					Name:      dputils.GetBackupVolumeSnapshotName(backup.Name, "data", 0),
					Namespace: backup.Namespace,
				}
			})

			It("should success after all volume snapshot ready", func() {
				By("patching volumesnapshot status to ready")
				testdp.PatchVolumeSnapshotStatus(&testCtx, vsKey, true)

				By("checking volume snapshot source is equal to pvc")
				Eventually(testapps.CheckObj(&testCtx, vsKey, func(g Gomega, fetched *vsv1.VolumeSnapshot) {
					g.Expect(*fetched.Spec.Source.PersistentVolumeClaimName).To(Equal(pvcName))
				})).Should(Succeed())
			})

			It("should fail if volumesnapshot reports error", func() {
				By("patching volumesnapshot status with error")
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, vsKey, func(tmpVS *vsv1.VolumeSnapshot) {
					msg := "Failed to set default snapshot class with error: some error"
					vsError := vsv1.VolumeSnapshotError{
						Message: &msg,
					}
					snapStatus := vsv1.VolumeSnapshotStatus{Error: &vsError}
					tmpVS.Status = &snapStatus
				})).Should(Succeed())

				By("checking backup failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})
		})

		Context("creates a snapshot backup on error", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				By("By remove persistent pvc")
				// delete rest mocked objects
				inNS := client.InNamespace(testCtx.DefaultNamespace)
				ml := client.HasLabels{testCtx.TestObjLabelKey}
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx,
					generics.PersistentVolumeClaimSignature, true, inNS, ml)
			})

			It("should fail when disable volumesnapshot", func() {
				By("creating a backup from backupPolicy " + testdp.BackupPolicyName)
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.BackupMethod = testdp.VSBackupMethodName
				})
				backupKey = client.ObjectKeyFromObject(backup)

				By("check backup failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})

			It("should fail without pvc", func() {
				By("creating a backup from backupPolicy " + testdp.BackupPolicyName)
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.BackupMethod = testdp.VSBackupMethodName
				})
				backupKey = client.ObjectKeyFromObject(backup)

				By("check backup failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})
		})
	})

	When("with exceptional settings", func() {
		var (
			backupPolicy *dpv1alpha1.BackupPolicy
		)

		Context("creates a backup with non-existent backup policy", func() {
			var backupKey types.NamespacedName
			BeforeEach(func() {
				By("creating a backup from backupPolicy " + testdp.BackupPolicyName)
				backup := testdp.NewFakeBackup(&testCtx, nil)
				backupKey = client.ObjectKeyFromObject(backup)
			})
			It("should fail", func() {
				By("check backup status failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})
		})

		Context("creates a backup using non-existent backup method", func() {
			BeforeEach(func() {
				By("creating a backupPolicy without backup method")
				backupPolicy = testdp.NewFakeBackupPolicy(&testCtx, nil)
			})

			It("should fail because of no-existent backup method", func() {
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.BackupPolicyName = backupPolicy.Name
					backup.Spec.BackupMethod = "non-existent"
				})
				backupKey := client.ObjectKeyFromObject(backup)

				By("check backup status failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})
		})

		Context("creates a backup with invalid backup method", func() {
			BeforeEach(func() {
				backupPolicy = testdp.NewFakeBackupPolicy(&testCtx, func(backupPolicy *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.BackupMethods = append(backupPolicy.Spec.BackupMethods, dpv1alpha1.BackupMethod{
						Name:          "invalid",
						ActionSetName: "",
					})
				})
			})

			It("should fail because backup method doesn't specify snapshotVolumes with empty actionSet", func() {
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Spec.BackupPolicyName = backupPolicy.Name
					backup.Spec.BackupMethod = "invalid"
				})
				backupKey := client.ObjectKeyFromObject(backup)

				By("check backup status failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})

			It("should fail because of no-existing actionSet", func() {
				backup := testdp.NewFakeBackup(&testCtx, nil)
				backupKey := client.ObjectKeyFromObject(backup)

				By("check backup status failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})

			It("should fail because actionSet's backup type isn't Full", func() {
				actionSet := testdp.NewFakeActionSet(&testCtx)
				actionSetKey := client.ObjectKeyFromObject(actionSet)
				Eventually(testapps.GetAndChangeObj(&testCtx, actionSetKey, func(fetched *dpv1alpha1.ActionSet) {
					fetched.Spec.BackupType = dpv1alpha1.BackupTypeIncremental
				}))

				backup := testdp.NewFakeBackup(&testCtx, nil)
				backupKey := client.ObjectKeyFromObject(backup)

				By("check backup status failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})
		})

		Context("create continuous backup", func() {
			It("should fail when continuous backup don't have backupschedule label", func() {
				By("create actionset with continuous backuptype")
				actionSet := testdp.NewFakeActionSet(&testCtx)
				actionSetKey := client.ObjectKeyFromObject(actionSet)
				Eventually(testapps.GetAndChangeObj(&testCtx, actionSetKey, func(fetched *dpv1alpha1.ActionSet) {
					fetched.Spec.BackupType = dpv1alpha1.BackupTypeContinuous
				})).Should(Succeed())
				By("create continuous backup without backupschedule label")
				backupPolicy = testdp.NewFakeBackupPolicy(&testCtx, nil)
				backup := testdp.NewFakeBackup(&testCtx, func(bp *dpv1alpha1.Backup) {
					bp.ObjectMeta.Name = "continuousbackup"
					bp.Labels = map[string]string{}
				})
				backupKey := client.ObjectKeyFromObject(backup)
				By("check backup phase")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).Should(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})

			It("continue reconcile when continuous backup is Failed after fixing the issue", func() {
				By("create actionset and backupRepo for continuous backup")
				actionSet := testdp.NewFakeActionSet(&testCtx)
				Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(actionSet), func(fetched *dpv1alpha1.ActionSet) {
					fetched.Spec.BackupType = dpv1alpha1.BackupTypeContinuous
				})).Should(Succeed())
				_ = testdp.NewFakeStorageProvider(&testCtx, nil)
				backupRepo, repoPVCName := testdp.NewFakeBackupRepo(&testCtx, nil)

				By("create backupPolicy with non-exist backupRepo")
				backupPolicy = testdp.NewFakeBackupPolicy(&testCtx, func(bp *dpv1alpha1.BackupPolicy) {
					bp.Spec.BackupRepoName = pointer.String("non-exist-repo")
				})

				By("create a backupSchedule and enable continuous backup")
				backupSchedule := testdp.NewFakeBackupSchedule(&testCtx, func(schedule *dpv1alpha1.BackupSchedule) {
					schedule.Spec.Schedules[0].Enabled = pointer.Bool(true)
				})

				By("expect backup phase to Failed")
				backupName := dpbackup.GenerateCRNameByBackupSchedule(backupSchedule, testdp.BackupMethodName)
				backupKey := client.ObjectKey{Name: backupName, Namespace: testCtx.DefaultNamespace}
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).Should(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())

				By("use the correct backupRepo")
				Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(policy *dpv1alpha1.BackupPolicy) {
					policy.Spec.BackupRepoName = pointer.String(backupRepo.Name)
				})).Should(Succeed())

				By("expect backup phase to Running")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).Should(Equal(dpv1alpha1.BackupPhaseRunning))
					g.Expect(fetched.Status.PersistentVolumeClaimName).Should(Equal(repoPVCName))
				})).Should(Succeed())
			})
		})
	})

	When("with backup repo", func() {
		var (
			repoPVCName string
			sp          *dpv1alpha1.StorageProvider
			repo        *dpv1alpha1.BackupRepo
		)

		BeforeEach(func() {
			By("creating backup repo")
			sp = testdp.NewFakeStorageProvider(&testCtx, nil)
			repo, repoPVCName = testdp.NewFakeBackupRepo(&testCtx, nil)

			By("creating actionSet")
			_ = testdp.NewFakeActionSet(&testCtx)
		})

		Context("explicitly specify backup repo", func() {
			It("should use the backup repo specified in the policy", func() {
				By("creating backup policy and backup")
				_ = testdp.NewFakeBackupPolicy(&testCtx, nil)
				backup := testdp.NewFakeBackup(&testCtx, nil)
				By("checking backup, it should use the PVC from the backup repo")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
					g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName))
				})).Should(Succeed())
			})

			It("should use the backup repo specified in the backup object", func() {
				By("creating a second backup repo")
				repo2, repoPVCName2 := testdp.NewFakeBackupRepo(&testCtx, func(repo *dpv1alpha1.BackupRepo) {
					repo.Name += "2"
				})
				By("creating backup policy and backup")
				_ = testdp.NewFakeBackupPolicy(&testCtx, func(backupPolicy *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.BackupRepoName = &repo.Name
				})
				backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					if backup.Labels == nil {
						backup.Labels = map[string]string{}
					}
					backup.Labels[dataProtectionBackupRepoKey] = repo2.Name
				})
				By("checking backup, it should use the PVC from repo2")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
					g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName2))
				})).Should(Succeed())
			})
		})

		Context("default backup repo", func() {
			It("should use the default backup repo if it's not specified", func() {
				By("creating backup policy and backup")
				_ = testdp.NewFakeBackupPolicy(&testCtx, func(backupPolicy *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.BackupRepoName = nil
				})
				backup := testdp.NewFakeBackup(&testCtx, nil)
				By("checking backup, it should use the PVC from the backup repo")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
					g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName))
				})).Should(Succeed())
			})

			It("should associate the default backup repo with the backup object", func() {
				By("creating backup policy and backup")
				_ = testdp.NewFakeBackupPolicy(&testCtx, func(backupPolicy *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.BackupRepoName = nil
				})
				backup := testdp.NewFakeBackup(&testCtx, nil)
				By("checking backup labels")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
					g.Expect(backup.Labels[dataProtectionBackupRepoKey]).Should(BeEquivalentTo(repo.Name))
				})).Should(Succeed())

				By("creating backup2")
				backup2 := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
					backup.Name += "2"
				})
				By("checking backup2 labels")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup2), func(g Gomega, backup *dpv1alpha1.Backup) {
					g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName))
					g.Expect(backup.Labels[dataProtectionBackupRepoKey]).Should(BeEquivalentTo(repo.Name))
				})).Should(Succeed())
			})

			Context("multiple default backup repos", func() {
				var repoPVCName2 string
				BeforeEach(func() {
					By("creating a second backup repo")
					sp2 := testdp.NewFakeStorageProvider(&testCtx, func(sp *dpv1alpha1.StorageProvider) {
						sp.Name += "2"
					})
					_, repoPVCName2 = testdp.NewFakeBackupRepo(&testCtx, func(repo *dpv1alpha1.BackupRepo) {
						repo.Name += "2"
						repo.Spec.StorageProviderRef = sp2.Name
					})
					By("creating backup policy")
					_ = testdp.NewFakeBackupPolicy(&testCtx, func(backupPolicy *dpv1alpha1.BackupPolicy) {
						// set backupRepoName in backupPolicy to nil to make it use the default backup repo
						backupPolicy.Spec.BackupRepoName = nil
					})
				})

				It("should fail if there are multiple default backup repos", func() {
					By("creating backup")
					backup := testdp.NewFakeBackup(&testCtx, nil)
					By("checking backup, it should fail because there are multiple default backup repos")
					Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
						g.Expect(backup.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.BackupPhaseFailed))
						g.Expect(backup.Status.FailureReason).Should(ContainSubstring("multiple default BackupRepo found"))
					})).Should(Succeed())
				})

				It("should only repos in ready status can be selected as the default backup repo", func() {
					By("making repo to failed status")
					Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(sp),
						func(fetched *dpv1alpha1.StorageProvider) {
							fetched.Status.Phase = dpv1alpha1.StorageProviderNotReady
							fetched.Status.Conditions = nil
						})).ShouldNot(HaveOccurred())
					Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(repo),
						func(g Gomega, repo *dpv1alpha1.BackupRepo) {
							g.Expect(repo.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.BackupRepoFailed))
						})).Should(Succeed())
					By("creating backup")
					backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
						backup.Name = "second-backup"
					})
					By("checking backup, it should use the PVC from repo2")
					Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
						g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName2))
					})).Should(Succeed())
				})
			})
		})

		Context("no backup repo available", func() {
			It("should throw error", func() {
				By("making the backup repo as non-default")
				Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(repo), func(repo *dpv1alpha1.BackupRepo) {
					delete(repo.Annotations, dptypes.DefaultBackupRepoAnnotationKey)
				})).Should(Succeed())
				By("creating backup")
				_ = testdp.NewFakeBackupPolicy(&testCtx, func(backupPolicy *dpv1alpha1.BackupPolicy) {
					backupPolicy.Spec.BackupRepoName = nil
				})
				backup := testdp.NewFakeBackup(&testCtx, nil)
				By("checking backup, it should fail because the backup repo are not available")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
					g.Expect(backup.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.BackupPhaseFailed))
					g.Expect(backup.Status.FailureReason).Should(ContainSubstring("no default BackupRepo found"))
				})).Should(Succeed())
			})
		})
	})

	When("use kopia", func() {
		var (
			backupPolicy *dpv1alpha1.BackupPolicy
			repoPVCName  string
			cluster      *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			By("creating an actionSet")
			actionSet := testdp.NewFakeActionSet(&testCtx)

			By("creating storage provider")
			_ = testdp.NewFakeStorageProvider(&testCtx, nil)

			By("creating backup repo")
			_, repoPVCName = testdp.NewFakeBackupRepo(&testCtx, nil)

			By("creating a backupPolicy from actionSet: " + actionSet.Name)
			backupPolicy = testdp.NewFakeBackupPolicy(&testCtx, nil)

			cluster = clusterInfo.Cluster
		})

		Context("wait for gemini to handle", func() {
			var (
				backupKey types.NamespacedName
				backup    *dpv1alpha1.Backup
			)

			getJobKey := func() client.ObjectKey {
				return client.ObjectKey{
					Name:      dpbackup.GenerateBackupJobName(backup, dpbackup.BackupDataJobNamePrefix+"-0"),
					Namespace: backup.Namespace,
				}
			}

			BeforeEach(func() {
				By("making the backupPolicy to use kopia")
				Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(backupPolicy),
					func(policy *dpv1alpha1.BackupPolicy) {
						policy.Spec.UseKopia = true
					})).Should(Succeed())
				By("creating a backup from backupPolicy " + testdp.BackupPolicyName)
				backup = testdp.NewFakeBackup(&testCtx, nil)
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should continue to process after gemini acknowledged", func() {
				By("check backup status")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).Should(BeEmpty())
				})).Should(Succeed())

				By("simulate gemini processing")
				Eventually(testapps.GetAndChangeObj(&testCtx, backupKey, func(backup *dpv1alpha1.Backup) {
					if backup.Annotations == nil {
						backup.Annotations = map[string]string{}
					}
					backup.Annotations[dptypes.GeminiAcknowledgedAnnotationKey] = trueVal
				})).Should(Succeed())

				By("check backup status again")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.PersistentVolumeClaimName).Should(Equal(repoPVCName))
					g.Expect(fetched.Status.Path).Should(Equal(dpbackup.BuildBaseBackupPath(fetched, "", backupPolicy.Spec.PathPrefix)))
					g.Expect(fetched.Status.KopiaRepoPath).Should(Equal(dpbackup.BuildKopiaRepoPath(fetched, "", backupPolicy.Spec.PathPrefix)))
					g.Expect(fetched.Status.Phase).Should(Equal(dpv1alpha1.BackupPhaseRunning))
					g.Expect(fetched.Annotations[constant.EncryptedSystemAccountsAnnotationKey]).ShouldNot(BeEmpty())
				})).Should(Succeed())

				testdp.PatchK8sJobStatus(&testCtx, getJobKey(), batchv1.JobComplete)

				By("backup job should have completed")
				Eventually(testapps.CheckObj(&testCtx, getJobKey(), func(g Gomega, fetched *batchv1.Job) {
					_, finishedType, _ := dputils.IsJobFinished(fetched)
					g.Expect(fetched.Labels[constant.AppManagedByLabelKey]).Should(Equal(dptypes.AppName))
					g.Expect(finishedType).To(Equal(batchv1.JobComplete))
				})).Should(Succeed())

				By("backup should have completed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
					g.Expect(fetched.Labels[dptypes.ClusterUIDLabelKey]).Should(Equal(string(cluster.UID)))
					g.Expect(fetched.Labels[constant.AppInstanceLabelKey]).Should(Equal(testdp.ClusterName))
					g.Expect(fetched.Labels[constant.KBAppComponentLabelKey]).Should(Equal(testdp.ComponentName))
					g.Expect(fetched.Labels[constant.AppManagedByLabelKey]).Should(Equal(dptypes.AppName))
					g.Expect(fetched.Annotations[constant.ClusterSnapshotAnnotationKey]).ShouldNot(BeEmpty())
				})).Should(Succeed())

				By("backup job should be deleted after backup completed")
				Eventually(testapps.CheckObjExists(&testCtx, getJobKey(), &batchv1.Job{}, false)).Should(Succeed())
			})
		})
	})
})
