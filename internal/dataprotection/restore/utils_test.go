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

package restore

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/internal/testutil/dataprotection"
)

var _ = Describe("Restore Utils Test", func() {

	cleanEnv := func() {
		By("clean resources")
		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// namespaced
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ReferenceGrantSignature, true, inNS)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StorageClassSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("with restore manager functions", func() {
		var (
			actionSet   *dpv1alpha1.ActionSet
			replicas    = 2
			myNamespace = "my-namespace"
		)

		BeforeEach(func() {

			By("create actionSet")
			actionSet = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.WithName(testdp.ActionSetName))

		})

		getReqCtx := func() intctrlutil.RequestCtx {
			return intctrlutil.RequestCtx{
				Ctx: ctx,
				Req: ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: testCtx.DefaultNamespace,
					},
				},
			}
		}

		mockMyNamespace := func() *corev1.Namespace {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: myNamespace,
				},
			}
			Expect(client.IgnoreAlreadyExists(testCtx.CreateObj(ctx, namespace))).Should(Succeed())
			return namespace
		}

		mockRestoreMGR := func(restoreNamespace string, backupPvcName string) *RestoreManager {
			backup := mockBackupForRestore(&testCtx, actionSet.Name, backupPvcName, true, true)
			restore := testdp.NewRestoreactory(restoreNamespace, testdp.RestoreName).
				SetBackup(backup.Name, testCtx.DefaultNamespace).
				SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
					testdp.DataVolumeMountPath, "", int32(replicas), int32(0)).
				Create(&testCtx).Get()
			return NewRestoreManager(restore, recorder, k8sClient.Scheme())
		}

		It("test ValidateAndInitRestoreMGR function, reference the backup with same namespace", func() {
			restoreMGR := mockRestoreMGR(testCtx.DefaultNamespace, testdp.BackupPVCName)
			reqCtx := getReqCtx()
			Expect(ValidateAndInitRestoreMGR(reqCtx, k8sClient, recorder, restoreMGR)).Should(Succeed())
		})

		It("test ValidateAndInitRestoreMGR function, reference the backup with different namespace and no referenceGrant", func() {
			mockMyNamespace()
			restoreMGR := mockRestoreMGR(myNamespace, "")
			reqCtx := getReqCtx()
			err := ValidateAndInitRestoreMGR(reqCtx, k8sClient, recorder, restoreMGR)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("isn't allowed"))
			testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, client.InNamespace(myNamespace))
		})

		It("test ValidateAndInitRestoreMGR function, reference the backup with different namespace and backup with pvc", func() {
			restoreMGR := mockRestoreMGR(myNamespace, testdp.BackupPVCName)
			reqCtx := getReqCtx()
			err := ValidateAndInitRestoreMGR(reqCtx, k8sClient, recorder, restoreMGR)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("isn't supported when the accessMethod of backupRepo"))
		})

		It("test ValidateAndInitRestoreMGR function, reference the backup with different namespace and referenceGrant", func() {
			mockMyNamespace()
			restoreMGR := mockRestoreMGR(myNamespace, "")
			reqCtx := getReqCtx()
			referenceGrant := &gatewayv1beta1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grant-backup",
					Namespace: testCtx.DefaultNamespace,
				},
				Spec: gatewayv1beta1.ReferenceGrantSpec{
					From: []gatewayv1beta1.ReferenceGrantFrom{
						{
							Group:     dptypes.DataprotectionAPIGroup,
							Namespace: gatewayv1beta1.Namespace(myNamespace),
							Kind:      dptypes.RestoreKind,
						},
					},
					To: []gatewayv1beta1.ReferenceGrantTo{
						{
							Group: dptypes.DataprotectionAPIGroup,
							Kind:  dptypes.BackupKind,
						},
					},
				},
			}
			Expect(testCtx.CreateObj(ctx, referenceGrant)).Should(Succeed())
			Expect(ValidateAndInitRestoreMGR(reqCtx, k8sClient, recorder, restoreMGR)).Should(Succeed())
			testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, client.InNamespace(myNamespace))
		})

		It("test getMountPathWithSourceVolume function", func() {
			backup := mockBackupForRestore(&testCtx, actionSet.Name, testdp.BackupPVCName, true, true)
			Expect(getMountPathWithSourceVolume(backup, testdp.DataVolumeName)).Should(Equal(testdp.DataVolumeMountPath))
		})

	})

})
