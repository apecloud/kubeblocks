/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Volume Populator Controller test", func() {
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
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterSignature, true, inNS)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StorageClassSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("backup target selector helpers", func() {
		It("detects effective PVC selectors beyond the app instance label", func() {
			Expect(backupTargetHasEffectivePVCSelector(nil)).To(BeFalse())
			Expect(backupTargetHasEffectivePVCSelector(&dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{
					PodSelector: &dpv1alpha1.PodSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								constant.AppInstanceLabelKey: "cluster",
							},
						},
					},
				},
			})).To(BeFalse())
			Expect(backupTargetHasEffectivePVCSelector(&dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{
					PodSelector: &dpv1alpha1.PodSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      constant.VolumeClaimTemplateNameLabelKey,
								Operator: metav1.LabelSelectorOpExists,
							}},
						},
					},
				},
			})).To(BeTrue())
		})

		It("matches PVCs with label selector operators", func() {
			target := &dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{
					PodSelector: &dpv1alpha1.PodSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								constant.AppInstanceLabelKey:             "cluster",
								constant.VolumeClaimTemplateNameLabelKey: "data",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"hot", "warm"}},
								{Key: "archived", Operator: metav1.LabelSelectorOpDoesNotExist},
							},
						},
					},
				},
			}
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constant.AppInstanceLabelKey:             "cluster",
						constant.VolumeClaimTemplateNameLabelKey: "data",
						"tier":                                   "hot",
					},
				},
			}

			Expect(backupTargetMatchesPVC(&dpv1alpha1.BackupStatusTarget{}, pvc)).To(BeFalse())
			Expect(backupTargetMatchesPVC(target, pvc)).To(BeTrue())

			target.PodSelector.MatchExpressions[0].Values = []string{"cold"}
			Expect(backupTargetMatchesPVC(target, pvc)).To(BeFalse())

			target.PodSelector.MatchExpressions[0] = metav1.LabelSelectorRequirement{
				Key:      "tier",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"cold"},
			}
			Expect(backupTargetMatchesPVC(target, pvc)).To(BeTrue())

			target.PodSelector.MatchExpressions[0].Values = []string{"hot"}
			Expect(backupTargetMatchesPVC(target, pvc)).To(BeFalse())

			target.PodSelector.MatchExpressions[0] = metav1.LabelSelectorRequirement{
				Key:      "tier",
				Operator: metav1.LabelSelectorOpExists,
			}
			Expect(backupTargetMatchesPVC(target, pvc)).To(BeTrue())

			target.PodSelector.MatchExpressions[0] = metav1.LabelSelectorRequirement{
				Key:      "tier",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			}
			Expect(backupTargetMatchesPVC(target, pvc)).To(BeFalse())

			target.PodSelector.MatchExpressions[0] = metav1.LabelSelectorRequirement{
				Key:      "tier",
				Operator: metav1.LabelSelectorOperator("Unknown"),
			}
			Expect(backupTargetMatchesPVC(target, pvc)).To(BeFalse())
		})
	})

	Context("system account secret helpers", func() {
		It("patches existing mutable secrets and recreates changed immutable secrets", func() {
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).Should(Succeed())
			Expect(kbappsv1.AddToScheme(scheme)).Should(Succeed())

			component := &kbappsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
					UID:       types.UID("component-uid"),
				},
			}
			mutableSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      constant.GenerateAccountSecretName("cluster", "mysql", "admin"),
				},
				Data: map[string][]byte{},
			}
			immutable := true
			immutableSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      constant.GenerateAccountSecretName("cluster", "mysql", "root"),
				},
				Immutable: &immutable,
				Data: map[string][]byte{
					constant.AccountNameForSecret:   []byte("root"),
					constant.AccountPasswdForSecret: []byte("old-password"),
				},
			}
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "data-target-0",
				},
			}
			reconciler := &VolumePopulatorReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(component, mutableSecret, immutableSecret).
					Build(),
				Scheme: scheme,
			}
			reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

			Expect(reconciler.upsertSystemAccountSecret(reqCtx, pvc, systemAccountSecretScopeComponent,
				"cluster", "mysql", "admin", []byte("new-password"), map[string]string{"role": "admin"})).Should(Succeed())
			patched := &corev1.Secret{}
			Expect(reconciler.Client.Get(context.Background(), client.ObjectKey{
				Namespace: "default",
				Name:      mutableSecret.Name,
			}, patched)).Should(Succeed())
			Expect(patched.Labels["role"]).To(Equal("admin"))
			Expect(patched.Annotations[constant.SystemAccountProvisionedAnnotationKey]).To(Equal("true"))
			Expect(patched.Data[constant.AccountNameForSecret]).To(Equal([]byte("admin")))
			Expect(patched.Data[constant.AccountPasswdForSecret]).To(Equal([]byte("new-password")))
			Expect(patched.OwnerReferences).To(HaveLen(1))
			Expect(patched.OwnerReferences[0].Kind).To(Equal("Component"))

			Expect(reconciler.upsertSystemAccountSecret(reqCtx, pvc, systemAccountSecretScopeComponent,
				"cluster", "mysql", "root", []byte("new-root-password"), map[string]string{"role": "root"})).Should(Succeed())
			recreated := &corev1.Secret{}
			Expect(reconciler.Client.Get(context.Background(), client.ObjectKey{
				Namespace: "default",
				Name:      immutableSecret.Name,
			}, recreated)).Should(Succeed())
			Expect(recreated.Immutable).To(BeNil())
			Expect(recreated.Labels["role"]).To(Equal("root"))
			Expect(recreated.Data[constant.AccountPasswdForSecret]).To(Equal([]byte("new-root-password")))
			Expect(systemAccountSecretMatches(recreated, "root", []byte("new-root-password"))).To(BeTrue())
			Expect(systemAccountSecretMatches(recreated, "root", []byte("old-password"))).To(BeFalse())
		})

		It("validates PVC names against instance volume templates", func() {
			instance := &workloadsv1.Instance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "cluster-mysql-0",
				},
				Spec: workloadsv1.InstanceSpec{
					InstanceSetName: "cluster-mysql",
					VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
						ObjectMeta: metav1.ObjectMeta{Name: "data"},
					}},
				},
			}
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "data-cluster-mysql-0",
					Labels: map[string]string{
						constant.KBAppPodNameLabelKey: "cluster-mysql-0",
					},
					Annotations: map[string]string{
						constant.RestoreVolumeTemplateAnnotationKey: "data",
					},
				},
			}

			Expect(validatePVCMatchesInstanceTemplate(pvc, instance)).Should(Succeed())
			Expect(pvcConditionMatches([]corev1.PersistentVolumeClaimCondition{{
				Type:   "Restore",
				Status: corev1.ConditionTrue,
				Reason: "Ready",
			}}, corev1.PersistentVolumeClaimCondition{
				Type:   "Restore",
				Status: corev1.ConditionTrue,
				Reason: "Ready",
			})).To(BeTrue())
			Expect(pvcConditionMatches(nil, corev1.PersistentVolumeClaimCondition{Type: "Restore"})).To(BeFalse())

			pvc.Labels[constant.KBAppPodNameLabelKey] = ""
			Expect(validatePVCMatchesInstanceTemplate(pvc, instance)).ShouldNot(Succeed())
			pvc.Labels[constant.KBAppPodNameLabelKey] = "cluster-mysql-1"
			Expect(validatePVCMatchesInstanceTemplate(pvc, instance)).ShouldNot(Succeed())
			pvc.Labels[constant.KBAppPodNameLabelKey] = "cluster-mysql-0"
			pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey] = "logs"
			Expect(validatePVCMatchesInstanceTemplate(pvc, instance)).ShouldNot(Succeed())
		})
	})

	When("volume populator controller test", func() {
		var (
			actionSet   *dpv1alpha1.ActionSet
			pvcName     = "data-mysql-mysql-0"
			storageSize = "20Gi"
			// intreeProvisioner = "kubernetes.io/no-provisioner"
			csiProvisioner = "csi.test.io/provisioner"
		)

		createStorageClass := func(volumeBinding storagev1.VolumeBindingMode) *storagev1.StorageClass {
			storageClass := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: testdp.StorageClassName,
				},
				Provisioner:       csiProvisioner,
				VolumeBindingMode: &volumeBinding,
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, storageClass)).Should(Succeed())
			return storageClass
		}

		BeforeEach(func() {
			By("create actionSet")
			actionSet = testdp.NewFakeActionSet(&testCtx, nil)
		})

		initResources := func(volumeBinding storagev1.VolumeBindingMode, useVolumeSnapshotBackup, mockBackupCompleted bool) *corev1.PersistentVolumeClaim {
			By("create storageClass")
			createStorageClass(volumeBinding)

			By("create backup")
			backup := mockBackupForRestore(actionSet.Name, "", "", mockBackupCompleted, useVolumeSnapshotBackup, "")

			By("create PVC and set spec.dataSourceRef to backup")
			pvc := testapps.NewPersistentVolumeClaimFactory(
				testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
				SetAnnotations(map[string]string{
					constant.RestoreSourceNamespaceAnnotationKey: testCtx.DefaultNamespace,
					constant.RestoreVolumeTemplateAnnotationKey:  testdp.DataVolumeName,
				}).
				SetStorage(storageSize).
				SetStorageClass(testdp.StorageClassName).
				SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.BackupKind, backup.Name).
				Create(&testCtx).GetObject()
			return pvc
		}

		mockPV := func(populatePVCName string) *corev1.PersistentVolume {
			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pv",
				},
				Spec: corev1.PersistentVolumeSpec{
					StorageClassName: testdp.StorageClassName,
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Capacity: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse(storageSize),
					},
					ClaimRef: &corev1.ObjectReference{
						Namespace: testCtx.DefaultNamespace,
						Name:      populatePVCName,
					},
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "kubernetes.io",
							VolumeHandle: "test-volume-handle",
						},
					},
				},
			}
			Expect(testCtx.Create(ctx, pv)).Should(Succeed())
			populatePVC := &corev1.PersistentVolumeClaim{}
			// bind pv
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: populatePVCName, Namespace: testCtx.DefaultNamespace}, populatePVC)).Should(Succeed())
			Expect(testapps.ChangeObj(&testCtx, populatePVC, func(c *corev1.PersistentVolumeClaim) {
				c.Spec.VolumeName = pv.Name
			})).Should(Succeed())
			return pv
		}

		checkJobsSA := func(jobList *batchv1.JobList) {
			for _, job := range jobList.Items {
				Expect(job.Spec.Template.Spec.ServiceAccountName).Should(Equal(viper.GetString(dptypes.CfgKeyWorkerServiceAccountName)))
			}
		}

		findPVCCondition := func(pvc *corev1.PersistentVolumeClaim, conditionType corev1.PersistentVolumeClaimConditionType) *corev1.PersistentVolumeClaimCondition {
			for i := range pvc.Status.Conditions {
				if pvc.Status.Conditions[i].Type == conditionType {
					return &pvc.Status.Conditions[i]
				}
			}
			return nil
		}

		findRestoreCondition := func(restore *dpv1alpha1.Restore, conditionType string) *metav1.Condition {
			for i := range restore.Status.Conditions {
				if restore.Status.Conditions[i].Type == conditionType {
					return &restore.Status.Conditions[i]
				}
			}
			return nil
		}

		testVolumePopulate := func(volumeBinding storagev1.VolumeBindingMode, useVolumeSnapshotBackup bool) {
			pvc := initResources(volumeBinding, useVolumeSnapshotBackup, true)

			pvcKey := client.ObjectKeyFromObject(pvc)
			if volumeBinding == storagev1.VolumeBindingWaitForFirstConsumer {
				By("wait for pvc has selected the node")
				Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
					g.Expect(len(tmpPVC.Status.Conditions)).Should(Equal(0))
				})).Should(Succeed())
			}

			By("mock pvc has selected the node")
			Expect(testapps.ChangeObj(&testCtx, pvc, func(claim *corev1.PersistentVolumeClaim) {
				if claim.Annotations == nil {
					claim.Annotations = map[string]string{}
				}
				claim.Annotations[AnnSelectedNode] = "test-node"
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				populatingCondition := findPVCCondition(tmpPVC, PersistentVolumeClaimPopulating)
				g.Expect(populatingCondition).ShouldNot(BeNil())
				g.Expect(populatingCondition.Reason).Should(Equal(ReasonPopulatingProcessing))
				restoreCondition := findPVCCondition(tmpPVC, corev1.PersistentVolumeClaimConditionType(kbappsv1.ConditionTypeRestore))
				g.Expect(restoreCondition).ShouldNot(BeNil())
				g.Expect(restoreCondition.Status).Should(Equal(corev1.ConditionUnknown))
			})).Should(Succeed())

			By("expect for populate pvc created")
			populatePVCName := getPopulatePVCName(pvc.UID)
			populatePVC := &corev1.PersistentVolumeClaim{}
			Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, populatePVC, true))
			Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, func(g Gomega, restore *dpv1alpha1.Restore) {
				g.Expect(restore.Spec.Backup.Name).Should(Equal(pvc.Spec.DataSourceRef.Name))
				g.Expect(restore.Spec.PrepareDataConfig.DataSourceRef.VolumeSource).Should(Equal(testdp.DataVolumeName))
				g.Expect(restore.OwnerReferences).ShouldNot(BeEmpty())
				g.Expect(restore.OwnerReferences[0].UID).Should(Equal(pvc.UID))
			})).Should(Succeed())

			By("expect for job created")
			Eventually(testapps.List(&testCtx, generics.JobSignature,
				client.MatchingLabels{dprestore.DataProtectionPopulatePVCLabelKey: populatePVCName},
				client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(1))

			By("mock to create pv and bind to populate pvc")
			pv := mockPV(populatePVCName)

			By("mock job to succeed")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList,
				client.MatchingLabels{dprestore.DataProtectionPopulatePVCLabelKey: getPopulatePVCName(pvc.UID)},
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			checkJobsSA(jobList)
			testdp.ReplaceK8sJobStatus(&testCtx, client.ObjectKeyFromObject(&jobList.Items[0]), batchv1.JobComplete)

			By("expect for pvc has been populated")
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				populatingCondition := findPVCCondition(tmpPVC, PersistentVolumeClaimPopulating)
				g.Expect(populatingCondition).ShouldNot(BeNil())
				g.Expect(populatingCondition.Reason).Should(Equal(ReasonPopulatingSucceed))
				restoreCondition := findPVCCondition(tmpPVC, corev1.PersistentVolumeClaimConditionType(kbappsv1.ConditionTypeRestore))
				g.Expect(restoreCondition).ShouldNot(BeNil())
				g.Expect(restoreCondition.Status).Should(Equal(corev1.ConditionTrue))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, func(g Gomega, restore *dpv1alpha1.Restore) {
				condition := findRestoreCondition(restore, dprestore.ConditionTypeRestorePreparedData)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Status).Should(Equal(metav1.ConditionTrue))
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pv), func(g Gomega, tmpPV *corev1.PersistentVolume) {
				g.Expect(tmpPV.Spec.ClaimRef.Name).Should(Equal(pvc.Name))
				g.Expect(tmpPV.Spec.ClaimRef.UID).Should(Equal(pvc.UID))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				g.Expect(tmpPVC.Spec.VolumeName).Should(Equal(pv.Name))
			})).Should(Succeed())

			By("expect for resources are cleaned up")
			Eventually(testapps.List(&testCtx, generics.JobSignature,
				client.MatchingLabels{dprestore.DataProtectionPopulatePVCLabelKey: populatePVCName},
				client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(0))
			Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, populatePVC, false))
		}

		Context("test volume populator", func() {
			It("test VolumePopulator when volumeBinding of storageClass is WaitForFirstConsumer", func() {
				testVolumePopulate(storagev1.VolumeBindingWaitForFirstConsumer, false)
			})

			It("test VolumePopulator when volumeBinding of storageClass is Immediate", func() {
				testVolumePopulate(storagev1.VolumeBindingImmediate, false)
			})

			It("test VolumePopulator when backup uses volume snapshot", func() {
				testVolumePopulate(storagev1.VolumeBindingWaitForFirstConsumer, true)
			})

			It("infers source target from PVC labels for multi-target backups", func() {
				createStorageClass(storagev1.VolumeBindingImmediate)
				backup := mockBackupForRestore(actionSet.Name, "", "", true, false, "")
				Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
					backup.Status.Target = nil
					backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{
						{
							BackupTarget: dpv1alpha1.BackupTarget{
								Name: "other",
								PodSelector: &dpv1alpha1.PodSelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											constant.AppInstanceLabelKey:    "source-cluster",
											constant.KBAppComponentLabelKey: "other",
										},
									},
									Strategy: dpv1alpha1.PodSelectionStrategyAny,
								},
							},
						},
						{
							BackupTarget: dpv1alpha1.BackupTarget{
								Name: "mysql",
								PodSelector: &dpv1alpha1.PodSelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											constant.AppInstanceLabelKey:    "source-cluster",
											constant.KBAppComponentLabelKey: testdp.ComponentName,
										},
									},
									Strategy: dpv1alpha1.PodSelectionStrategyAny,
								},
							},
						},
					}
				})).Should(Succeed())

				pvc := testapps.NewPersistentVolumeClaimFactory(
					testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
					SetAnnotations(map[string]string{
						constant.RestoreSourceNamespaceAnnotationKey: testCtx.DefaultNamespace,
						constant.RestoreVolumeTemplateAnnotationKey:  testdp.DataVolumeName,
					}).
					SetStorage(storageSize).
					SetStorageClass(testdp.StorageClassName).
					SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.BackupKind, backup.Name).
					Create(&testCtx).GetObject()

				Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
					Name: getPopulatePVCName(pvc.UID)}, func(g Gomega, restore *dpv1alpha1.Restore) {
					g.Expect(restore.Spec.Backup.SourceTargetName).Should(Equal("mysql"))
				})).Should(Succeed())
			})

			It("infers source target pod for all-pod target backups", func() {
				createStorageClass(storagev1.VolumeBindingImmediate)
				backup := mockBackupForRestore(actionSet.Name, "", "", true, false, "")
				Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
					backup.Status.Target = nil
					backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{{
						BackupTarget: dpv1alpha1.BackupTarget{
							Name: "mysql",
							PodSelector: &dpv1alpha1.PodSelector{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										constant.KBAppComponentLabelKey: testdp.ComponentName,
									},
								},
								Strategy: dpv1alpha1.PodSelectionStrategyAll,
							},
						},
						SelectedTargetPods: []string{"source-mysql-0", "source-mysql-1"},
					}}
				})).Should(Succeed())

				pvc := testapps.NewPersistentVolumeClaimFactory(
					testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
					SetAnnotations(map[string]string{
						constant.RestoreSourceNamespaceAnnotationKey: testCtx.DefaultNamespace,
						constant.RestoreVolumeTemplateAnnotationKey:  testdp.DataVolumeName,
					}).
					AddLabels(constant.KBAppPodNameLabelKey, "target-mysql-1").
					SetStorage(storageSize).
					SetStorageClass(testdp.StorageClassName).
					SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.BackupKind, backup.Name).
					Create(&testCtx).GetObject()

				Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
					Name: getPopulatePVCName(pvc.UID)}, func(g Gomega, restore *dpv1alpha1.Restore) {
					requiredPolicy := restore.Spec.PrepareDataConfig.RequiredPolicyForAllPodSelection
					g.Expect(restore.Spec.Backup.SourceTargetName).Should(Equal("mysql"))
					g.Expect(requiredPolicy).ShouldNot(BeNil())
					g.Expect(requiredPolicy.DataRestorePolicy).Should(Equal(dpv1alpha1.OneToManyRestorePolicy))
					g.Expect(requiredPolicy.SourceOfOneToMany).ShouldNot(BeNil())
					g.Expect(requiredPolicy.SourceOfOneToMany.TargetPodName).Should(Equal("source-mysql-1"))
				})).Should(Succeed())
			})

			It("uses explicit source target pod annotation when instance template ordinals overlap", func() {
				createStorageClass(storagev1.VolumeBindingImmediate)
				backup := mockBackupForRestore(actionSet.Name, "", "", true, false, "")
				Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
					backup.Status.Target = nil
					backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{{
						BackupTarget: dpv1alpha1.BackupTarget{
							Name: "mysql",
							PodSelector: &dpv1alpha1.PodSelector{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										constant.KBAppComponentLabelKey: testdp.ComponentName,
									},
								},
								Strategy: dpv1alpha1.PodSelectionStrategyAll,
							},
						},
						SelectedTargetPods: []string{"source-mysql-tpl-a-1", "source-mysql-tpl-b-1"},
					}}
				})).Should(Succeed())

				pvc := testapps.NewPersistentVolumeClaimFactory(
					testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
					SetAnnotations(map[string]string{
						constant.RestoreSourceNamespaceAnnotationKey: testCtx.DefaultNamespace,
						constant.RestoreVolumeTemplateAnnotationKey:  testdp.DataVolumeName,
						constant.RestoreParametersAnnotationKey:      fmt.Sprintf(`{"%s":"source-mysql-tpl-b-1"}`, dptypes.SourceTargetPodNameAnnotationKey),
					}).
					AddLabels(constant.KBAppPodNameLabelKey, "target-mysql-tpl-b-1").
					AddLabels(constant.KBAppInstanceTemplateLabelKey, "tpl-b").
					SetStorage(storageSize).
					SetStorageClass(testdp.StorageClassName).
					SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.BackupKind, backup.Name).
					Create(&testCtx).GetObject()

				Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
					Name: getPopulatePVCName(pvc.UID)}, func(g Gomega, restore *dpv1alpha1.Restore) {
					requiredPolicy := restore.Spec.PrepareDataConfig.RequiredPolicyForAllPodSelection
					g.Expect(requiredPolicy).ShouldNot(BeNil())
					g.Expect(requiredPolicy.SourceOfOneToMany).ShouldNot(BeNil())
					g.Expect(requiredPolicy.SourceOfOneToMany.TargetPodName).Should(Equal("source-mysql-tpl-b-1"))
				})).Should(Succeed())
			})

			It("restores system account secrets before volume population", func() {
				pvc := initResources(storagev1.VolumeBindingWaitForFirstConsumer, false, true)
				Expect(testCtx.CreateObj(testCtx.Ctx, &kbappsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      testdp.ClusterName,
					},
					Spec: kbappsv1.ClusterSpec{
						TerminationPolicy: kbappsv1.Delete,
					},
				})).Should(Succeed())
				Expect(testCtx.CreateObj(testCtx.Ctx, &kbappsv1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      constant.GenerateClusterComponentName(testdp.ClusterName, testdp.ComponentName),
					},
					Spec: kbappsv1.ComponentSpec{
						TerminationPolicy: kbappsv1.Delete,
						CompDef:           testdp.ComponentName,
						Replicas:          1,
					},
				})).Should(Succeed())
				pvcKey := client.ObjectKeyFromObject(pvc)
				backupKey := types.NamespacedName{Namespace: testCtx.DefaultNamespace, Name: pvc.Spec.DataSourceRef.Name}
				encryptor := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
				encryptedPassword, err := encryptor.Encrypt([]byte("restored-password"))
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(testapps.GetAndChangeObj(&testCtx, backupKey, func(backup *dpv1alpha1.Backup) {
					if backup.Annotations == nil {
						backup.Annotations = map[string]string{}
					}
					backup.Annotations[constant.EncryptedSystemAccountsAnnotationKey] = fmt.Sprintf(`{"%s":{"admin":"%s"}}`, testdp.ComponentName, encryptedPassword)
				})).Should(Succeed())

				Expect(testapps.ChangeObj(&testCtx, pvc, func(claim *corev1.PersistentVolumeClaim) {
					if claim.Annotations == nil {
						claim.Annotations = map[string]string{}
					}
					claim.Annotations[AnnSelectedNode] = "test-node"
				})).Should(Succeed())

				secretKey := types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      constant.GenerateAccountSecretName(testdp.ClusterName, testdp.ComponentName, "admin"),
				}
				Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
					g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
					g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("restored-password")))
					g.Expect(secret.Annotations).Should(HaveKeyWithValue(constant.SystemAccountProvisionedAnnotationKey, "true"))
				})).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
					restoreCondition := findPVCCondition(tmpPVC, corev1.PersistentVolumeClaimConditionType(kbappsv1.ConditionTypeRestore))
					g.Expect(restoreCondition).ShouldNot(BeNil())
					g.Expect(restoreCondition.Status).Should(Equal(corev1.ConditionUnknown))
				})).Should(Succeed())
			})

			It("test VolumePopulator when it fails", func() {
				pvc := initResources(storagev1.VolumeBindingImmediate, false, false)
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pvc), func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
					populatingCondition := findPVCCondition(tmpPVC, PersistentVolumeClaimPopulating)
					g.Expect(populatingCondition).ShouldNot(BeNil())
					g.Expect(populatingCondition.Reason).Should(Equal(ReasonPopulatingFailed))
					restoreCondition := findPVCCondition(tmpPVC, corev1.PersistentVolumeClaimConditionType(kbappsv1.ConditionTypeRestore))
					g.Expect(restoreCondition).ShouldNot(BeNil())
					g.Expect(restoreCondition.Status).Should(Equal(corev1.ConditionFalse))
				})).Should(Succeed())

			})

		})
	})
})

func TestResolveSourceTargetPodNameRequiresExplicitMappingForInstanceTemplate(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{},
				Strategy:      dpv1alpha1.PodSelectionStrategyAll,
			},
		},
		SelectedTargetPods: []string{"source-mysql-tpl-a-1", "source-mysql-tpl-b-1"},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-mysql-tpl-b-1",
			Labels: map[string]string{
				constant.KBAppPodNameLabelKey:          "target-mysql-tpl-b-1",
				constant.KBAppInstanceTemplateLabelKey: "tpl-b",
			},
			Annotations: map[string]string{},
		},
	}

	sourcePodName, err := resolveSourceTargetPodName(target, pvc)
	require.Error(t, err)
	require.Empty(t, sourcePodName)

	parameters, err := json.Marshal(map[string]string{
		dptypes.SourceTargetPodNameAnnotationKey: "source-mysql-tpl-b-1",
	})
	require.NoError(t, err)
	pvc.Annotations[constant.RestoreParametersAnnotationKey] = string(parameters)
	sourcePodName, err = resolveSourceTargetPodName(target, pvc)
	require.NoError(t, err)
	require.Equal(t, "source-mysql-tpl-b-1", sourcePodName)
}

func TestRestoreParametersKeepRuntimeSettingsOutOfActionParameters(t *testing.T) {
	parameters := map[string]string{
		"restore-param":                                       "restore-value",
		dptypes.SourceTargetPodNameAnnotationKey:              "source-mysql-0",
		dptypes.VolumeRestorePolicyParameterKey:               string(dpv1alpha1.VolumeClaimRestorePolicySerial),
		dptypes.DeferPostReadyUntilClusterRunningParameterKey: "true",
	}

	actionParameters := restoreActionParameters(parameters)

	require.Equal(t, map[string]string{"restore-param": "restore-value"}, actionParameters)
	policy, err := volumeRestorePolicyFromParameters(parameters)
	require.NoError(t, err)
	require.Equal(t, dpv1alpha1.VolumeClaimRestorePolicySerial, policy)
}

func TestRestoreEnvFromParameters(t *testing.T) {
	envJSON, err := json.Marshal([]corev1.EnvVar{{Name: "RESTORE_ENV", Value: "true"}})
	require.NoError(t, err)

	env, err := restoreEnvFromParameters(map[string]string{
		dptypes.RestoreEnvParameterKey: string(envJSON),
	})

	require.NoError(t, err)
	require.Equal(t, []corev1.EnvVar{{Name: "RESTORE_ENV", Value: "true"}}, env)
}

func TestDecidePVCRestoreUsesTargetVolumes(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	pvc := newPVCForRestoreDecision("logs", "mysql", "")

	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)

	pvc = newPVCForRestoreDecision("data", "mysql", "")
	decision, err = reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeRestoreData, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)
}

func TestDecidePVCRestoreKeepsSingleBackupTargetScopedToMatchingComponent(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision([]string{"data"}, []string{"redis"})
	backup.Status.Targets[0].PodSelector = &dpv1alpha1.PodSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				constant.AppInstanceLabelKey:    "source-cluster",
				constant.KBAppComponentLabelKey: "redis",
			},
		},
		Strategy: dpv1alpha1.PodSelectionStrategyAny,
	}

	pvc := newPVCForRestoreDecision("data", "redis", "")
	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeRestoreData, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)
	require.Equal(t, "redis", decision.sourceTarget.Name)

	pvc = newPVCForRestoreDecision("data", "redis-sentinel", "")
	decision, err = reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.True(t, decision.skipPostReady)
	require.Nil(t, decision.sourceTarget)
}

func TestDecidePVCRestoreIgnoresRoleLabelInPVCMatch(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	backup.Status.Target.PodSelector = &dpv1alpha1.PodSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				constant.AppInstanceLabelKey:    "source-cluster",
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.KBAppComponentLabelKey: "falkordb",
				constant.RoleLabelKey:           "secondary",
			},
		},
		Strategy: dpv1alpha1.PodSelectionStrategyAny,
	}

	pvc := newPVCForRestoreDecision("data", "falkordb", "")

	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeRestoreData, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)
}

func TestDecidePVCRestoreTreatsNilTargetVolumesAsProvisionOnly(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	pvc := newPVCForRestoreDecision("data", "mysql", "")

	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.False(t, decision.skipPostReady)
}

func TestDecidePVCRestoreNoTargetVolumesTargetComponent(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Target.PodSelector = &dpv1alpha1.PodSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				constant.AppInstanceLabelKey:    "source-cluster",
				constant.KBAppComponentLabelKey: "tidb",
				constant.RoleLabelKey:           "leader",
			},
		},
		Strategy: dpv1alpha1.PodSelectionStrategyAny,
	}

	// PVC from the backup target component: postReady must not be skipped
	pvc := newPVCForRestoreDecision("data", "tidb", "")
	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.False(t, decision.skipPostReady,
		"target component PVC must not skip postReady when no volume-level restore exists")
	require.NotNil(t, decision.sourceTarget,
		"sourceTarget must be recovered for the target component")

	// PVC from a different component: postReady must be skipped
	pvc = newPVCForRestoreDecision("data", "pd", "")
	decision, err = reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.True(t, decision.skipPostReady,
		"non-target component PVC must skip postReady to avoid running restore on wrong component")
	require.Nil(t, decision.sourceTarget)
}

func TestDecidePVCRestoreNoTargetVolumesTargetComponentMatchExpression(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Target.PodSelector = &dpv1alpha1.PodSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				constant.AppInstanceLabelKey: "source-cluster",
				constant.RoleLabelKey:        "leader",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      constant.KBAppComponentLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"tidb"},
			}},
		},
		Strategy: dpv1alpha1.PodSelectionStrategyAny,
	}

	pvc := newPVCForRestoreDecision("data", "tidb", "")
	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.False(t, decision.skipPostReady,
		"target component selected by MatchExpressions must not skip postReady")
	require.NotNil(t, decision.sourceTarget)

	pvc = newPVCForRestoreDecision("data", "pd", "")
	decision, err = reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.True(t, decision.skipPostReady,
		"non-target component must still skip postReady when component expression does not match")
	require.Nil(t, decision.sourceTarget)
}

func TestDecidePVCRestoreNoTargetVolumesMultiComponentTargets(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision(nil, []string{"etcd"})
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Targets[0].PodSelector = &dpv1alpha1.PodSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				constant.AppInstanceLabelKey:    "source-cluster",
				constant.KBAppComponentLabelKey: "etcd",
				constant.RoleLabelKey:           "leader",
			},
		},
		Strategy: dpv1alpha1.PodSelectionStrategyAny,
	}

	// Etcd PVC matches target component — postReady enabled
	pvc := newPVCForRestoreDecision("data", "etcd", "")
	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)
	require.Equal(t, "etcd", decision.sourceTarget.Name)
}

func TestDecidePVCRestoreNoTargetVolumesPodOnlySelectorUsesTarget(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Target.PodSelector = &dpv1alpha1.PodSelector{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				constant.AppInstanceLabelKey: "source-cluster",
				constant.RoleLabelKey:        "leader",
			},
		},
		Strategy: dpv1alpha1.PodSelectionStrategyAny,
	}

	pvc := newPVCForRestoreDecision("data", "mysql", "")
	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.False(t, decision.skipPostReady,
		"pod-only selectors have no effective PVC selector, so the target applies to the PVC")
	require.NotNil(t, decision.sourceTarget)
}

func TestMatchToPopulateSupportsRestoreAndBackupDataSources(t *testing.T) {
	apiGroup := dptypes.DataprotectionAPIGroup
	reconciler := &VolumePopulatorReconciler{}
	tests := []struct {
		name    string
		ref     *corev1.TypedObjectReference
		want    bool
		wantErr bool
	}{
		{
			name: "restore datasource",
			ref: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     dptypes.RestoreKind,
				Name:     "restore",
			},
			want: true,
		},
		{
			name: "backup datasource",
			ref: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     dptypes.BackupKind,
				Name:     "backup",
			},
			want: true,
		},
		{
			name: "restore datasource in another namespace is rejected",
			ref: &corev1.TypedObjectReference{
				APIGroup:  &apiGroup,
				Kind:      dptypes.RestoreKind,
				Name:      "restore",
				Namespace: ptr.To("other"),
			},
			wantErr: true,
		},
		{
			name: "unknown kind is ignored",
			ref: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     "Other",
				Name:     "source",
			},
		},
		{
			name: "other api group is ignored",
			ref: &corev1.TypedObjectReference{
				Kind: dptypes.BackupKind,
				Name: "backup",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "target"},
				Spec: corev1.PersistentVolumeClaimSpec{
					DataSourceRef: tt.ref,
				},
			}

			got, err := reconciler.MatchToPopulate(pvc)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBackupNamespaceFromPVC(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "target", Name: "data"},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: "backup"},
		},
	}

	namespace, err := backupNamespaceFromPVC(pvc)
	require.NoError(t, err)
	require.Equal(t, "target", namespace)

	pvc.Annotations = map[string]string{constant.RestoreSourceNamespaceAnnotationKey: "target"}
	namespace, err = backupNamespaceFromPVC(pvc)
	require.NoError(t, err)
	require.Equal(t, "target", namespace)

	pvc.Spec.DataSourceRef.Namespace = ptr.To("target")
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = "target"
	namespace, err = backupNamespaceFromPVC(pvc)
	require.NoError(t, err)
	require.Equal(t, "target", namespace)

	pvc.Spec.DataSourceRef.Namespace = nil
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = "other"
	namespace, err = backupNamespaceFromPVC(pvc)
	require.NoError(t, err)
	require.Equal(t, "other", namespace)

	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = "other"
	pvc.Spec.DataSourceRef.Namespace = ptr.To("other")
	namespace, err = backupNamespaceFromPVC(pvc)
	require.NoError(t, err)
	require.Equal(t, "other", namespace)

	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = "target"
	_, err = backupNamespaceFromPVC(pvc)
	require.Error(t, err)
}

func TestAuthorizedBackupNamespaceFromPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, workloadsv1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "target",
			Name:      "data-cluster-mysql-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:             "cluster",
				constant.KBAppComponentLabelKey:          "mysql",
				constant.KBAppPodNameLabelKey:            "cluster-mysql-0",
				constant.VolumeClaimTemplateNameLabelKey: "data",
			},
			Annotations: map[string]string{
				constant.RestoreSourceNamespaceAnnotationKey: "source",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     dptypes.BackupKind,
				Name:     "backup",
			},
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	namespace, err := reconciler.authorizedBackupNamespaceFromPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	require.Error(t, err)
	require.Empty(t, namespace)

	cluster := &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pvc.Namespace,
			Name:      "cluster",
		},
		Spec: kbappsv1.ClusterSpec{
			Restore: &kbappsv1.ClusterRestore{
				Source: kbappsv1.ClusterRestoreSource{
					APIGroup:  dptypes.DataprotectionAPIGroup,
					Kind:      dptypes.BackupKind,
					Name:      "backup",
					Namespace: "source",
				},
			},
		},
	}
	reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()

	namespace, err = reconciler.authorizedBackupNamespaceFromPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	require.Error(t, err)
	require.Empty(t, namespace)

	its := &workloadsv1.InstanceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: workloadsv1.GroupVersion.String(),
			Kind:       workloadsv1.InstanceSetKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pvc.Namespace,
			Name:      "cluster-mysql",
			UID:       types.UID("its-uid"),
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "cluster",
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
		Spec: workloadsv1.InstanceSetSpec{
			Replicas: ptr.To[int32](2),
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
			}},
			Instances: []workloadsv1.InstanceTemplate{{
				Name:     "special",
				Replicas: ptr.To[int32](1),
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{Name: "logs"},
				}},
			}},
		},
	}
	pvc.OwnerReferences = []metav1.OwnerReference{{
		APIVersion:         workloadsv1.GroupVersion.String(),
		Kind:               workloadsv1.InstanceSetKind,
		Name:               its.Name,
		UID:                its.UID,
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}}
	reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, its).Build()

	pvc.Name = "data-cluster-mysql-9"
	pvc.Labels[constant.KBAppPodNameLabelKey] = "cluster-mysql-9"
	namespace, err = reconciler.authorizedBackupNamespaceFromPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	require.Error(t, err)
	require.Empty(t, namespace)

	pvc.Name = "logs-cluster-mysql-0"
	pvc.Labels[constant.KBAppPodNameLabelKey] = "cluster-mysql-0"
	pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] = "logs"
	pvc.Labels[constant.KBAppInstanceTemplateLabelKey] = "special"
	namespace, err = reconciler.authorizedBackupNamespaceFromPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	require.Error(t, err)
	require.Empty(t, namespace)

	pvc.Name = "data-cluster-mysql-0"
	pvc.Labels[constant.KBAppPodNameLabelKey] = "cluster-mysql-0"
	pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] = "data"
	delete(pvc.Labels, constant.KBAppInstanceTemplateLabelKey)
	namespace, err = reconciler.authorizedBackupNamespaceFromPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	require.NoError(t, err)
	require.Equal(t, "source", namespace)

	pvc.Name = "logs-cluster-mysql-special-0"
	pvc.Labels[constant.KBAppPodNameLabelKey] = "cluster-mysql-special-0"
	pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] = "logs"
	pvc.Labels[constant.KBAppInstanceTemplateLabelKey] = "special"
	namespace, err = reconciler.authorizedBackupNamespaceFromPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	require.NoError(t, err)
	require.Equal(t, "source", namespace)

	flatITS := its.DeepCopy()
	flatITS.Spec.FlatInstanceOrdinal = true
	reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, flatITS).Build()

	pvc.Name = "logs-cluster-mysql-1"
	pvc.Labels[constant.KBAppPodNameLabelKey] = "cluster-mysql-1"
	pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] = "logs"
	pvc.Labels[constant.KBAppInstanceTemplateLabelKey] = "special"
	namespace, err = reconciler.authorizedBackupNamespaceFromPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc)

	require.NoError(t, err)
	require.Equal(t, "source", namespace)
}

func TestHandleSyncPVCErrorKeepsInternalRequeueWhenPVCIsPopulating(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "target"},
		Status: corev1.PersistentVolumeClaimStatus{
			Conditions: []corev1.PersistentVolumeClaimCondition{{
				Type:   PersistentVolumeClaimPopulating,
				Status: corev1.ConditionTrue,
			}},
		},
	}
	reconciler := &VolumePopulatorReconciler{Recorder: record.NewFakeRecorder(10)}

	result, err := reconciler.handleSyncPVCError(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		pvc,
		intctrlutil.NewRequeueError(reconcileInterval, "waiting for postReady restore"),
	)

	require.NoError(t, err)
	require.Equal(t, reconcileInterval, result.RequeueAfter)
}

func TestDecidePVCRestoreAssignsShardingTargetsByStableComponentOrder(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	cluster := newClusterForShardingDecision(3)
	components := []client.Object{
		newComponentForShardingDecision("shard-c"),
		newComponentForShardingDecision("shard-a"),
		newComponentForShardingDecision("shard-b"),
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(append([]client.Object{cluster}, components...)...).
			Build(),
	}
	backup := newBackupForRestoreDecision([]string{"data"}, []string{"target-a", "target-b"})
	pvc := newPVCForRestoreDecision("data", "shard-b", "shard")

	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeRestoreData, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)
	require.Equal(t, "target-b", decision.sourceTarget.Name)

	pvc = newPVCForRestoreDecision("data", "shard-c", "shard")
	decision, err = reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.True(t, decision.skipPostReady)
	require.Nil(t, decision.sourceTarget)
}

func TestDecidePVCRestoreFailsWhenShardingTargetsExceedShards(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	cluster := newClusterForShardingDecision(2)
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()}
	backup := newBackupForRestoreDecision([]string{"data"}, []string{"target-a", "target-b", "target-c"})
	pvc := newPVCForRestoreDecision("data", "shard-a", "shard")

	_, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
}

func TestDecidePVCRestoreRequeuesWhenShardingComponentsIncomplete(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	cluster := newClusterForShardingDecision(3)
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(cluster, newComponentForShardingDecision("shard-a")).
			Build(),
	}
	backup := newBackupForRestoreDecision([]string{"data"}, []string{"target-a", "target-b"})
	pvc := newPVCForRestoreDecision("data", "shard-a", "shard")

	_, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err.Error())
}

func TestProvisionOnlyCreatesPopulatePVCWithoutJob(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "logs-target-0",
			UID:       "logs-target-0-uid",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     dptypes.BackupKind,
				Name:     "backup",
			},
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(pvc).WithObjects(pvc).Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreCtx := &pvcRestoreContext{
		mode: pvcRestoreModeProvisionOnly,
		restoreMgr: dprestore.NewRestoreManager(&dpv1alpha1.Restore{
			Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: "default"}},
		}, nil, scheme, reconciler.Client),
	}

	err := reconciler.ProvisionOnly(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, restoreCtx)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err.Error())
	populatePVC := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: getPopulatePVCName(pvc.UID)}, populatePVC))
	jobs := &batchv1.JobList{}
	require.NoError(t, reconciler.Client.List(context.Background(), jobs))
	require.Empty(t, jobs.Items)
}

func TestDispatchUnboundPVCFallsBackToProvisionOnlyWhenOnlyPostReadyExists(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-restore-qdrant-0",
			UID:       "data-restore-qdrant-0-uid",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
			},
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     dptypes.BackupKind,
				Name:     "backup",
			},
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(pvc).WithObjects(pvc).Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: "default"}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: &dpv1alpha1.Backup{}}}
	restoreCtx := &pvcRestoreContext{
		mode:       pvcRestoreModeRestoreData,
		restoreMgr: restoreMgr,
	}

	err := reconciler.dispatchUnboundPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, restoreCtx)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), "expected requeue from ProvisionOnly fallback, got: %v", err)
	populatePVC := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: getPopulatePVCName(pvc.UID)}, populatePVC))
	require.Empty(t, populatePVC.Spec.DataSourceRef, "populate PVC must not have dataSourceRef (ProvisionOnly path)")
}

func TestDispatchUnboundPVCFailsWhenNoRestoreActionsExist(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-restore-cluster-0",
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: "default"}},
	}, nil, scheme, reconciler.Client)
	restoreCtx := &pvcRestoreContext{
		mode:       pvcRestoreModeRestoreData,
		restoreMgr: restoreMgr,
	}

	err := reconciler.dispatchUnboundPVC(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, restoreCtx)

	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal),
		"expected fatal error when neither prepareData nor postReady exists, got: %v", err)
}

func TestBuildPostReadyRestoreSelectsHighestPriorityRole(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	compDef := &kbappsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
		Spec: kbappsv1.ComponentDefinitionSpec{
			Roles: []kbappsv1.ReplicaRole{
				{Name: "follower", UpdatePriority: 1},
				{Name: "leader", UpdatePriority: 10},
			},
		},
	}
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
		},
		Spec: kbappsv1.ComponentSpec{CompDef: compDef.Name},
	}
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{Name: backup.Name}
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, compDef).Build(),
		Scheme: scheme,
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{}, nil, scheme, reconciler.Client)

	restore, err := reconciler.buildPostReadyRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, restoreMgr, comp, "", nil)

	require.NoError(t, err)
	require.Equal(t, "leader", restore.Spec.ReadyConfig.JobAction.Target.PodSelector.LabelSelector.MatchLabels[instanceset.RoleLabelKey])
	require.NotContains(t, restore.Spec.ReadyConfig.ExecAction.Target.PodSelector.MatchLabels, instanceset.RoleLabelKey)
}

func TestBuildPostReadyRestoreUsesInitAccountFromComponentDefinition(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	compDef := &kbappsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
		Spec: kbappsv1.ComponentDefinitionSpec{
			SystemAccounts: []kbappsv1.SystemAccount{
				{Name: "app"},
				{Name: "root", InitAccount: true},
			},
		},
	}
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
		},
		Spec: kbappsv1.ComponentSpec{CompDef: compDef.Name},
	}
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{Name: backup.Name}
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, compDef).Build(),
		Scheme: scheme,
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{}, nil, scheme, reconciler.Client)

	restore, err := reconciler.buildPostReadyRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, restoreMgr, comp, "", nil)

	require.NoError(t, err)
	require.NotNil(t, restore.Spec.ReadyConfig.ConnectionCredential)
	require.Equal(t, constant.GenerateAccountSecretName("cluster", "mysql", "root"), restore.Spec.ReadyConfig.ConnectionCredential.SecretName)
}

func TestEnsurePostReadyRestoreCompletedDoesNotReuseStaleRestore(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.UID = types.UID("current-pvc")
	pvc.Spec.VolumeName = "target-pv"
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pvc.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       "component-uid",
		},
		Status: kbappsv1.ComponentStatus{
			Phase: kbappsv1.RunningComponentPhase,
		},
	}
	staleRestore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.ShortenKubeName("cluster-mysql-backup-post-ready", constant.KubeNameMaxLength),
		},
		Status: dpv1alpha1.RestoreStatus{
			Phase: dpv1alpha1.RestorePhaseCompleted,
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pvc).
			WithObjects(backup, pvc, comp, staleRestore).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	completed, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		pvc,
		&pvcRestoreContext{restoreMgr: restoreMgr, mode: pvcRestoreModeRestoreData},
	)

	require.NoError(t, err)
	require.False(t, completed)
	currentRestore := &dpv1alpha1.Restore{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{
		Namespace: pvc.Namespace,
		Name:      postReadyRestoreName(comp.UID),
	}, currentRestore))
	require.NotEqual(t, staleRestore.Name, currentRestore.Name)
	require.Equal(t, backup.Name, currentRestore.Spec.Backup.Name)
}

func TestEnsurePostReadyRestoreCompletedUsesOneRestorePerComponent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	pvc1 := newPVCForRestoreDecision("data", "mysql", "")
	pvc1.UID = types.UID("data-pvc")
	pvc1.Spec.VolumeName = "data-pv"
	pvc1.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pvc1.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc1.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	pvc2 := newPVCForRestoreDecision("logs", "mysql", "")
	pvc2.UID = types.UID("logs-pvc")
	pvc2.Spec.VolumeName = "logs-pv"
	pvc2.Spec.DataSourceRef = pvc1.Spec.DataSourceRef.DeepCopy()
	pvc2.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc2.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       "component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pvc1, pvc2).
			WithObjects(backup, pvc1, pvc2, comp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}
	restoreCtx := &pvcRestoreContext{restoreMgr: restoreMgr, mode: pvcRestoreModeRestoreData}

	completed, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pvc1, restoreCtx)
	require.NoError(t, err)
	require.False(t, completed)
	completed, err = reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pvc2, restoreCtx)
	require.NoError(t, err)
	require.False(t, completed)

	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 1)
	require.Equal(t, postReadyRestoreName(comp.UID), restoreList.Items[0].Name)
	require.Equal(t, backup.Name, restoreList.Items[0].Spec.Backup.Name)
}

func TestCompleteBoundPVCReleasesPopulatePVCBeforeWaitingForPostReady(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.UID = types.UID("target-pvc")
	pvc.Finalizers = []string{dptypes.DataProtectionFinalizerName}
	pvc.Spec.VolumeName = "data-pv"
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pvc.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	populatePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pvc.Namespace,
			Name:      getPopulatePVCName(pvc.UID),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "data-pv",
		},
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "data-pv"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			ClaimRef: &corev1.ObjectReference{
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
				UID:       pvc.UID,
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       "component-uid",
		},
		Status: kbappsv1.ComponentStatus{
			Phase: kbappsv1.CreatingComponentPhase,
			Conditions: []metav1.Condition{{
				Type:   kbappsv1.ComponentConditionProgressing,
				Status: metav1.ConditionTrue,
				Reason: "PostProvision",
			}},
		},
	}
	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "prepare-data-restore",
		},
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pvc, restore).
			WithObjects(backup, pvc, populatePVC, pv, comp, restore).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(restore, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	err := reconciler.completeBoundPVCIfNeeded(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		pvc,
		&pvcRestoreContext{restoreMgr: restoreMgr, mode: pvcRestoreModeRestoreData},
	)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	currentPVC := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(pvc), currentPVC))
	require.NotContains(t, currentPVC.Finalizers, dptypes.DataProtectionFinalizerName)
	require.Equal(t, corev1.ClaimBound, currentPVC.Status.Phase)
	require.Equal(t, resource.MustParse("1Gi"), currentPVC.Status.Capacity[corev1.ResourceStorage])
	require.Equal(t, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, currentPVC.Status.AccessModes)
	populatingCondition := findPVCConditionByType(currentPVC, string(PersistentVolumeClaimPopulating))
	require.NotNil(t, populatingCondition)
	require.Equal(t, ReasonPopulatingSucceed, populatingCondition.Reason)
	restoreCondition := findPVCConditionByType(currentPVC, kbappsv1.ConditionTypeRestore)
	require.Nil(t, restoreCondition)
	currentPopulatePVC := &corev1.PersistentVolumeClaim{}
	require.Error(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(populatePVC), currentPopulatePVC))
}

func TestCompleteBoundPVCContinuesPostReadyAfterPopulateReleased(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.UID = types.UID("target-pvc")
	pvc.Spec.VolumeName = "data-pv"
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pvc.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{
		{
			Type:   PersistentVolumeClaimPopulating,
			Status: corev1.ConditionTrue,
			Reason: ReasonPopulatingSucceed,
		},
	}
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       "component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pvc).
			WithObjects(backup, pvc, comp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	err := reconciler.completeBoundPVCIfNeeded(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		pvc,
		&pvcRestoreContext{restoreMgr: restoreMgr, mode: pvcRestoreModeRestoreData},
	)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 1)
	require.Equal(t, postReadyRestoreName(comp.UID), restoreList.Items[0].Name)
	currentPVC := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(pvc), currentPVC))
	populatingCondition := findPVCConditionByType(currentPVC, string(PersistentVolumeClaimPopulating))
	require.NotNil(t, populatingCondition)
	require.Equal(t, ReasonPopulatingSucceed, populatingCondition.Reason)
	restoreCondition := findPVCConditionByType(currentPVC, kbappsv1.ConditionTypeRestore)
	require.Nil(t, restoreCondition)
}

func TestCompleteBoundPVCMarksRestoreSucceededAfterPostReadyCompleted(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.UID = types.UID("target-pvc")
	pvc.Spec.VolumeName = "data-pv"
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pvc.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type:   PersistentVolumeClaimPopulating,
		Status: corev1.ConditionTrue,
		Reason: ReasonPopulatingSucceed,
	}}
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       "component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pvc).
			WithObjects(backup, pvc, comp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}
	postReadyRestore, err := reconciler.buildPostReadyRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		pvc,
		restoreMgr,
		comp,
		"",
		nil,
	)
	require.NoError(t, err)
	postReadyRestore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
	require.NoError(t, reconciler.Client.Create(context.Background(), postReadyRestore))

	err = reconciler.completeBoundPVCIfNeeded(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		pvc,
		&pvcRestoreContext{restoreMgr: restoreMgr, mode: pvcRestoreModeRestoreData},
	)

	require.NoError(t, err)
	currentPVC := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(pvc), currentPVC))
	populatingCondition := findPVCConditionByType(currentPVC, string(PersistentVolumeClaimPopulating))
	require.NotNil(t, populatingCondition)
	require.Equal(t, ReasonPopulatingSucceed, populatingCondition.Reason)
	restoreCondition := findPVCConditionByType(currentPVC, kbappsv1.ConditionTypeRestore)
	require.NotNil(t, restoreCondition)
	require.Equal(t, corev1.ConditionTrue, restoreCondition.Status)
	require.Equal(t, ReasonPopulatingSucceed, restoreCondition.Reason)
}

func TestEnsurePostReadyRestoreCompletedRejectsMismatchedExistingRestore(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.UID = types.UID("data-pvc")
	pvc.Spec.VolumeName = "data-pv"
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pvc.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       "component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	existing := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      postReadyRestoreName(comp.UID),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps.kubeblocks.io/v1",
				Kind:       "Component",
				Name:       comp.Name,
				UID:        comp.UID,
			}},
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: "other"},
		},
		Status: dpv1alpha1.RestoreStatus{Phase: dpv1alpha1.RestorePhaseCompleted},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pvc).
			WithObjects(backup, pvc, comp, existing).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	completed, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()},
		pvc,
		&pvcRestoreContext{restoreMgr: restoreMgr, mode: pvcRestoreModeRestoreData},
	)

	require.Error(t, err)
	require.False(t, completed)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
}

func TestRestoreParametersToPairsSortsKeys(t *testing.T) {
	pairs := restoreParametersToPairs(map[string]string{
		"mysql.kubeblocks.io/skip-binlog":       "true",
		"dataprotection.kubeblocks.io/parallel": "4",
		"apps.kubeblocks.io/foo":                "bar",
	})

	require.Equal(t, []dpv1alpha1.ParameterPair{
		{Name: "apps.kubeblocks.io/foo", Value: "bar"},
		{Name: "dataprotection.kubeblocks.io/parallel", Value: "4"},
		{Name: "mysql.kubeblocks.io/skip-binlog", Value: "true"},
	}, pairs)
}

func TestEnsureInternalRestoreRejectsSpecMutation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-mysql-0",
			UID:       "pvc-uid",
		},
	}
	existing := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      getPopulatePVCName(pvc.UID),
		},
		Spec: dpv1alpha1.RestoreSpec{
			Backup:      dpv1alpha1.BackupRef{Name: "backup", Namespace: "default"},
			RestoreTime: "2026-05-01T00:00:00Z",
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build(),
		Scheme: scheme,
	}
	desired := existing.DeepCopy()
	desired.Spec.RestoreTime = "2026-05-02T00:00:00Z"

	restore, err := reconciler.ensureInternalRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, desired)

	require.Error(t, err)
	require.Nil(t, restore)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
}

func TestWaitForSerialPredecessorsWaitsForEarlierUnboundPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	previous := newRestorePVCForSerialTest("data-target-0", "")
	current := newRestorePVCForSerialTest("data-target-1", "")
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	reconciler := &VolumePopulatorReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(previous, current, backup).Build(),
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
			},
		},
	}, nil, scheme, reconciler.Client)
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(current), current))
	current.Spec.DataSourceRef.APIGroup = &apiGroup

	err := reconciler.waitForSerialPredecessors(intctrlutil.RequestCtx{Ctx: context.Background()}, current, restoreMgr)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err.Error())
}

func TestWaitForSerialPredecessorsAllowsAfterEarlierBoundPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	previous := newRestorePVCForSerialTest("data-target-0", "pv-0")
	current := newRestorePVCForSerialTest("data-target-1", "")
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(previous, current, backup).Build()}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
			},
		},
	}, nil, scheme, reconciler.Client)

	err := reconciler.waitForSerialPredecessors(intctrlutil.RequestCtx{Ctx: context.Background()}, current, restoreMgr)

	require.NoError(t, err)
}

func TestWaitForSerialPredecessorsSkipsProvisionOnlyPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	previous := newRestorePVCForSerialTest("logs-target-0", "")
	previous.Labels[constant.VolumeClaimTemplateNameLabelKey] = "logs"
	previous.Annotations[constant.RestoreVolumeTemplateAnnotationKey] = "logs"
	current := newRestorePVCForSerialTest("data-target-1", "")
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(previous, current, backup).Build()}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
			},
		},
	}, nil, scheme, reconciler.Client)

	err := reconciler.waitForSerialPredecessors(intctrlutil.RequestCtx{Ctx: context.Background()}, current, restoreMgr)

	require.NoError(t, err)
}

func newRestorePVCForSerialTest(name, volumeName string) *corev1.PersistentVolumeClaim {
	apiGroup := dptypes.DataprotectionAPIGroup
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			UID:       types.UID(name + "-uid"),
			Labels: map[string]string{
				constant.AppManagedByLabelKey:            constant.AppName,
				constant.AppInstanceLabelKey:             "cluster",
				constant.KBAppComponentLabelKey:          "mysql",
				constant.KBAppPodNameLabelKey:            name,
				constant.VolumeClaimTemplateNameLabelKey: "data",
			},
			Annotations: map[string]string{
				constant.RestoreSourceKindAnnotationKey:      dptypes.BackupKind,
				constant.RestoreSourceNamespaceAnnotationKey: "default",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: volumeName,
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     dptypes.BackupKind,
				Name:     "backup",
			},
		},
	}
}

func newBackupForRestoreDecision(targetVolumes []string, targets []string) *dpv1alpha1.Backup {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backup",
		},
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{},
			Target: &dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{Name: "target"},
			},
		},
	}
	if targetVolumes != nil {
		backup.Status.BackupMethod.TargetVolumes = &dpv1alpha1.TargetVolumeInfo{Volumes: targetVolumes}
	}
	if targets != nil {
		backup.Status.Target = nil
		for _, target := range targets {
			backup.Status.Targets = append(backup.Status.Targets, dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{Name: target},
			})
		}
	}
	return backup
}

func newPVCForRestoreDecision(volumeName, componentName, shardingName string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      volumeName + "-" + componentName + "-0",
			Labels: map[string]string{
				constant.AppManagedByLabelKey:            constant.AppName,
				constant.AppInstanceLabelKey:             "cluster",
				constant.KBAppComponentLabelKey:          componentName,
				constant.VolumeClaimTemplateNameLabelKey: volumeName,
			},
			Annotations: map[string]string{
				constant.RestoreVolumeTemplateAnnotationKey: volumeName,
			},
		},
	}
	if shardingName != "" {
		pvc.Labels[constant.KBAppShardingNameLabelKey] = shardingName
	}
	return pvc
}

func newClusterForShardingDecision(shards int32) *kbappsv1.Cluster {
	return &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster",
		},
		Spec: kbappsv1.ClusterSpec{
			Shardings: []kbappsv1.ClusterSharding{{
				Name:   "shard",
				Shards: shards,
			}},
		},
	}
}

func newComponentForShardingDecision(componentName string) *kbappsv1.Component {
	return &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster-" + componentName,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:      constant.AppName,
				constant.AppInstanceLabelKey:       "cluster",
				constant.KBAppComponentLabelKey:    componentName,
				constant.KBAppShardingNameLabelKey: "shard",
			},
		},
	}
}

func TestRebindPVCAndPVWaitsUntilPopulatePVCIsBound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "default",
			Name:            "data-target-0",
			UID:             "target-pvc-uid",
			ResourceVersion: "1",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: "backup"},
		},
	}
	populatePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "populate",
		},
	}
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	rebound, err := reconciler.rebindPVCAndPV(reqCtx, populatePVC, pvc)
	require.NoError(t, err)
	require.False(t, rebound)

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv"}}
	reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv, pvc).Build()
	populatePVC.Spec.VolumeName = pv.Name

	rebound, err = reconciler.rebindPVCAndPV(reqCtx, populatePVC, pvc)
	require.NoError(t, err)
	require.True(t, rebound)

	patchedPV := &corev1.PersistentVolume{}
	require.NoError(t, reconciler.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: pv.Name}, patchedPV))
	require.NotNil(t, patchedPV.Spec.ClaimRef)
	require.Equal(t, pvc.Name, patchedPV.Spec.ClaimRef.Name)
	require.Equal(t, pvc.UID, patchedPV.Spec.ClaimRef.UID)

	patchedPVC := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvc), patchedPVC))
	require.Equal(t, pv.Name, patchedPVC.Spec.VolumeName)
}

func TestRestoreSystemAccountSecretsUsesShardingSecretName(t *testing.T) {
	require.Equal(t, "cluster-shard-admin", systemAccountSecretName(systemAccountSecretScopeSharding, "cluster", "shard", "admin"))
	require.Equal(t, constant.GenerateAccountSecretName("cluster", "mysql", "admin"),
		systemAccountSecretName(systemAccountSecretScopeComponent, "cluster", "mysql", "admin"))
}

func TestRestoreSystemAccountSecretsRestoresComponentAndShardingSecrets(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))

	encryptor := intctrlutil.NewEncryptor("")
	componentPassword, err := encryptor.Encrypt([]byte("component-password"))
	require.NoError(t, err)
	shardingPassword, err := encryptor.Encrypt([]byte("sharding-password"))
	require.NoError(t, err)
	accounts, err := json.Marshal(map[string]map[string]string{
		"mysql": {
			"admin": componentPassword,
		},
		"shard": {
			"root": shardingPassword,
		},
	})
	require.NoError(t, err)
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backup",
			Annotations: map[string]string{
				constant.EncryptedSystemAccountsAnnotationKey: string(accounts),
			},
		},
	}
	cluster := &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster",
			UID:       types.UID("cluster-uid"),
		},
	}
	component := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       types.UID("component-uid"),
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:       "cluster",
				constant.KBAppComponentLabelKey:    "mysql",
				constant.KBAppShardingNameLabelKey: "shard",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: backup.Name},
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, cluster, component).Build(),
		Scheme: scheme,
	}

	err = reconciler.restoreSystemAccountSecrets(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup.Namespace)
	require.NoError(t, err)

	componentSecret := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      constant.GenerateAccountSecretName("cluster", "mysql", "admin"),
	}, componentSecret))
	require.Equal(t, []byte("component-password"), componentSecret.Data[constant.AccountPasswdForSecret])
	require.Len(t, componentSecret.OwnerReferences, 1)
	require.Equal(t, "Component", componentSecret.OwnerReferences[0].Kind)
	require.Equal(t, component.Name, componentSecret.OwnerReferences[0].Name)

	shardingSecret := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "cluster-shard-root",
	}, shardingSecret))
	require.Equal(t, []byte("sharding-password"), shardingSecret.Data[constant.AccountPasswdForSecret])
	require.Len(t, shardingSecret.OwnerReferences, 1)
	require.Equal(t, "Cluster", shardingSecret.OwnerReferences[0].Kind)
	require.Equal(t, cluster.Name, shardingSecret.OwnerReferences[0].Name)
}

func TestRestoreSystemAccountSecretsReturnsFatalForInvalidAccountsPayload(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backup",
			Annotations: map[string]string{
				constant.EncryptedSystemAccountsAnnotationKey: "{",
			},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "cluster",
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: backup.Name},
		},
	}
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup).Build()}

	err := reconciler.restoreSystemAccountSecrets(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup.Namespace)
	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal))
}

// TiDB-shaped multi-component test: backup target (tidb) has no data PVC,
// data PVCs belong to other components (pd, tikv).
// ActionSet has only postReady (no prepareData) — logical backup pattern.

func TestDecidePVCRestore_MultiComponent_PostReadyOnly_SkipsNonMatchingPVC(t *testing.T) {
	// Documents current behavior (the bug): when backup target is scoped to
	// component "tidb" and a PVC belongs to component "pd", skipPostReady=true.
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision(nil, nil) // no targetVolumes (logical backup)
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Target = &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: "tidb",
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey:    "cluster",
						constant.KBAppComponentLabelKey: "tidb",
					},
				},
			},
		},
	}

	// PD data PVC — belongs to "pd" component, not "tidb"
	pdPVC := newPVCForRestoreDecision("data", "pd", "")
	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	// Current behavior: skipPostReady=true for non-matching component
	require.True(t, decision.skipPostReady, "current behavior: non-matching PVC gets skipPostReady=true")
	require.Nil(t, decision.sourceTarget)

	// TiKV data PVC — also not "tidb"
	tikvPVC := newPVCForRestoreDecision("data", "tikv", "")
	decision, err = reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, tikvPVC, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.True(t, decision.skipPostReady, "current behavior: non-matching PVC gets skipPostReady=true")
	require.Nil(t, decision.sourceTarget)
}

func TestEnsurePostReadyRestore_MultiComponent_PostReadyOnly_ShouldNotSilentlySkip(t *testing.T) {
	// Desired behavior: for a TiDB-shaped multi-component logical backup restore
	// (postReady-only ActionSet, backup target scoped to "tidb", data PVCs in pd/tikv),
	// the full production decision→postReady path should produce exactly 1 Restore CR
	// anchored to the backup target component context (tidb), not silently skip all.
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup

	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Target = &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: "tidb",
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey:    "cluster",
						constant.KBAppComponentLabelKey: "tidb",
					},
				},
			},
		},
	}

	// PD data PVC
	pdPVC := newPVCForRestoreDecision("data", "pd", "")
	pdPVC.UID = types.UID("pd-pvc-uid")
	pdPVC.Spec.VolumeName = "pd-data-pv"
	pdPVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pdPVC.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pdPVC.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	// TiKV data PVC
	tikvPVC := newPVCForRestoreDecision("data", "tikv", "")
	tikvPVC.UID = types.UID("tikv-pvc-uid")
	tikvPVC.Spec.VolumeName = "tikv-data-pv"
	tikvPVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	tikvPVC.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	tikvPVC.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	// All three components: pd, tikv, tidb (backup target)
	pdComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "pd"),
			UID:       "pd-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	tikvComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "tikv"),
			UID:       "tikv-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	tidbComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "tidb"),
			UID:       "tidb-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}

	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pdPVC, tikvPVC).
			WithObjects(backup, pdPVC, tikvPVC, pdComp, tikvComp, tidbComp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	// Step 1: derive context from the production decision path (not hardcoded)
	pdDecision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, backup, nil)
	require.NoError(t, err)
	tikvDecision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, tikvPVC, backup, nil)
	require.NoError(t, err)

	pdCtx := &pvcRestoreContext{
		restoreMgr:    restoreMgr,
		mode:          pdDecision.mode,
		skipPostReady: pdDecision.skipPostReady,
	}
	tikvCtx := &pvcRestoreContext{
		restoreMgr:    restoreMgr,
		mode:          tikvDecision.mode,
		skipPostReady: tikvDecision.skipPostReady,
	}

	// Step 2: process both PVCs through ensurePostReadyRestoreCompleted
	completed1, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, pdCtx)
	require.NoError(t, err)
	completed2, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, tikvPVC, tikvCtx)
	require.NoError(t, err)

	// Step 3: assert exactly 1 Restore CR
	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 1,
		"postReady-only ActionSet with multi-component backup should create exactly 1 Restore CR, "+
			"not silently skip all non-matching PVCs (got %d)", len(restoreList.Items))

	// Step 4: assert the Restore CR is anchored to the backup target component (tidb)
	restore := restoreList.Items[0]

	// Owner must be the tidb component (backup target), not pd or tikv
	require.Len(t, restore.OwnerReferences, 1, "Restore CR should have exactly 1 owner reference")
	require.Equal(t, tidbComp.UID, restore.OwnerReferences[0].UID,
		"Restore CR owner should be the backup target component (tidb), not a random data component")
	require.Equal(t, tidbComp.Name, restore.OwnerReferences[0].Name)

	// ReadyConfig pod selectors must target tidb component pods
	require.NotNil(t, restore.Spec.ReadyConfig)
	require.NotNil(t, restore.Spec.ReadyConfig.ExecAction)
	require.Equal(t, "tidb",
		restore.Spec.ReadyConfig.ExecAction.Target.PodSelector.MatchLabels[constant.KBAppComponentLabelKey],
		"ExecAction should target tidb component pods (where BR tool + PD_ADDRESS env are)")
	require.NotNil(t, restore.Spec.ReadyConfig.JobAction)
	require.Equal(t, "tidb",
		restore.Spec.ReadyConfig.JobAction.Target.PodSelector.LabelSelector.MatchLabels[constant.KBAppComponentLabelKey],
		"JobAction should target tidb component pods")

	// Dedup: Restore name should be deterministic (per tidb component UID, not per data PVC)
	require.Equal(t, postReadyRestoreName(tidbComp.UID), restore.Name,
		"Restore CR name should be keyed on backup target component UID for deterministic dedup")

	// Backup source target should reference the backup target
	require.Equal(t, "tidb", restore.Spec.Backup.SourceTargetName,
		"Restore CR should reference backup target name")

	// Both PVCs should not both report completed=true when postReady is in-progress
	require.False(t, completed1 && completed2,
		"at least one PVC should wait for postReady restore, not all silently skip")
}

func TestEnsurePostReadyRestore_MultiComponent_PrepareDataAndPostReady_RedirectsPostReady(t *testing.T) {
	// TiDB PITR shape: data PVCs belong to non-target components, while the
	// backup target component owns the postReady replay action. The presence of
	// prepareData must not make skipPostReady treat postReady as already done.
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup

	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	backup.Status.Target = &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: "tidb",
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey:    "cluster",
						constant.KBAppComponentLabelKey: "tidb",
					},
				},
			},
		},
	}

	pdPVC := newPVCForRestoreDecision("data", "pd", "")
	pdPVC.UID = types.UID("pd-pvc-uid")
	pdPVC.Spec.VolumeName = "pd-data-pv"
	pdPVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pdPVC.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pdPVC.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	pdComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "pd"),
			UID:       "pd-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	tidbComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "tidb"),
			UID:       "tidb-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}

	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pdPVC).
			WithObjects(backup, pdPVC, pdComp, tidbComp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PrepareDataBackupSets = []dprestore.BackupActionSet{{Backup: backup}}
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	pdDecision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, backup, nil)
	require.NoError(t, err)
	require.True(t, pdDecision.skipPostReady)

	completed, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, &pvcRestoreContext{
			restoreMgr:    restoreMgr,
			mode:          pdDecision.mode,
			skipPostReady: pdDecision.skipPostReady,
		})
	require.NoError(t, err)
	require.False(t, completed, "postReady restore should be in progress, not silently completed")

	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 1,
		"prepareData+postReady with non-matching PVC must still create target-component postReady Restore CR")

	restore := restoreList.Items[0]
	require.Equal(t, tidbComp.UID, restore.OwnerReferences[0].UID)
	require.Equal(t, "tidb", restore.Spec.Backup.SourceTargetName)
	require.Equal(t, postReadyRestoreName(tidbComp.UID), restore.Name)
}

func TestEnsurePostReadyRestore_ShardingMissingTargetSkip_DoesNotRedirect(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup

	cluster := newClusterForShardingDecision(3)
	components := []client.Object{
		newComponentForShardingDecision("shard-c"),
		newComponentForShardingDecision("shard-a"),
		newComponentForShardingDecision("shard-b"),
	}
	backup := newBackupForRestoreDecision(nil, []string{"target-a", "target-b"})
	backup.Status.BackupMethod.TargetVolumes = nil

	pvc := newPVCForRestoreDecision("data", "shard-c", "shard")
	pvc.UID = types.UID("shard-c-pvc-uid")
	pvc.Spec.VolumeName = "shard-c-data-pv"
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pvc.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	objects := append([]client.Object{cluster, backup, pvc}, components...)
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pvc).
			WithObjects(objects...).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	decision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.True(t, decision.skipPostReady)
	require.Nil(t, decision.sourceTarget)

	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	completed, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, &pvcRestoreContext{
			restoreMgr:    restoreMgr,
			mode:          decision.mode,
			skipPostReady: decision.skipPostReady,
		})
	require.NoError(t, err)
	require.True(t, completed, "sharding PVC without source target should preserve skip-only behavior")

	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 0,
		"sharding missing-target skip must not create redirected postReady Restore CR")
}

func TestEnsurePostReadyRestore_ShardingSingleTargetMissingTargetSkip_DoesNotRedirect(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup

	cluster := newClusterForShardingDecision(3)
	components := []client.Object{
		newComponentForShardingDecision("shard-c"),
		newComponentForShardingDecision("shard-a"),
		newComponentForShardingDecision("shard-b"),
	}
	backup := newBackupForRestoreDecision(nil, []string{"target-a"})
	backup.Status.BackupMethod.TargetVolumes = nil

	pvc := newPVCForRestoreDecision("data", "shard-b", "shard")
	pvc.UID = types.UID("shard-b-pvc-uid")
	pvc.Spec.VolumeName = "shard-b-data-pv"
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pvc.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	objects := append([]client.Object{cluster, backup, pvc}, components...)
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pvc).
			WithObjects(objects...).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	decision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)
	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.True(t, decision.skipPostReady)
	require.Nil(t, decision.sourceTarget)

	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	completed, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, &pvcRestoreContext{
			restoreMgr:    restoreMgr,
			mode:          decision.mode,
			skipPostReady: decision.skipPostReady,
		})
	require.NoError(t, err)
	require.True(t, completed, "single-target sharding PVC without source target should preserve skip-only behavior")

	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 0,
		"single-target sharding missing-target skip must not create redirected postReady Restore CR")
}

func TestEnsurePostReadyRestore_MultiComponent_PostReadyRedirectPreservesTargetExecutionFields(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	targetMounts := []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}

	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = &dpv1alpha1.TargetVolumeInfo{VolumeMounts: targetMounts}
	backup.Status.Target = &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: "tidb",
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey:    "cluster",
						constant.KBAppComponentLabelKey: "tidb",
					},
				},
				Strategy: dpv1alpha1.PodSelectionStrategyAll,
			},
		},
	}

	pdPVC := newPVCForRestoreDecision("data", "pd", "")
	pdPVC.UID = types.UID("pd-pvc-uid")
	pdPVC.Spec.VolumeName = "pd-data-pv"
	pdPVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pdPVC.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pdPVC.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	pdComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "pd"),
			UID:       "pd-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	tidbComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "tidb"),
			UID:       "tidb-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}

	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pdPVC).
			WithObjects(backup, pdPVC, pdComp, tidbComp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	decision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, backup, nil)
	require.NoError(t, err)
	require.True(t, decision.skipPostReady)

	_, err = reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, &pvcRestoreContext{
			restoreMgr:    restoreMgr,
			mode:          decision.mode,
			skipPostReady: decision.skipPostReady,
		})
	require.NoError(t, err)

	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 1)

	restore := restoreList.Items[0]
	require.Equal(t, tidbComp.UID, restore.OwnerReferences[0].UID)
	require.Equal(t, "tidb", restore.Spec.Backup.SourceTargetName)
	require.NotNil(t, restore.Spec.ReadyConfig.JobAction)
	require.Equal(t, dpv1alpha1.PodSelectionStrategyAll,
		restore.Spec.ReadyConfig.JobAction.Target.PodSelector.Strategy)
	require.Equal(t, targetMounts, restore.Spec.ReadyConfig.JobAction.Target.VolumeMounts)
	require.Equal(t, &dpv1alpha1.RequiredPolicyForAllPodSelection{
		DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy,
	}, restore.Spec.ReadyConfig.JobAction.RequiredPolicyForAllPodSelection)
}

func TestRebindPVCAndPV_NilPopulatePVC_ReturnsFatalError(t *testing.T) {
	// When rebindPVCAndPV is called with nil populatePVC (restoreData path entered
	// without prepareData backup set), it must return a fatal error — not panic,
	// and not silently return (false, nil) which would cause infinite requeue.
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "default",
			Name:            "data-target-0",
			UID:             "target-pvc-uid",
			ResourceVersion: "1",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: "backup"},
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build(),
	}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	require.NotPanics(t, func() {
		rebound, err := reconciler.rebindPVCAndPV(reqCtx, nil, pvc)
		require.Error(t, err, "nil populatePVC should return error, not silently succeed")
		require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal),
			"nil populatePVC is an invalid contract state, should be fatal error: %v", err)
		require.False(t, rebound)
	}, "rebindPVCAndPV should not panic when populatePVC is nil")
}

func TestEnsurePostReadyRestore_MultiComponent_PostReadyOnly_TargetComponentNotYetCreated(t *testing.T) {
	// When the backup target component (tidb) hasn't been created yet (multi-component
	// sequential creation: PD→TiKV→TiDB), the redirect path should return (false,nil)
	// for standard requeue — no Reconciler error, no Restore CR created.
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup

	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Target = &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: "tidb",
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey:    "cluster",
						constant.KBAppComponentLabelKey: "tidb",
					},
				},
			},
		},
	}

	pdPVC := newPVCForRestoreDecision("data", "pd", "")
	pdPVC.UID = types.UID("pd-pvc-uid")
	pdPVC.Spec.VolumeName = "pd-data-pv"
	pdPVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pdPVC.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pdPVC.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	// Only PD and TiKV components exist — tidb NOT yet created (sequential creation)
	pdComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "pd"),
			UID:       "pd-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	tikvComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "tikv"),
			UID:       "tikv-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	// NOTE: no tidb Component — simulates sequential creation where tidb doesn't exist yet

	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pdPVC).
			WithObjects(backup, pdPVC, pdComp, tikvComp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	pdDecision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, backup, nil)
	require.NoError(t, err)
	require.True(t, pdDecision.skipPostReady, "PD PVC should have skipPostReady=true")

	pdCtx := &pvcRestoreContext{
		restoreMgr:    restoreMgr,
		mode:          pdDecision.mode,
		skipPostReady: pdDecision.skipPostReady,
	}

	completed, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, pdCtx)
	require.NoError(t, err, "target Component not yet created should requeue without error, not spam Reconciler error")
	require.False(t, completed, "should not be completed when target component doesn't exist")

	// No Restore CR should be created
	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 0, "no Restore CR should be created while target component doesn't exist")
}

func TestEnsurePostReadyRestore_MultiComponent_PostReadyOnly_WaitsForAllComponentPVCs(t *testing.T) {
	// When PD PVCs are bound but TiKV PVCs are NOT yet bound, the redirect path must
	// wait (return false, nil) instead of creating the postReady Restore CR early.
	// This prevents the logical restore job from running before the cluster is ready.
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup

	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Target = &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: "tidb",
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey:    "cluster",
						constant.KBAppComponentLabelKey: "tidb",
					},
				},
			},
		},
	}

	// PD data PVC — bound (has VolumeName)
	pdPVC := newPVCForRestoreDecision("data", "pd", "")
	pdPVC.UID = types.UID("pd-pvc-uid")
	pdPVC.Spec.VolumeName = "pd-data-pv"
	pdPVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pdPVC.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pdPVC.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	// TiKV data PVC — NOT bound (no VolumeName)
	tikvPVC := newPVCForRestoreDecision("data", "tikv", "")
	tikvPVC.UID = types.UID("tikv-pvc-uid")
	tikvPVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	tikvPVC.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	tikvPVC.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	pdComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "pd"),
			UID:       "pd-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	tikvComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "tikv"),
			UID:       "tikv-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.CreatingComponentPhase},
	}
	tidbComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "tidb"),
			UID:       "tidb-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}

	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pdPVC, tikvPVC).
			WithObjects(backup, pdPVC, tikvPVC, pdComp, tikvComp, tidbComp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	pdDecision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, backup, nil)
	require.NoError(t, err)
	require.True(t, pdDecision.skipPostReady, "PD PVC should have skipPostReady=true")

	pdCtx := &pvcRestoreContext{
		restoreMgr:    restoreMgr,
		mode:          pdDecision.mode,
		skipPostReady: pdDecision.skipPostReady,
	}

	// PD PVC triggers redirect, but TiKV PVC is not bound yet — should wait
	completed, err := reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, pdCtx)
	require.NoError(t, err, "should requeue without error while TiKV PVCs are still pending")
	require.False(t, completed, "should not be completed while TiKV PVCs are unbound")

	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 0,
		"no Restore CR should be created while other component PVCs are still unbound")
}

func TestEnsurePostReadyRestore_MultiComponent_PostReadyOnly_TargetsSlice(t *testing.T) {
	// Regression: backup target in Status.Targets[0] (not Status.Target) must also
	// trigger the postReady-only redirect. This mirrors resolveSourceTargetFromBackup
	// which handles both Status.Target and len(Status.Targets)==1.
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup

	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	backup.Status.Target = nil
	backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: "tidb",
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey:    "cluster",
						constant.KBAppComponentLabelKey: "tidb",
					},
				},
			},
		},
	}}

	pdPVC := newPVCForRestoreDecision("data", "pd", "")
	pdPVC.UID = types.UID("pd-pvc-uid")
	pdPVC.Spec.VolumeName = "pd-data-pv"
	pdPVC.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     backup.Name,
	}
	pdPVC.Annotations[constant.RestoreSourceKindAnnotationKey] = dptypes.BackupKind
	pdPVC.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace

	pdComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "pd"),
			UID:       "pd-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}
	tidbComp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "tidb"),
			UID:       "tidb-component-uid",
		},
		Status: kbappsv1.ComponentStatus{Phase: kbappsv1.RunningComponentPhase},
	}

	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithStatusSubresource(pdPVC).
			WithObjects(backup, pdPVC, pdComp, tidbComp).
			Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}

	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: backup.Name, Namespace: backup.Namespace}},
	}, nil, scheme, reconciler.Client)
	restoreMgr.PostReadyBackupSets = []dprestore.BackupActionSet{{Backup: backup}}

	pdDecision, err := reconciler.decidePVCRestore(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, backup, nil)
	require.NoError(t, err)

	pdCtx := &pvcRestoreContext{
		restoreMgr:    restoreMgr,
		mode:          pdDecision.mode,
		skipPostReady: pdDecision.skipPostReady,
	}

	_, err = reconciler.ensurePostReadyRestoreCompleted(
		intctrlutil.RequestCtx{Ctx: context.Background()}, pdPVC, pdCtx)
	require.NoError(t, err)

	restoreList := &dpv1alpha1.RestoreList{}
	require.NoError(t, reconciler.Client.List(context.Background(), restoreList, client.InNamespace("default")))
	require.Len(t, restoreList.Items, 1,
		"postReady-only with Status.Targets[0] should create exactly 1 Restore CR")

	restore := restoreList.Items[0]
	require.Equal(t, tidbComp.UID, restore.OwnerReferences[0].UID,
		"Restore CR owner should be the backup target component (tidb) even when target is in Status.Targets[0]")
	require.Equal(t, "tidb", restore.Spec.Backup.SourceTargetName)
	require.Equal(t, postReadyRestoreName(tidbComp.UID), restore.Name)
}
