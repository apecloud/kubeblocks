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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

var _ = Describe("BackupRepo controller", func() {
	const namespace2 = "namespace2"
	const pvcProtectionFinalizer = "kubernetes.io/pvc-protection"

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// non-namespaced
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.BackupRepoSignature, true, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StorageProviderSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.CSIDriverSignature, ml)

		// namespaced
		inNS := client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS))
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.BackupSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PersistentVolumeClaimSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StorageClassSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.SecretSignature, inNS, ml)

		// namespace2
		inNS2 := client.InNamespace(namespace2)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.BackupSignature, true, inNS2, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PersistentVolumeClaimSignature, true, inNS2, ml)

		// delete namespace2
		Eventually(func(g Gomega) {
			// from https://github.com/kubernetes-sigs/controller-runtime/issues/880#issuecomment-749742403
			namespaceObj := &corev1.Namespace{}
			err := testCtx.Cli.Get(testCtx.Ctx, types.NamespacedName{Name: namespace2}, namespaceObj)
			if apierrors.IsNotFound(err) {
				return
			}
			namespaceObj.Spec.Finalizers = []corev1.FinalizerName{}
			// We have to use the k8s.io/client-go library here to expose
			// ability to patch the /finalize subresource on the namespace
			clientGo, err := kubernetes.NewForConfig(testEnv.Config)
			Expect(err).Should(Succeed())
			_, err = clientGo.CoreV1().Namespaces().Finalize(testCtx.Ctx, namespaceObj, metav1.UpdateOptions{})
			Expect(err).Should(Succeed())
		}).Should(Succeed())

		// By("deleting the Namespace to perform the tests")
		// Eventually(func(g Gomega) {
		// 	namespace := testCtx.GetNamespaceObj()
		// 	err := testCtx.Cli.Delete(testCtx.Ctx, &namespace)
		// 	g.Expect(client.IgnoreNotFound(err)).To(Not(HaveOccurred()))
		// 	g.Expect(client.IgnoreNotFound(testCtx.Cli.Get(
		// 		testCtx.Ctx, testCtx.GetNamespaceKey(), &namespace))).To(Not(HaveOccurred()))
		// }).Should(Succeed())
	}

	ensureNamespace := func(name string) {
		Eventually(func(g Gomega) {
			obj := &corev1.Namespace{}
			obj.Name = name
			err := testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(obj), &corev1.Namespace{})
			if err == nil {
				return
			}
			g.Expect(client.IgnoreNotFound(err)).Should(Succeed())
			err = testCtx.Cli.Create(testCtx.Ctx, obj)
			g.Expect(err).Should(Succeed())
		}).Should(Succeed())
	}

	BeforeEach(func() {
		viper.SetDefault(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)
		cleanEnv()
		ensureNamespace(namespace2)
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("BackupRepo controller test", func() {
		const defaultCSIDriverName = "default.csi.driver"
		var credentialSecretKey types.NamespacedName
		var repoKey types.NamespacedName
		var providerKey types.NamespacedName
		var repo *dpv1alpha1.BackupRepo

		createCredentialSecretSpec := func() {
			obj := &corev1.Secret{}
			obj.GenerateName = "credential-"
			obj.Namespace = testCtx.DefaultNamespace
			obj.StringData = map[string]string{
				"cred-key1": "cred-val1",
				"cred-key2": "cred-val2",
			}
			secret := testapps.CreateK8sResource(&testCtx, obj)
			credentialSecretKey = client.ObjectKeyFromObject(secret)
		}

		createBackupRepoSpec := func(mutateFunc func(repo *dpv1alpha1.BackupRepo)) {
			obj := &dpv1alpha1.BackupRepo{}
			obj.GenerateName = "backuprepo-"
			obj.Spec = dpv1alpha1.BackupRepoSpec{
				StorageProviderRef: providerKey.Name,
				VolumeCapacity:     resource.MustParse("100Gi"),
				PVReclaimPolicy:    corev1.PersistentVolumeReclaimRetain,
				Config: map[string]string{
					"key1": "val1",
					"key2": "val2",
				},
				Credential: &corev1.SecretReference{
					Name:      credentialSecretKey.Name,
					Namespace: credentialSecretKey.Namespace,
				},
			}
			if mutateFunc != nil {
				mutateFunc(obj)
			}
			repo = testapps.CreateK8sResource(&testCtx, obj).(*dpv1alpha1.BackupRepo)
			repoKey = client.ObjectKeyFromObject(repo)
		}

		createStorageProviderSpec := func(mutateFunc func(repo *storagev1alpha1.StorageProvider)) {
			obj := &storagev1alpha1.StorageProvider{}
			obj.GenerateName = "storageprovider-"
			obj.Spec.CSIDriverName = defaultCSIDriverName
			obj.Spec.CSIDriverSecretTemplate = `
value-of-key1: {{ index .Parameters "key1" }}
value-of-key2: {{ index .Parameters "key2" }}
value-of-cred-key1: {{ index .Parameters "cred-key1" }}
value-of-cred-key2: {{ index .Parameters "cred-key2" }}
`
			obj.Spec.StorageClassTemplate = `
provisioner: default.csi.driver
parameters:
    value-of-key1: {{ index .Parameters "key1" }}
    value-of-key2: {{ index .Parameters "key2" }}
    value-of-cred-key1: {{ index .Parameters "cred-key1" }}
    value-of-cred-key2: {{ index .Parameters "cred-key2" }}
    secret-name: {{ .CSIDriverSecretRef.Name }}
    secret-namespace: {{ .CSIDriverSecretRef.Namespace }}
`
			obj.Status.Phase = storagev1alpha1.StorageProviderReady
			if mutateFunc != nil {
				mutateFunc(obj)
			}
			provider := testapps.CreateK8sResource(&testCtx, obj.DeepCopy())
			providerKey = client.ObjectKeyFromObject(provider)
			// update status
			newObj := provider.(*storagev1alpha1.StorageProvider)
			patch := client.MergeFrom(newObj.DeepCopy())
			newObj.Status = obj.Status
			Expect(testCtx.Cli.Status().Patch(testCtx.Ctx, newObj, patch)).NotTo(HaveOccurred())
		}

		createCSIDriverObjectSpec := func(driverName string) {
			obj := &storagev1.CSIDriver{}
			obj.Name = driverName
			testapps.CreateK8sResource(&testCtx, obj)
		}

		createBackupSpec := func(mutateFunc func(backup *dpv1alpha1.Backup)) *dpv1alpha1.Backup {
			obj := &dpv1alpha1.Backup{}
			obj.GenerateName = "backup-"
			obj.Namespace = testCtx.DefaultNamespace
			obj.Labels = map[string]string{
				dataProtectionBackupRepoKey:  repoKey.Name,
				dataProtectionNeedRepoPVCKey: "true",
			}
			obj.Spec.BackupType = dpv1alpha1.BackupTypeSnapshot
			obj.Spec.BackupPolicyName = "default"
			if mutateFunc != nil {
				mutateFunc(obj)
			}
			return testapps.CreateK8sResource(&testCtx, obj).(*dpv1alpha1.Backup)
		}

		deleteBackup := func(g Gomega, key types.NamespacedName) {
			backupObj := &dpv1alpha1.Backup{}
			err := testCtx.Cli.Get(testCtx.Ctx, key, backupObj)
			if apierrors.IsNotFound(err) {
				return
			}
			g.Expect(err).ShouldNot(HaveOccurred())
			// remove finalizers
			backupObj.Finalizers = nil
			err = testCtx.Cli.Update(testCtx.Ctx, backupObj)
			g.Expect(err).ShouldNot(HaveOccurred())
			// delete the backup
			err = testCtx.Cli.Delete(testCtx.Ctx, backupObj)
			g.Expect(err).ShouldNot(HaveOccurred())
		}

		BeforeEach(func() {
			cleanEnv()
			Expect(client.IgnoreAlreadyExists(testCtx.CreateNamespace())).To(Not(HaveOccurred()))
			createCredentialSecretSpec()
			createCSIDriverObjectSpec(defaultCSIDriverName)
			createStorageProviderSpec(nil)
			createBackupRepoSpec(nil)
		})

		AfterEach(func() {
			cleanEnv()
		})

		It("should monitor the status of the storage provider", func() {
			By("creating a BackupRepo which is referencing a non-existent storage provider")
			createBackupRepoSpec(func(repo *dpv1alpha1.BackupRepo) {
				repo.Spec.StorageProviderRef = "myprovider" // not exist for now
			})
			By("checking the status of the BackupRepo, should be not ready")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeStorageProviderReady)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(metav1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonStorageProviderNotFound))
				g.Expect(repo.Status.Phase).To(Equal(dpv1alpha1.BackupRepoFailed))
			})).Should(Succeed())

			By("creating the required storage provider")
			createStorageProviderSpec(func(provider *storagev1alpha1.StorageProvider) {
				provider.GenerateName = ""
				provider.Name = "myprovider"
			})

			By("checking the status of the BackupRepo, should become ready")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeStorageProviderReady)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(metav1.ConditionTrue))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonStorageProviderReady))
				g.Expect(repo.Status.Phase).To(Equal(dpv1alpha1.BackupRepoReady))
			})).Should(Succeed())

			By("updating the status of the storage provider to not ready")
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, providerKey, func(provider *storagev1alpha1.StorageProvider) {
				provider.Status.Phase = storagev1alpha1.StorageProviderNotReady
			})).Should(Succeed())
			By("checking the status of the BackupRepo, should become failed")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeStorageProviderReady)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(metav1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonStorageProviderNotReady))
				g.Expect(repo.Status.Phase).To(Equal(dpv1alpha1.BackupRepoFailed))
			})).Should(Succeed())

			By("deleting the storage provider")
			testapps.DeleteObject(&testCtx, providerKey, &storagev1alpha1.StorageProvider{})
			By("checking the status of the BackupRepo, condition should become NotFound")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeStorageProviderReady)
				g.Expect(cond).ToNot(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(metav1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonStorageProviderNotFound))
				g.Expect(repo.Status.Phase).To(Equal(dpv1alpha1.BackupRepoFailed))
			})).Should(Succeed())
		})

		It("should create StorageClass and Secret for the CSI driver", func() {
			var secretRef corev1.SecretReference
			var storageClassName string
			By("checking the BackupRepo, should be ready")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.Phase).Should(Equal(dpv1alpha1.BackupRepoReady))
				g.Expect(repo.Status.GeneratedCSIDriverSecret).NotTo(BeNil())
				g.Expect(repo.Status.GeneratedStorageClassName).NotTo(BeEmpty())
				g.Expect(repo.Status.BackupPVCName).NotTo(BeEmpty())
				secretRef = *repo.Status.GeneratedCSIDriverSecret
				storageClassName = repo.Status.GeneratedStorageClassName
			})).Should(Succeed())

			By("checking the Secret")
			secretKey := types.NamespacedName{Name: secretRef.Name, Namespace: secretRef.Namespace}
			Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
				g.Expect(secret.Data).To(Equal(map[string][]byte{
					"value-of-key1":      []byte("val1"),
					"value-of-key2":      []byte("val2"),
					"value-of-cred-key1": []byte("cred-val1"),
					"value-of-cred-key2": []byte("cred-val2"),
				}))
				g.Expect(isOwned(repo, secret)).To(BeTrue())
				g.Expect(secret.Labels[dataProtectionBackupRepoKey]).To(Equal(repoKey.Name))
			})).Should(Succeed())

			By("checking the StorageClass")
			storageClassNameKey := types.NamespacedName{Name: storageClassName}
			Eventually(testapps.CheckObj(&testCtx, storageClassNameKey, func(g Gomega, storageClass *storagev1.StorageClass) {
				g.Expect(storageClass.Parameters).To(Equal(map[string]string{
					"value-of-key1":      "val1",
					"value-of-key2":      "val2",
					"value-of-cred-key1": "cred-val1",
					"value-of-cred-key2": "cred-val2",
					"secret-name":        secretKey.Name,
					"secret-namespace":   secretKey.Namespace,
				}))
				g.Expect(isOwned(repo, storageClass)).To(BeTrue())
				g.Expect(storageClass.Labels[dataProtectionBackupRepoKey]).To(Equal(repoKey.Name))
				g.Expect(storageClass.Provisioner).To(Equal(defaultCSIDriverName))
				g.Expect(*storageClass.ReclaimPolicy).To(Equal(corev1.PersistentVolumeReclaimRetain))
				g.Expect(*storageClass.VolumeBindingMode).To(Equal(storagev1.VolumeBindingImmediate))
			})).Should(Succeed())
		})

		It("should update the Secret object if the template or values got changed", func() {
			By("checking the Secret")
			var secretKey types.NamespacedName
			var reversion string
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.GeneratedCSIDriverSecret).NotTo(BeNil())
				secretKey = types.NamespacedName{
					Name:      repo.Status.GeneratedCSIDriverSecret.Name,
					Namespace: repo.Status.GeneratedCSIDriverSecret.Namespace,
				}
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
				reversion = secret.ResourceVersion
			})).Should(Succeed())

			By("updating the template")
			Eventually(testapps.GetAndChangeObj(&testCtx, providerKey, func(provider *storagev1alpha1.StorageProvider) {
				provider.Spec.CSIDriverSecretTemplate += "\nnew-item: new-value"
			})).Should(Succeed())
			By("checking the Secret again, should have new generation and new content")
			Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
				g.Expect(secret.Data).To(Equal(map[string][]byte{
					"value-of-key1":      []byte("val1"),
					"value-of-key2":      []byte("val2"),
					"value-of-cred-key1": []byte("cred-val1"),
					"value-of-cred-key2": []byte("cred-val2"),
					"new-item":           []byte("new-value"),
				}))
				g.Expect(secret.ResourceVersion).ToNot(Equal(reversion))
				reversion = secret.ResourceVersion
			})).Should(Succeed())

			By("updating the config")
			Eventually(testapps.GetAndChangeObj(&testCtx, repoKey, func(repo *dpv1alpha1.BackupRepo) {
				repo.Spec.Config["key1"] = "changed-val1"
			})).Should(Succeed())
			By("checking the Secret again, should have new generation and new content")
			Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
				g.Expect(secret.Data).To(Equal(map[string][]byte{
					"value-of-key1":      []byte("changed-val1"),
					"value-of-key2":      []byte("val2"),
					"value-of-cred-key1": []byte("cred-val1"),
					"value-of-cred-key2": []byte("cred-val2"),
					"new-item":           []byte("new-value"),
				}))
				g.Expect(secret.ResourceVersion).ToNot(Equal(reversion))
				reversion = secret.ResourceVersion
			})).Should(Succeed())

			By("updating the credential")
			Eventually(testapps.GetAndChangeObj(&testCtx, credentialSecretKey, func(secret *corev1.Secret) {
				secret.Data["cred-key1"] = []byte("changed-cred-val1")
			})).Should(Succeed())
			By("checking the Secret again, should have new generation and new content")
			Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
				g.Expect(secret.Data).To(Equal(map[string][]byte{
					"value-of-key1":      []byte("changed-val1"),
					"value-of-key2":      []byte("val2"),
					"value-of-cred-key1": []byte("changed-cred-val1"),
					"value-of-cred-key2": []byte("cred-val2"),
					"new-item":           []byte("new-value"),
				}))
				g.Expect(secret.ResourceVersion).ToNot(Equal(reversion))
				reversion = secret.ResourceVersion
			})).Should(Succeed())
		})

		It("should fail if the secret referenced by the credential secret not found", func() {
			By("checking the repo object to make sure it's ready")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.Phase).Should(Equal(dpv1alpha1.BackupRepoReady))
			})).Should(Succeed())
			By("updating to a non-existing credential")
			Eventually(testapps.GetAndChangeObj(&testCtx, repoKey, func(repo *dpv1alpha1.BackupRepo) {
				repo.Spec.Credential.Name += "non-existing"
			})).Should(Succeed())
			By("checking the repo object again, it should be failed")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.Phase).Should(Equal(dpv1alpha1.BackupRepoFailed))
			})).Should(Succeed())
		})

		It("should fail if the secret template is invalid", func() {
			By("setting a invalid template")
			Eventually(testapps.GetAndChangeObj(&testCtx, providerKey, func(provider *storagev1alpha1.StorageProvider) {
				provider.Spec.CSIDriverSecretTemplate = "{{ bad template }"
			})).Should(Succeed())
			By("checking the repo status")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.Phase).Should(Equal(dpv1alpha1.BackupRepoFailed))
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeStorageClassCreated)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(corev1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonBadSecretTemplate))
				g.Expect(cond.Message).Should(ContainSubstring(`function "bad" not defined`))
			})).Should(Succeed())
		})

		It("should fail if the render result of the secret template is not a yaml", func() {
			By("setting a invalid template")
			Eventually(testapps.GetAndChangeObj(&testCtx, providerKey, func(provider *storagev1alpha1.StorageProvider) {
				provider.Spec.CSIDriverSecretTemplate = "bad yaml"
			})).Should(Succeed())
			By("checking the repo status")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.Phase).Should(Equal(dpv1alpha1.BackupRepoFailed))
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeStorageClassCreated)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(corev1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonBadSecretTemplate))
				g.Expect(cond.Message).Should(ContainSubstring(`cannot unmarshal string into Go value of type map[string]string`))
			})).Should(Succeed())
		})

		It("should fail if the storage class template is invalid", func() {
			By("setting a invalid template")
			Eventually(testapps.GetAndChangeObj(&testCtx, providerKey, func(provider *storagev1alpha1.StorageProvider) {
				provider.Spec.StorageClassTemplate = "{{ bad template }"
			})).Should(Succeed())
			By("creating a new repo to reference the provider")
			createBackupRepoSpec(nil)
			By("checking the repo status")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.Phase).Should(Equal(dpv1alpha1.BackupRepoFailed))
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeStorageClassCreated)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(corev1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonBadStorageClassTemplate))
				g.Expect(cond.Message).Should(ContainSubstring(`function "bad" not defined`))
			})).Should(Succeed())
		})

		It("should fail if the render result of the storage class template is not a yaml", func() {
			By("setting a invalid template")
			Eventually(testapps.GetAndChangeObj(&testCtx, providerKey, func(provider *storagev1alpha1.StorageProvider) {
				provider.Spec.StorageClassTemplate = "bad yaml"
			})).Should(Succeed())
			By("creating a new repo to reference the provider")
			createBackupRepoSpec(nil)
			By("checking the repo status")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.Phase).Should(Equal(dpv1alpha1.BackupRepoFailed))
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeStorageClassCreated)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(corev1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonBadStorageClassTemplate))
				g.Expect(cond.Message).Should(ContainSubstring(`cannot unmarshal string into Go value of type v1.StorageClass`))
			})).Should(Succeed())
		})

		createBackupAndCheckPVC := func(namespace string) (backup *dpv1alpha1.Backup, pvcName string) {
			By("making sure the repo is ready")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.Phase).Should(Equal(dpv1alpha1.BackupRepoReady))
				g.Expect(repo.Status.BackupPVCName).ShouldNot(BeEmpty())
				pvcName = repo.Status.BackupPVCName
			})).Should(Succeed())
			By("creating a Backup object in the namespace")
			backup = createBackupSpec(func(backup *dpv1alpha1.Backup) {
				backup.Namespace = namespace
			})
			By("updating the status of the Backup to completed")
			Eventually(func(g Gomega) {
				obj := &dpv1alpha1.Backup{}
				err := testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(backup), obj)
				g.Expect(err).ShouldNot(HaveOccurred())
				if obj.Status.Phase == dpv1alpha1.BackupFailed {
					// the controller will set the status to failed because
					// essential objects (e.g. backup policy) are missed.
					// we set the status to completed after that, to avoid conflict.
					obj.Status.Phase = dpv1alpha1.BackupCompleted
					err = testCtx.Cli.Status().Update(testCtx.Ctx, obj)
					g.Expect(err).ShouldNot(HaveOccurred())
				} else {
					// check again
					g.Expect(false).Should(BeTrue())
				}
			}).Should(Succeed())
			By("checking the PVC has been created in the namespace")
			pvcKey := types.NamespacedName{
				Name:      pvcName,
				Namespace: namespace,
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc).ShouldNot(BeNil())
			})).Should(Succeed())
			return backup, pvcName
		}

		It("should create a PVC in Backup's namespace (in default namespace)", func() {
			createBackupAndCheckPVC(testCtx.DefaultNamespace)
		})

		It("should create a PVC in Backup's namespace (in namespace2)", func() {
			createBackupAndCheckPVC(namespace2)
		})

		It("should block the deletion of the BackupRepo if derived objects are not deleted", func() {
			backup, pvcName := createBackupAndCheckPVC(namespace2)

			By("deleting the BackupRepo")
			testapps.DeleteObject(&testCtx, repoKey, &dpv1alpha1.BackupRepo{})

			By("checking the BackupRepo, the deletion should be blocked because there are associated backups")
			Eventually(func(g Gomega) {
				repo := &dpv1alpha1.BackupRepo{}
				err := testCtx.Cli.Get(testCtx.Ctx, repoKey, repo)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(repo.DeletionTimestamp).ShouldNot(BeNil())
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeDerivedObjectsDeleted)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(corev1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonHaveAssociatedBackups))
			}).Should(Succeed())

			By("deleting the Backup")
			Eventually(func(g Gomega) {
				deleteBackup(g, client.ObjectKeyFromObject(backup))
			}).Should(Succeed())

			By("checking the BackupRepo, the deletion should be blocked because the PVC is still present")
			Eventually(func(g Gomega) {
				repo := &dpv1alpha1.BackupRepo{}
				err := testCtx.Cli.Get(testCtx.Ctx, repoKey, repo)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(repo.DeletionTimestamp).ShouldNot(BeNil())
				cond := meta.FindStatusCondition(repo.Status.Conditions, ConditionTypeDerivedObjectsDeleted)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).Should(BeEquivalentTo(corev1.ConditionFalse))
				g.Expect(cond.Reason).Should(BeEquivalentTo(ReasonHaveResidualPVCs))
			}).Should(Succeed())

			By("releasing the PVC")
			pvcKey := types.NamespacedName{
				Namespace: namespace2,
				Name:      pvcName,
			}
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, pvcKey, func(pvc *corev1.PersistentVolumeClaim) {
				controllerutil.RemoveFinalizer(pvc, pvcProtectionFinalizer)
			})).Should(Succeed())

			By("checking the BackupRepo, it should have been deleted")
			Eventually(func(g Gomega) {
				repo := &dpv1alpha1.BackupRepo{}
				err := testCtx.Cli.Get(testCtx.Ctx, repoKey, repo)
				g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			}).Should(Succeed())

			By("checking derived objects should be all deleted")
			Eventually(func(g Gomega) {
				// get the newest repo object
				repo := &dpv1alpha1.BackupRepo{}
				err := testCtx.Cli.Get(testCtx.Ctx, repoKey, repo)
				if apierrors.IsNotFound(err) {
					return
				}
				g.Expect(err).ShouldNot(HaveOccurred())

				// check the secret for the CSI driver
				err = testCtx.Cli.Get(testCtx.Ctx, types.NamespacedName{
					Name:      repo.Status.GeneratedCSIDriverSecret.Name,
					Namespace: repo.Status.GeneratedCSIDriverSecret.Namespace,
				}, &corev1.Secret{})
				g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())

				// check the storage class
				err = testCtx.Cli.Get(testCtx.Ctx, types.NamespacedName{
					Name: repo.Status.GeneratedStorageClassName,
				}, &storagev1.StorageClass{})
				g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())

				// check the PVC
				pvc := &corev1.PersistentVolumeClaim{}
				err = testCtx.Cli.Get(testCtx.Ctx, pvcKey, pvc)
				g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			}).Should(Succeed())
		})

		It("should update backupRepo.status.isDefault", func() {
			By("making the repo default")
			Eventually(testapps.GetAndChangeObj(&testCtx, repoKey, func(repo *dpv1alpha1.BackupRepo) {
				repo.Annotations = map[string]string{
					constant.DefaultBackupRepoAnnotationKey: trueVal,
				}
			})).Should(Succeed())
			By("checking the repo is default")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.IsDefault).Should(BeTrue())
			})).Should(Succeed())

			By("making the repo non default")
			Eventually(testapps.GetAndChangeObj(&testCtx, repoKey, func(repo *dpv1alpha1.BackupRepo) {
				repo.Annotations = nil
			})).Should(Succeed())
			By("checking the repo is not default")
			Eventually(testapps.CheckObj(&testCtx, repoKey, func(g Gomega, repo *dpv1alpha1.BackupRepo) {
				g.Expect(repo.Status.IsDefault).Should(BeFalse())
			})).Should(Succeed())
		})
	})
})
