/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

func NewFakeIncActionSet(testCtx *testutil.TestContext) *dpv1alpha1.ActionSet {
	return NewFakeActionSet(testCtx, func(as *dpv1alpha1.ActionSet) {
		as.Name = IncActionSetName
		as.Spec.BackupType = dpv1alpha1.BackupTypeIncremental
	})
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

func MockActionSetWithSchema(testCtx *testutil.TestContext, actionSet *dpv1alpha1.ActionSet) {
	Expect(testapps.ChangeObj(testCtx, actionSet, func(as *dpv1alpha1.ActionSet) {
		as.Spec.ParametersSchema = &dpv1alpha1.ActionSetParametersSchema{
			OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					ParameterString: {
						Type: ParameterStringType,
					},
					ParameterArray: {
						Type: ParameterArrayType,
						Items: &apiextensionsv1.JSONSchemaPropsOrArray{
							Schema: &apiextensionsv1.JSONSchemaProps{
								Type: ParameterStringType,
							},
						},
					},
				},
			},
		}
		as.Spec.Backup.WithParameters = []string{ParameterString, ParameterArray}
		as.Spec.Restore.WithParameters = []string{ParameterString, ParameterArray}
	})).Should(Succeed())
	By("the actionSet should be available")
	Eventually(testapps.CheckObj(testCtx, client.ObjectKeyFromObject(actionSet),
		func(g Gomega, as *dpv1alpha1.ActionSet) {
			g.Expect(as.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.AvailablePhase))
			g.Expect(as.Status.Message).Should(BeEmpty())
		})).Should(Succeed())
}
