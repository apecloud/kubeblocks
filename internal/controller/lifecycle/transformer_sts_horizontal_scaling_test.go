/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("sts horizontal scaling test", func() {
	// TODO: refactor the following ut
	//
	//	ctx := context.Background()
	//	newReqCtx := func() intctrlutil.RequestCtx {
	//		reqCtx := intctrlutil.RequestCtx{
	//			Ctx:      ctx,
	//			Log:      logger,
	//			Recorder: clusterRecorder,
	//		}
	//		return reqCtx
	//	}
	//
	//	newVolumeSnapshot := func(clusterName, componentName string) *snapshotv1.VolumeSnapshot {
	//		vsYAML := `
	//
	// apiVersion: snapshot.storage.k8s.io/v1
	// kind: VolumeSnapshot
	// metadata:
	//
	//	labels:
	//	  app.kubernetes.io/name: mysql-apecloud-mysql
	//	  backupjobs.dataprotection.kubeblocks.io/name: wesql-01-replicasets-scaling-qf6cr
	//	  backuppolicies.dataprotection.kubeblocks.io/name: wesql-01-replicasets-scaling-hcxps
	//	  dataprotection.kubeblocks.io/backup-type: snapshot
	//	name: test-volume-snapshot
	//	namespace: default
	//
	// spec:
	//
	//	source:
	//	  persistentVolumeClaimName: data-wesql-01-replicasets-0
	//	volumeSnapshotClassName: csi-aws-ebs-snapclass
	//
	// `
	//
	//		vs := snapshotv1.VolumeSnapshot{}
	//		Expect(yaml.Unmarshal([]byte(vsYAML), &vs)).ShouldNot(HaveOccurred())
	//		labels := map[string]string{
	//			constant.KBManagedByKey:         "cluster",
	//			constant.AppInstanceLabelKey:    clusterName,
	//			constant.KBAppComponentLabelKey: componentName,
	//		}
	//		for k, v := range labels {
	//			vs.Labels[k] = v
	//		}
	//		return &vs
	//	}
	//
	//	Context("with HorizontalScalePolicy set to CloneFromSnapshot and VolumeSnapshot exists", func() {
	//		It("determines return value of doBackup according to whether VolumeSnapshot is ReadyToUse", func() {
	//			By("prepare cluster and construct component")
	//			reqCtx := newReqCtx()
	//			cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
	//			component := component.BuildComponent(
	//				reqCtx,
	//				*cluster,
	//				*clusterDef,
	//				clusterDef.Spec.ComponentDefs[0],
	//				cluster.Spec.ComponentSpecs[0],
	//				&clusterVersion.Spec.ComponentVersions[0])
	//			Expect(component).ShouldNot(BeNil())
	//			component.HorizontalScalePolicy = &appsv1alpha1.HorizontalScalePolicy{
	//				Type:             appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
	//				VolumeMountsName: "data",
	//			}
	//
	//			By("prepare VolumeSnapshot and set ReadyToUse to true")
	//			vs := newVolumeSnapshot(cluster.Name, mysqlCompName)
	//			Expect(testCtx.CreateObj(ctx, vs)).Should(Succeed())
	//			Expect(testapps.ChangeObjStatus(&testCtx, vs, func() {
	//				t := true
	//				vs.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: &t}
	//			})).Should(Succeed())
	//
	//			// prepare doBackup input parameters
	//			snapshotKey := types.NamespacedName{
	//				Namespace: "default",
	//				Name:      "test-snapshot",
	//			}
	//			sts := newStsObj()
	//			stsProto := *sts.DeepCopy()
	//			r := int32(3)
	//			stsProto.Spec.Replicas = &r
	//
	//			By("doBackup should return requeue=false")
	//			shouldRequeue, err := doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
	//			Expect(err).ShouldNot(HaveOccurred())
	//			Expect(shouldRequeue).Should(BeFalse())
	//
	//			By("Set ReadyToUse to nil, doBackup should return requeue=true")
	//			Expect(testapps.ChangeObjStatus(&testCtx, vs, func() {
	//				vs.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: nil}
	//			})).Should(Succeed())
	//			shouldRequeue, err = doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
	//			Expect(err).ShouldNot(HaveOccurred())
	//			Expect(shouldRequeue).Should(BeTrue())
	//		})
	//
	//		// REIVEW: this test seems always failed
	//		It("should do backup to create volumesnapshot when there exists a deleting volumesnapshot", func() {
	//			By("prepare cluster and construct component")
	//			reqCtx := newReqCtx()
	//			cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
	//			component := component.BuildComponent(
	//				reqCtx,
	//				*cluster,
	//				*clusterDef,
	//				clusterDef.Spec.ComponentDefs[0],
	//				cluster.Spec.ComponentSpecs[0],
	//				&clusterVersion.Spec.ComponentVersions[0])
	//			Expect(component).ShouldNot(BeNil())
	//			component.HorizontalScalePolicy = &appsv1alpha1.HorizontalScalePolicy{
	//				Type:             appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
	//				VolumeMountsName: "data",
	//			}
	//
	//			By("prepare VolumeSnapshot and set finalizer to prevent it from deletion")
	//			vs := newVolumeSnapshot(cluster.Name, mysqlCompName)
	//			Expect(testCtx.CreateObj(ctx, vs)).Should(Succeed())
	//			Expect(testapps.ChangeObj(&testCtx, vs, func() {
	//				vs.Finalizers = append(vs.Finalizers, "test-finalizer")
	//			})).Should(Succeed())
	//
	//			By("deleting volume snapshot")
	//			Expect(k8sClient.Delete(ctx, vs)).Should(Succeed())
	//
	//			By("checking DeletionTimestamp exists")
	//			Eventually(func(g Gomega) {
	//				tmpVS := snapshotv1.VolumeSnapshot{}
	//				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: vs.Namespace, Name: vs.Name}, &tmpVS)).Should(Succeed())
	//				g.Expect(tmpVS.DeletionTimestamp).ShouldNot(BeNil())
	//			}).Should(Succeed())
	//
	//			// prepare doBackup input parameters
	//			snapshotKey := types.NamespacedName{
	//				Namespace: "default",
	//				Name:      "test-snapshot",
	//			}
	//			sts := newStsObj()
	//			stsProto := *sts.DeepCopy()
	//			r := int32(3)
	//			stsProto.Spec.Replicas = &r
	//
	//			By("doBackup should create volumesnapshot and return requeue=true")
	//			shouldRequeue, err := doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
	//			Expect(err).ShouldNot(HaveOccurred())
	//			Expect(shouldRequeue).Should(BeTrue())
	//
	//			newVS := snapshotv1.VolumeSnapshot{}
	//			By("checking volumesnapshot created by doBackup exists")
	//			Eventually(func(g Gomega) {
	//				g.Expect(k8sClient.Get(ctx, snapshotKey, &newVS)).Should(Succeed())
	//			}).Should(Succeed())
	//
	//			By("mocking volumesnapshot status ready")
	//			Expect(testapps.ChangeObjStatus(&testCtx, &newVS, func() {
	//				t := true
	//				newVS.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: &t}
	//			})).Should(Succeed())
	//
	//			By("do backup again, this time should create pvcs")
	//			shouldRequeue, err = doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
	//
	//			By("checking not requeue, since create pvc is the last step of doBackup")
	//			Expect(shouldRequeue).Should(BeFalse())
	//			Expect(err).ShouldNot(HaveOccurred())
	//
	//			By("checking pvcs reference right volumesnapshot")
	//			Eventually(func(g Gomega) {
	//				for i := *stsProto.Spec.Replicas - 1; i > *sts.Spec.Replicas; i-- {
	//					pvc := &corev1.PersistentVolumeClaim{}
	//					g.Expect(k8sClient.Get(ctx,
	//						types.NamespacedName{
	//							Namespace: cluster.Namespace,
	//							Name:      fmt.Sprintf("%s-%s-%d", testapps.DataVolumeName, sts.Name, i)},
	//						pvc)).Should(Succeed())
	//					g.Expect(pvc.Spec.DataSource.Name).Should(Equal(snapshotKey.Name))
	//				}
	//			}).Should(Succeed())
	//
	//			By("remove finalizer to clean up")
	//			Expect(testapps.ChangeObj(&testCtx, vs, func() {
	//				vs.SetFinalizers(vs.Finalizers[:len(vs.Finalizers)-1])
	//			})).Should(Succeed())
	//		})
	//	})
	//
	//	Context("backup test", func() {
	//		It("should not delete backups not created by lifecycle", func() {
	//			backupPolicyName := "test-backup-policy"
	//			backupName := "test-backup-job"
	//
	//			By("creating a backup as user do")
	//			backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
	//				SetTTL("7d").
	//				SetBackupPolicyName(backupPolicyName).
	//				SetBackupType(dataprotectionv1alpha1.BackupTypeSnapshot).
	//				AddAppInstanceLabel(clusterName).
	//				AddAppComponentLabel(mysqlCompName).
	//				AddAppManangedByLabel().
	//				Create(&testCtx).GetObject()
	//			backupKey := client.ObjectKeyFromObject(backup)
	//
	//			By("checking backup exists")
	//			Eventually(func(g Gomega) {
	//				tmpBackup := dataprotectionv1alpha1.Backup{}
	//				g.Expect(k8sClient.Get(ctx, backupKey, &tmpBackup)).Should(Succeed())
	//				g.Expect(tmpBackup.Labels[constant.AppInstanceLabelKey]).NotTo(Equal(""))
	//				g.Expect(tmpBackup.Labels[constant.KBAppComponentLabelKey]).NotTo(Equal(""))
	//			}).Should(Succeed())
	//
	//			By("call deleteBackup in lifecycle which should only delete backups created by itself")
	//			Expect(deleteBackup(ctx, k8sClient, clusterName, mysqlCompName))
	//
	//			By("checking backup does not be deleted")
	//			Consistently(func(g Gomega) {
	//				tmpBackup := dataprotectionv1alpha1.Backup{}
	//				Expect(k8sClient.Get(ctx, backupKey, &tmpBackup)).Should(Succeed())
	//			}).Should(Succeed())
	//		})
	//	})
})
