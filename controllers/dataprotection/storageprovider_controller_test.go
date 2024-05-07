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

	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("StorageProvider controller", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// non-namespaced
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.StorageProviderSignature, true, ml)
		testapps.ClearResources(&testCtx, intctrlutil.CSIDriverSignature, ml)

		// namespaced
		inNS := client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS))

		// delete rest mocked objects
		testapps.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.SecretSignature, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("StorageProvider controller test", func() {
		var key types.NamespacedName
		var reconclier *StorageProviderReconciler
		BeforeEach(func() {
			cleanEnv()
			Expect(client.IgnoreAlreadyExists(testCtx.CreateNamespace())).To(Not(HaveOccurred()))
			reconclier = &StorageProviderReconciler{
				Client:   k8sManager.GetClient(),
				Scheme:   k8sManager.GetScheme(),
				Recorder: k8sManager.GetEventRecorderFor("storage-provider-controller"),
			}
		})

		AfterEach(func() {
			cleanEnv()
		})

		createStorageProviderSpec := func(driverName string) {
			obj := &dpv1alpha1.StorageProvider{}
			obj.GenerateName = "storageprovider-"
			obj.Spec.CSIDriverName = driverName
			provider := testapps.CreateK8sResource(&testCtx, obj)
			key = types.NamespacedName{
				Name: provider.GetName(),
			}
		}

		createCSIDriverObjectSpec := func(driverName string) {
			obj := &storagev1.CSIDriver{}
			obj.Name = driverName
			testapps.CreateK8sResource(&testCtx, obj)
		}

		deleteCSIDriverObject := func(driverName string) {
			Eventually(func(g Gomega) {
				obj := &storagev1.CSIDriver{}
				obj.Name = driverName
				ExpectWithOffset(2, testCtx.Cli.Delete(testCtx.Ctx, obj)).ShouldNot(HaveOccurred())
			}).Should(Succeed())
		}

		getProvider := func(g Gomega) *dpv1alpha1.StorageProvider {
			provider := &dpv1alpha1.StorageProvider{}
			g.ExpectWithOffset(1, testCtx.Cli.Get(ctx, key, provider)).To(Not(HaveOccurred()))
			return provider
		}

		shouldReady := func(g Gomega, provider *dpv1alpha1.StorageProvider) {
			g.ExpectWithOffset(1, provider.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.StorageProviderReady))

			val := meta.IsStatusConditionTrue(provider.Status.Conditions,
				dpv1alpha1.ConditionTypeCSIDriverInstalled)
			g.ExpectWithOffset(1, val).Should(BeTrue())
		}

		shouldNotReady := func(g Gomega, provider *dpv1alpha1.StorageProvider) {
			g.ExpectWithOffset(1, provider.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.StorageProviderNotReady))

			val := meta.IsStatusConditionPresentAndEqual(
				provider.Status.Conditions,
				dpv1alpha1.ConditionTypeCSIDriverInstalled,
				metav1.ConditionUnknown)
			g.ExpectWithOffset(1, val).Should(BeTrue())
		}

		reconcileNoError := func() {
			_, err := reconclier.Reconcile(testCtx.Ctx, reconcile.Request{NamespacedName: key})
			Expect(err).ToNot(HaveOccurred())
		}

		It("should reconcile a StorageProvider to Ready status if it doesn't specify csiDriverName", func() {
			By("creating a StorageProvider with an empty csiDriverName")
			createStorageProviderSpec("")

			By("checking status.phase and status.conditions")
			Eventually(func(g Gomega) {
				reconcileNoError()
				shouldReady(g, getProvider(g))
			}).Should(Succeed())
		})

		It("should reconcile a StorageProvider to Ready status if the specified csiDriverName is present", func() {
			By("creating a StorageProvider with csi1")
			createCSIDriverObjectSpec("csi1")
			createStorageProviderSpec("csi1")

			By("checking status.phase and status.conditions")
			Eventually(func(g Gomega) {
				reconcileNoError()
				provider := getProvider(g)
				g.Expect(provider.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.StorageProviderReady))

				val := meta.IsStatusConditionTrue(provider.Status.Conditions,
					dpv1alpha1.ConditionTypeCSIDriverInstalled)
				g.Expect(val).Should(BeTrue())
			}).Should(Succeed())
		})

		It("should reconcile a StorageProvider base on the status of the CSI driver object", func() {
			By("creating a StorageProvider with csi2")
			createStorageProviderSpec("csi2")
			By("checking status.phase, it should be NotReady because CSI driver is not installed yet")
			Eventually(func(g Gomega) {
				reconcileNoError()
				shouldNotReady(g, getProvider(g))
			}).Should(Succeed())

			By("creating CSI driver object csi2")
			createCSIDriverObjectSpec("csi2")
			By("checking status.phase, it should become Ready")
			Eventually(func(g Gomega) {
				reconcileNoError()
				shouldReady(g, getProvider(g))
			}).Should(Succeed())

			By("deleting CSI driver object csi2")
			deleteCSIDriverObject("csi2")
			By("checking status.phase, it should become NotReady")
			Eventually(func(g Gomega) {
				reconcileNoError()
				shouldNotReady(g, getProvider(g))
			}).Should(Succeed())
		})

		It("should able to delete a StorageProvider", func() {
			By("creating a StorageProvider with csi3")
			createStorageProviderSpec("csi3")

			By("checking StorageProvider object")
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, provider *dpv1alpha1.StorageProvider) {
				reconcileNoError()
				g.Expect(provider.GetFinalizers()).To(ContainElement(dptypes.DataProtectionFinalizerName))
			})).Should(Succeed())

			By("deleting StorageProvider object")
			Eventually(func(g Gomega) {
				reconcileNoError()
				provider := &dpv1alpha1.StorageProvider{}
				err := testCtx.Cli.Get(ctx, key, provider)
				if apierrors.IsNotFound(err) {
					return
				}
				g.Expect(err).ToNot(HaveOccurred())
				Expect(testCtx.Cli.Delete(testCtx.Ctx, provider)).ToNot(HaveOccurred())
			}).Should(Succeed())
		})
	})
})
