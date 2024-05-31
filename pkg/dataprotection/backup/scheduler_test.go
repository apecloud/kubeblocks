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

package backup

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("Scheduler Test", func() {
	buildScheduler := func() *Scheduler {
		return &Scheduler{
			RequestCtx: ctrlutil.RequestCtx{
				Log:      logger,
				Ctx:      testCtx.Ctx,
				Recorder: recorder,
			},
			Client: testCtx.Cli,
		}
	}

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
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, ml, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupScheduleSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	When("with default settings", func() {
		var (
			backupPolicy   *dpv1alpha1.BackupPolicy
			backupSchedule *dpv1alpha1.BackupSchedule
			actionSet      *dpv1alpha1.ActionSet
			scheduler      *Scheduler
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			By("creating an actionSet")
			actionSet = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.WithName(testdp.ActionSetName))
			clusterInfo := testdp.NewFakeCluster(&testCtx)
			cluster = clusterInfo.Cluster
			backupPolicy = testdp.NewBackupPolicyFactory(testCtx.DefaultNamespace, testdp.BackupPolicyName).
				SetBackupRepoName(testdp.BackupRepoName).
				SetPathPrefix(testdp.BackupPathPrefix).
				SetTarget(constant.AppInstanceLabelKey, testdp.ClusterName,
					constant.KBAppComponentLabelKey, testdp.ComponentName,
					constant.RoleLabelKey, constant.Leader).
				AddBackupMethod(testdp.BackupMethodName, false, actionSet.Name).
				AddBackupMethod(testdp.VSBackupMethodName, true, "").
				Create(&testCtx).GetObject()
			backupSchedule = testdp.NewFakeBackupSchedule(&testCtx, func(schedule *dpv1alpha1.BackupSchedule) {
				schedule.OwnerReferences = []v1.OwnerReference{
					{
						APIVersion: appsv1alpha1.APIVersion,
						Kind:       "Cluster",
						Name:       cluster.Name,
						UID:        cluster.UID,
					},
				}
			})

			scheduler = buildScheduler()
		})

		Context("test Schedule", func() {
			It("should schedule", func() {
				scheduler.BackupSchedule = backupSchedule
				scheduler.BackupPolicy = backupPolicy
				Expect(scheduler.Schedule()).Should(Succeed())
			})

			It("schedule should fail if invalid backup policy", func() {
				scheduler.BackupSchedule = backupSchedule
				scheduler.BackupPolicy = backupPolicy
				for i := range scheduler.BackupPolicy.Spec.BackupMethods {
					scheduler.BackupPolicy.Spec.BackupMethods[i].Name = "not-exist"
				}
				Expect(scheduler.Schedule()).ShouldNot(Succeed())
			})

			It("test schedule for continuous backup", func() {
				By("set actionSet type to Continuous")
				Expect(testapps.ChangeObj(&testCtx, actionSet, func(as *dpv1alpha1.ActionSet) {
					actionSet.Spec.BackupType = dpv1alpha1.BackupTypeContinuous
				})).Should(Succeed())

				By("enable backupMethod and do scheduling")
				Expect(testapps.ChangeObj(&testCtx, backupSchedule, func(schedule *dpv1alpha1.BackupSchedule) {
					for i, v := range backupSchedule.Spec.Schedules {
						if v.BackupMethod == testdp.BackupMethodName {
							backupSchedule.Spec.Schedules[i].Enabled = pointer.Bool(true)
							break
						}
					}
				})).Should(Succeed())

				scheduler.BackupPolicy = backupPolicy
				scheduler.BackupSchedule = backupSchedule
				Expect(scheduler.Schedule()).Should(Succeed())

				By("check the continuous backup created")
				backupName := GenerateCRNameByBackupSchedule(backupSchedule, testdp.BackupMethodName)
				Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKey{Name: backupName, Namespace: testCtx.DefaultNamespace},
					&dpv1alpha1.Backup{}, true)).Should(Succeed())

				By("re-create backupSchedule and enable the method")
				Expect(k8sClient.Delete(ctx, backupSchedule)).Should(Succeed())
				backupSchedule = testdp.NewFakeBackupSchedule(&testCtx, func(schedule *dpv1alpha1.BackupSchedule) {
					schedule.OwnerReferences = []v1.OwnerReference{
						{
							APIVersion: appsv1alpha1.APIVersion,
							Kind:       "Cluster",
							Name:       cluster.Name,
							UID:        cluster.UID,
						},
					}
					for i, v := range backupSchedule.Spec.Schedules {
						if v.BackupMethod == testdp.BackupMethodName {
							backupSchedule.Spec.Schedules[i].Enabled = pointer.Bool(true)
							break
						}
					}
				})
				By("Expect only one continuous backup to exist")
				scheduler.BackupSchedule = backupSchedule
				Expect(scheduler.Schedule()).Should(Succeed())
				Eventually(testapps.List(&testCtx, generics.BackupSignature, client.MatchingLabels{
					dptypes.BackupTypeLabelKey:     string(dpv1alpha1.BackupTypeContinuous),
					dptypes.BackupScheduleLabelKey: backupSchedule.Name,
				}, client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(1))
			})
		})
	})
})
