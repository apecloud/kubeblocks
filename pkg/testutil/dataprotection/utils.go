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

package dataprotection

import (
	. "github.com/onsi/gomega"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

func PatchK8sJobStatus(testCtx *testutil.TestContext, key client.ObjectKey, jobStatus batchv1.JobConditionType) {
	Eventually(testapps.GetAndChangeObjStatus(testCtx, key, func(fetched *batchv1.Job) {
		jobCondition := batchv1.JobCondition{Type: jobStatus, Status: corev1.ConditionTrue}
		fetched.Status.Conditions = append(fetched.Status.Conditions, jobCondition)
	})).Should(Succeed())
}

func ReplaceK8sJobStatus(testCtx *testutil.TestContext, key client.ObjectKey, jobStatus batchv1.JobConditionType) {
	Eventually(testapps.GetAndChangeObjStatus(testCtx, key, func(fetched *batchv1.Job) {
		jobCondition := batchv1.JobCondition{Type: jobStatus, Status: corev1.ConditionTrue}
		fetched.Status.Conditions = []batchv1.JobCondition{jobCondition}
	})).Should(Succeed())
}

func PatchVolumeSnapshotStatus(testCtx *testutil.TestContext, key client.ObjectKey, readyToUse bool) {
	Eventually(testapps.GetAndChangeObjStatus(testCtx, key, func(fetched *vsv1.VolumeSnapshot) {
		snapStatus := vsv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
		fetched.Status = &snapStatus
	})).Should(Succeed())
}

func PatchBackupStatus(testCtx *testutil.TestContext, key client.ObjectKey, status dpv1alpha1.BackupStatus) {
	Eventually(testapps.GetAndChangeObjStatus(testCtx, key, func(fetched *dpv1alpha1.Backup) {
		fetched.Status = status
	})).Should(Succeed())
}
