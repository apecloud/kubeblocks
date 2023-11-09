/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupScheduleSignature, true, inNS)
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

			scheduler *Scheduler
		)

		BeforeEach(func() {
			backupPolicy = testdp.NewBackupPolicyFactory(testCtx.DefaultNamespace, testdp.BackupPolicyName).
				SetBackupRepoName(testdp.BackupRepoName).
				SetPathPrefix(testdp.BackupPathPrefix).
				SetTarget(constant.AppInstanceLabelKey, testdp.ClusterName,
					constant.KBAppComponentLabelKey, testdp.ComponentName,
					constant.RoleLabelKey, constant.Leader).
				AddBackupMethod(testdp.BackupMethodName, false, testdp.ActionSetName).
				AddBackupMethod(testdp.VSBackupMethodName, true, "").
				Create(&testCtx).GetObject()
			backupSchedule = testdp.NewFakeBackupSchedule(&testCtx, nil)

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
		})
	})
})
