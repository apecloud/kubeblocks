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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
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

func fakeActionSet(testCtx *testutil.TestContext, clusterDefName string) *dpv1alpha1.ActionSet {
	actionSet := &dpv1alpha1.ActionSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ActionSetName,
			Labels: map[string]string{},
		},
		Spec: dpv1alpha1.ActionSetSpec{
			Env: []corev1.EnvVar{
				{
					Name:  "test-name",
					Value: "test-value",
				},
			},
			BackupType: dpv1alpha1.BackupTypeFull,
			Backup: &dpv1alpha1.BackupActionSpec{
				BackupData: &dpv1alpha1.BackupDataActionSpec{
					JobActionSpec: dpv1alpha1.JobActionSpec{
						BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
							Image:   "xtrabackup",
							Command: []string{""},
						},
					},
				},
			},
			Restore: &dpv1alpha1.RestoreActionSpec{
				PrepareData: &dpv1alpha1.JobActionSpec{
					BaseJobActionSpec: dpv1alpha1.BaseJobActionSpec{
						Image: "xtrabackup",
						Command: []string{
							"sh",
							"-c",
							"/backup_scripts.sh",
						},
					},
				},
			},
		},
	}
	if len(clusterDefName) > 0 {
		actionSet.Labels[constant.ClusterDefLabelKey] = clusterDefName
	}
	testapps.CheckedCreateK8sResource(testCtx, actionSet)
	return actionSet
}

func CreateBackupPolicyTpl(testCtx *testutil.TestContext) *dpv1alpha1.BackupPolicyTemplate {
	By("create actionSet")
	fakeActionSet(testCtx, "")

	By("Creating a BackupPolicyTemplate")
	ttl := "7d"
	return NewBackupPolicyTemplateFactory(BackupPolicyTPLName).AddBackupMethod(BackupMethodName, false, ActionSetName).
		SetBackupMethodVolumeMounts("data", "/data").
		AddBackupMethod(VSBackupMethodName, true, "").
		SetBackupMethodVolumes([]string{"data"}).
		AddSchedule(BackupMethodName, "0 0 * * *", ttl, true).
		AddSchedule(VSBackupMethodName, "0 0 * * *", ttl, true).
		Create(testCtx).Get()
}

func CheckRestoreAndSetCompleted(testCtx *testutil.TestContext, clusterKey types.NamespacedName, compName string, scaleOutReplicas int) {
	By("Checking restore CR created")
	ml := client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterKey.Name,
		constant.KBAppComponentLabelKey: compName,
		constant.KBManagedByKey:         "cluster",
	}
	Eventually(testapps.List(testCtx, generics.RestoreSignature,
		ml, client.InNamespace(clusterKey.Namespace))).Should(HaveLen(scaleOutReplicas))

	By("Mocking restore phase to succeeded")
	MockRestoreCompleted(testCtx, ml)
}

func MockRestoreCompleted(testCtx *testutil.TestContext, ml client.MatchingLabels) {
	restoreList := dpv1alpha1.RestoreList{}
	Expect(testCtx.Cli.List(testCtx.Ctx, &restoreList, ml)).Should(Succeed())
	for _, rs := range restoreList.Items {
		err := testapps.GetAndChangeObjStatus(testCtx, client.ObjectKeyFromObject(&rs), func(res *dpv1alpha1.Restore) {
			res.Status.Phase = dpv1alpha1.RestorePhaseCompleted
		})()
		Expect(err).ShouldNot(HaveOccurred())
	}
}
