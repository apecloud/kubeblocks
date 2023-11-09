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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("Request Test", func() {
	buildRequest := func() *Request {
		return &Request{
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

		// namespaced
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
		testapps.ClearResources(&testCtx, generics.VolumeSnapshotClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
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
			backup       *dpv1alpha1.Backup
			actionSet    *dpv1alpha1.ActionSet
			backupPolicy *dpv1alpha1.BackupPolicy
			backupRepo   *dpv1alpha1.BackupRepo

			targetPod *corev1.Pod
			request   *Request
		)

		BeforeEach(func() {
			actionSet = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.WithName(testdp.ActionSetName))
			backup = testdp.NewFakeBackup(&testCtx, nil)
			backupRepo = testapps.CreateCustomizedObj(&testCtx, "backup/backuprepo.yaml", &dpv1alpha1.BackupRepo{}, nil)
			backupPolicy = testdp.NewBackupPolicyFactory(testCtx.DefaultNamespace, testdp.BackupPolicyName).
				SetBackupRepoName(testdp.BackupRepoName).
				SetPathPrefix(testdp.BackupPathPrefix).
				SetTarget(constant.AppInstanceLabelKey, testdp.ClusterName,
					constant.KBAppComponentLabelKey, testdp.ComponentName,
					constant.RoleLabelKey, constant.Leader).
				AddBackupMethod(testdp.BackupMethodName, false, testdp.ActionSetName).
				Create(&testCtx).GetObject()

			targetPod = clusterInfo.TargetPod
			request = buildRequest()
		})

		Context("build action", func() {
			It("should build action", func() {
				request.Backup = backup
				request.ActionSet = actionSet
				request.TargetPods = []*corev1.Pod{targetPod}
				request.BackupPolicy = backupPolicy
				request.BackupMethod = &backupPolicy.Spec.BackupMethods[0]
				request.BackupRepo = backupRepo
				_, err := request.BuildActions()
				Expect(err).NotTo(HaveOccurred())
			})

			It("build create volume snapshot action", func() {
				request.TargetPods = []*corev1.Pod{targetPod}
				request.BackupMethod = &dpv1alpha1.BackupMethod{
					Name:            testdp.VSBackupMethodName,
					SnapshotVolumes: boolptr.True(),
				}
				_, err := request.buildCreateVolumeSnapshotAction()
				Expect(err).Should(HaveOccurred())
			})

		})
	})
})
