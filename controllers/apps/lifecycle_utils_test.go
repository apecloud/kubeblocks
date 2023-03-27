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

package apps

import (
	"context"
	// "fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("lifecycle_utils", func() {

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResources(&testCtx, generics.VolumeSnapshotSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterName = "test-cluster"

	const mysqlCompType = "replicasets"
	const mysqlCompName = "mysql"

	const nginxCompType = "proxy"

	allFieldsClusterDefObj := func(needCreate bool) *appsv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
			AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
			AddComponent(testapps.StatelessNginxComponent, nginxCompType).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterDefObj)).Should(Succeed())
		}
		return clusterDefObj
	}

	allFieldsClusterVersionObj := func(needCreate bool) *appsv1alpha1.ClusterVersion {
		By("By assure an clusterVersion obj")
		clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponent(mysqlCompType).
			AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponent(nginxCompType).
			AddInitContainerShort("nginx-init", testapps.NginxImage).
			AddContainerShort("nginx", testapps.NginxImage).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterVersionObj)).Should(Succeed())
		}
		return clusterVersionObj
	}

	newAllFieldsClusterObj := func(
		clusterDefObj *appsv1alpha1.ClusterDefinition,
		clusterVersionObj *appsv1alpha1.ClusterVersion,
		needCreate bool,
	) (*appsv1alpha1.Cluster, *appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and ClusterVersion objects if not provided
		if clusterDefObj == nil {
			clusterDefObj = allFieldsClusterDefObj(needCreate)
		}
		if clusterVersionObj == nil {
			clusterVersionObj = allFieldsClusterVersionObj(needCreate)
		}

		pvcSpec := testapps.NewPVC("1Gi")
		clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
			GetObject()
		key := client.ObjectKeyFromObject(clusterObj)
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterObj)).Should(Succeed())
		}

		return clusterObj, clusterDefObj, clusterVersionObj, key
	}

	// NOTES: following code are problematic, caused "Ginkgo detected an issue with your spec structure":
	//   It looks like you are calling By outside of a running spec.  Make sure you
	//   call By inside a runnable node such as It or BeforeEach and not inside the
	//   body of a container such as Describe or Context.
	// REVIEW: is mock sts workload necessary? why?
	newStsObj := func() *appsv1.StatefulSet {
		container := corev1.Container{
			Name: "mysql",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "mysql-config",
				MountPath: "/mnt/config",
			}},
		}
		return testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, "mock-sts", clusterName, mysqlCompName).
			AddAppNameLabel("mock-app").
			AddAppInstanceLabel(clusterName).
			AddAppComponentLabel(mysqlCompName).
			SetReplicas(1).
			AddContainer(container).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: testapps.DataVolumeName},
				Spec:       testapps.NewPVC("1Gi"),
			}).GetObject()
	}
	ctx := context.Background()
	newReqCtx := func() intctrlutil.RequestCtx {
		reqCtx := intctrlutil.RequestCtx{
			Ctx:      ctx,
			Log:      logger,
			Recorder: clusterRecorder,
		}
		return reqCtx
	}

	newVolumeSnapshot := func(clusterName, componentName string) *snapshotv1.VolumeSnapshot {
		vsYAML := `
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  labels:
    app.kubernetes.io/name: mysql-apecloud-mysql
    backupjobs.dataprotection.kubeblocks.io/name: wesql-01-replicasets-scaling-qf6cr
    backuppolicies.dataprotection.kubeblocks.io/name: wesql-01-replicasets-scaling-hcxps
    dataprotection.kubeblocks.io/backup-type: snapshot
  name: test-volume-snapshot
  namespace: default
spec:
  source:
    persistentVolumeClaimName: data-wesql-01-replicasets-0
  volumeSnapshotClassName: csi-aws-ebs-snapclass
`
		vs := snapshotv1.VolumeSnapshot{}
		Expect(yaml.Unmarshal([]byte(vsYAML), &vs)).Should(Succeed())
		labels := map[string]string{
			constant.KBManagedByKey:         "cluster",
			constant.AppInstanceLabelKey:    clusterName,
			constant.KBAppComponentLabelKey: componentName,
		}
		for k, v := range labels {
			vs.Labels[k] = v
		}
		return &vs
	}

	Context("with HorizontalScalePolicy set to CloneFromSnapshot and VolumeSnapshot exists", func() {
		It("determines return value of doBackup according to whether VolumeSnapshot is ReadyToUse", func() {
			By("prepare cluster and construct component")
			reqCtx := newReqCtx()
			cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
			component := component.BuildComponent(
				reqCtx,
				*cluster,
				*clusterDef,
				clusterDef.Spec.ComponentDefs[0],
				cluster.Spec.ComponentSpecs[0],
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(component).ShouldNot(BeNil())
			component.HorizontalScalePolicy = &appsv1alpha1.HorizontalScalePolicy{
				Type:             appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
				VolumeMountsName: "data",
			}

			By("prepare VolumeSnapshot and set ReadyToUse to true")
			vs := newVolumeSnapshot(cluster.Name, mysqlCompName)
			Expect(testCtx.CreateObj(ctx, vs)).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, vs, func() {
				t := true
				vs.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: &t}
			})).Should(Succeed())

			// prepare doBackup input parameters
			snapshotKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-snapshot",
			}
			sts := newStsObj()
			stsProto := *sts.DeepCopy()
			r := int32(3)
			stsProto.Spec.Replicas = &r

			By("doBackup should return requeue=false")
			shouldRequeue, err := doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
			Expect(shouldRequeue).Should(BeFalse())
			Expect(err).ShouldNot(HaveOccurred())

			By("Set ReadyToUse to nil, doBackup should return requeue=true")
			Expect(testapps.ChangeObjStatus(&testCtx, vs, func() {
				vs.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: nil}
			})).Should(Succeed())
			shouldRequeue, err = doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
			Expect(shouldRequeue).Should(BeTrue())
			Expect(err).ShouldNot(HaveOccurred())
		})

		// It("should do backup to create volumesnapshot when there exists a deleting volumesnapshot", func() {
		// 	By("prepare cluster and construct component")
		// 	reqCtx := newReqCtx()
		// 	cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
		// 	component := component.BuildComponent(
		// 		reqCtx,
		// 		*cluster,
		// 		*clusterDef,
		// 		clusterDef.Spec.ComponentDefs[0],
		// 		cluster.Spec.ComponentSpecs[0],
		// 		&clusterVersion.Spec.ComponentVersions[0])
		// 	Expect(component).ShouldNot(BeNil())
		// 	component.HorizontalScalePolicy = &appsv1alpha1.HorizontalScalePolicy{
		// 		Type:             appsv1alpha1.HScaleDataClonePolicyFromSnapshot,
		// 		VolumeMountsName: "data",
		// 	}

		// 	By("prepare VolumeSnapshot and set finalizer to prevent it from deletion")
		// 	vs := newVolumeSnapshot(cluster.Name, mysqlCompName)
		// 	Expect(testCtx.CreateObj(ctx, vs)).Should(Succeed())
		// 	Expect(testapps.ChangeObj(&testCtx, vs, func() {
		// 		vs.Finalizers = append(vs.Finalizers, "test-finalizer")
		// 	})).Should(Succeed())

		// 	By("deleting volume snapshot")
		// 	Expect(k8sClient.Delete(ctx, vs)).Should(Succeed())

		// 	By("checking DeletionTimestamp exists")
		// 	Eventually(func(g Gomega) {
		// 		tmpVS := snapshotv1.VolumeSnapshot{}
		// 		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: vs.Namespace, Name: vs.Name}, &tmpVS)).Should(Succeed())
		// 		g.Expect(tmpVS.DeletionTimestamp).ShouldNot(BeNil())
		// 	}).Should(Succeed())

		// 	// prepare doBackup input parameters
		// 	snapshotKey := types.NamespacedName{
		// 		Namespace: "default",
		// 		Name:      "test-snapshot",
		// 	}
		// 	sts := newStsObj()
		// 	stsProto := *sts.DeepCopy()
		// 	r := int32(3)
		// 	stsProto.Spec.Replicas = &r

		// 	By("doBackup should create volumesnapshot and return requeue=true")
		// 	shouldRequeue, err := doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
		// 	Expect(err).ShouldNot(HaveOccurred())
		// 	Expect(shouldRequeue).Should(BeTrue())

		// 	newVS := snapshotv1.VolumeSnapshot{}
		// 	By("checking volumesnapshot created by doBackup exists")
		// 	Eventually(func(g Gomega) {
		// 		g.Expect(k8sClient.Get(ctx, snapshotKey, &newVS)).Should(Succeed())
		// 	}).Should(Succeed())

		// 	By("mocking volumesnapshot status ready")
		// 	Expect(testapps.ChangeObjStatus(&testCtx, &newVS, func() {
		// 		t := true
		// 		newVS.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: &t}
		// 	})).Should(Succeed())

		// 	By("do backup again, this time should create pvcs")
		// 	shouldRequeue, err = doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)

		// 	By("checking not requeue, since create pvc is the last step of doBackup")
		// 	Expect(shouldRequeue).Should(BeFalse())
		// 	Expect(err).ShouldNot(HaveOccurred())

		// 	By("checking pvcs reference right volumesnapshot")
		// 	Eventually(func(g Gomega) {
		// 		for i := *stsProto.Spec.Replicas - 1; i > *sts.Spec.Replicas; i-- {
		// 			pvc := &corev1.PersistentVolumeClaim{}
		// 			g.Expect(k8sClient.Get(ctx,
		// 				types.NamespacedName{
		// 					Namespace: cluster.Namespace,
		// 					Name:      fmt.Sprintf("%s-%s-%d", testapps.DataVolumeName, sts.Name, i)},
		// 				pvc)).Should(Succeed())
		// 			g.Expect(pvc.Spec.DataSource.Name == snapshotKey.Name).Should(BeTrue())
		// 		}
		// 	}).Should(Succeed())

		// 	By("remove finalizer to cleanup")
		// 	Expect(testapps.ChangeObj(&testCtx, vs, func() {
		// 		vs.SetFinalizers(vs.Finalizers[:len(vs.Finalizers)-1])
		// 	})).Should(Succeed())
		// })
	})

	Context("utils test", func() {
		It("should successfully delete object with cascade=orphan", func() {
			sts := newStsObj()
			Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
			Expect(deleteObjectOrphan(k8sClient, ctx, sts)).Should(Succeed())
		})
	})

	Context("test mergeServiceAnnotations", func() {
		It("original and target annotations are nil", func() {
			Expect(mergeServiceAnnotations(nil, nil)).Should(BeNil())
		})
		It("target annotations is nil", func() {
			originalAnnotations := map[string]string{"k1": "v1"}
			Expect(mergeServiceAnnotations(originalAnnotations, nil)).To(Equal(originalAnnotations))
		})
		It("original annotations is nil", func() {
			targetAnnotations := map[string]string{"k1": "v1"}
			Expect(mergeServiceAnnotations(nil, targetAnnotations)).To(Equal(targetAnnotations))
		})
		It("original annotations have prometheus annotations which should be removed", func() {
			originalAnnotations := map[string]string{"k1": "v1", "prometheus.io/path": "/metrics"}
			targetAnnotations := map[string]string{"k2": "v2"}
			expectAnnotations := map[string]string{"k1": "v1", "k2": "v2"}
			Expect(mergeServiceAnnotations(originalAnnotations, targetAnnotations)).To(Equal(expectAnnotations))
		})
		It("target annotations should override original annotations", func() {
			originalAnnotations := map[string]string{"k1": "v1", "prometheus.io/path": "/metrics"}
			targetAnnotations := map[string]string{"k1": "v11"}
			expectAnnotations := map[string]string{"k1": "v11"}
			Expect(mergeServiceAnnotations(originalAnnotations, targetAnnotations)).To(Equal(expectAnnotations))
		})
	})

	Context("backup test", func() {
		It("should not delete backups not created by lifecycle", func() {
			backupPolicyName := "test-backup-policy"
			backupName := "test-backup-job"

			By("creating a backup as user do")
			backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				SetTTL("168h0m0s").
				SetBackupPolicyName(backupPolicyName).
				SetBackupType(dataprotectionv1alpha1.BackupTypeSnapshot).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(mysqlCompName).
				AddAppManangedByLabel().
				Create(&testCtx).GetObject()
			backupKey := client.ObjectKeyFromObject(backup)

			By("checking backup exists")
			Eventually(func(g Gomega) {
				tmpBackup := dataprotectionv1alpha1.Backup{}
				g.Expect(k8sClient.Get(ctx, backupKey, &tmpBackup)).Should(Succeed())
				g.Expect(tmpBackup.Labels[constant.AppInstanceLabelKey]).NotTo(Equal(""))
				g.Expect(tmpBackup.Labels[constant.KBAppComponentLabelKey]).NotTo(Equal(""))
			}).Should(Succeed())

			By("call deleteBackup in lifecycle which should only delete backups created by itself")
			Expect(deleteBackup(ctx, k8sClient, clusterName, mysqlCompName))

			By("checking backup does not be deleted")
			Consistently(func(g Gomega) {
				tmpBackup := dataprotectionv1alpha1.Backup{}
				Expect(k8sClient.Get(ctx, backupKey, &tmpBackup)).Should(Succeed())
			}).Should(Succeed())
		})
	})
})
