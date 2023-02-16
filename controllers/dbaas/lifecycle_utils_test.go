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

package dbaas

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("lifecycle_utils", func() {

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testdbaas.ClearResources(&testCtx, intctrlutil.VolumeSnapshotSignature, inNS, ml)
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("has the checkAndUpdatePodVolumes function which generates Pod Volumes for mounting ConfigMap objects", func() {
		var sts appsv1.StatefulSet
		var volumes map[string]dbaasv1alpha1.ConfigTemplate
		BeforeEach(func() {
			sts = appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name:            "mysql",
									Image:           "docker.io/apecloud/apecloud-mysql-server:latest",
									ImagePullPolicy: "IfNotPresent",
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/data",
										},
									},
								},
							},
						},
					},
				},
			}
			volumes = make(map[string]dbaasv1alpha1.ConfigTemplate)

		})

		It("should succeed in corner case where input volumes is nil, which means no volume is added", func() {
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(1))
		})

		It("should succeed in normal test case, where one volume is added", func() {
			volumes["my_config"] = dbaasv1alpha1.ConfigTemplate{
				Name:                "myConfig",
				ConfigTplRef:        "myConfig",
				ConfigConstraintRef: "myConfig",
				VolumeName:          "myConfigVolume",
			}
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(2))
		})

		It("should succeed in normal test case, where two volumes are added", func() {
			volumes["my_config"] = dbaasv1alpha1.ConfigTemplate{
				Name:                "myConfig",
				ConfigTplRef:        "myConfig",
				ConfigConstraintRef: "myConfig",
				VolumeName:          "myConfigVolume",
			}
			volumes["my_config1"] = dbaasv1alpha1.ConfigTemplate{
				Name:                "myConfig",
				ConfigTplRef:        "myConfig",
				ConfigConstraintRef: "myConfig",
				VolumeName:          "myConfigVolume2",
			}
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(3))
		})

		It("should fail if updated volume doesn't contain ConfigMap", func() {
			const (
				cmName            = "my_config_for_test"
				replicaVolumeName = "mytest-cm-volume_for_test"
			)
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
				corev1.Volume{
					Name: replicaVolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				})
			volumes[cmName] = dbaasv1alpha1.ConfigTemplate{
				Name:                "configTplName",
				ConfigTplRef:        "configTplName",
				ConfigConstraintRef: "configTplName",
				VolumeName:          replicaVolumeName,
			}
			ps := &sts.Spec.Template.Spec
			Expect(checkAndUpdatePodVolumes(ps, volumes)).ShouldNot(Succeed())
		})

		It("should succeed if updated volume contains ConfigMap", func() {
			const (
				cmName            = "my_config_for_isv"
				replicaVolumeName = "mytest-cm-volume_for_isv"
			)

			// mock clusterdefinition has volume
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
				corev1.Volume{
					Name: replicaVolumeName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "anything"},
						},
					},
				})

			volumes[cmName] = dbaasv1alpha1.ConfigTemplate{
				Name:                "configTplName",
				ConfigTplRef:        "configTplName",
				ConfigConstraintRef: "configTplName",
				VolumeName:          replicaVolumeName,
			}
			ps := &sts.Spec.Template.Spec
			err := checkAndUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(sts.Spec.Template.Spec.Volumes)).To(Equal(2))
			volume := intctrlutil.GetVolumeMountName(sts.Spec.Template.Spec.Volumes, cmName)
			Expect(volume).ShouldNot(BeNil())
			Expect(volume.ConfigMap).ShouldNot(BeNil())
			Expect(volume.ConfigMap.Name).Should(BeEquivalentTo(cmName))
			Expect(volume.Name).Should(BeEquivalentTo(replicaVolumeName))
		})

	})

	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterName = "test-cluster"

	const mysqlCompType = "replicasets"
	const mysqlCompName = "mysql"

	const nginxCompType = "proxy"

	allFieldsClusterDefObj := func(needCreate bool) *dbaasv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefObj := testdbaas.NewClusterDefFactory(clusterDefName).
			AddComponent(testdbaas.StatefulMySQLComponent, mysqlCompType).
			AddComponent(testdbaas.StatelessNginxComponent, nginxCompType).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterDefObj)).Should(Succeed())
		}
		return clusterDefObj
	}

	allFieldsClusterVersionObj := func(needCreate bool) *dbaasv1alpha1.ClusterVersion {
		By("By assure an clusterVersion obj")
		clusterVersionObj := testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponent(mysqlCompType).
			AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
			AddComponent(nginxCompType).
			AddInitContainerShort("nginx-init", testdbaas.NginxImage).
			AddContainerShort("nginx", testdbaas.NginxImage).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterVersionObj)).Should(Succeed())
		}
		return clusterVersionObj
	}

	newAllFieldsClusterObj := func(
		clusterDefObj *dbaasv1alpha1.ClusterDefinition,
		clusterVersionObj *dbaasv1alpha1.ClusterVersion,
		needCreate bool,
	) (*dbaasv1alpha1.Cluster, *dbaasv1alpha1.ClusterDefinition, *dbaasv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj required default ClusterDefinition and ClusterVersion objects if not provided
		if clusterDefObj == nil {
			clusterDefObj = allFieldsClusterDefObj(needCreate)
		}
		if clusterVersionObj == nil {
			clusterVersionObj = allFieldsClusterVersionObj(needCreate)
		}

		pvcSpec := testdbaas.NewPVC("1Gi")
		clusterObj := testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).
			AddVolumeClaimTemplate(testdbaas.DataVolumeName, &pvcSpec).
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

	newStsObj := func() *appsv1.StatefulSet {
		container := corev1.Container{
			Name: "mysql",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "mysql-config",
				MountPath: "/mnt/config",
			}},
		}
		return testdbaas.NewStatefulSetFactory(testCtx.DefaultNamespace, "mock-sts", clusterName, mysqlCompName).
			AddLabels(intctrlutil.AppNameLabelKey, "mock-app",
				intctrlutil.AppInstanceLabelKey, clusterName,
				intctrlutil.AppComponentLabelKey, mysqlCompName,
			).SetReplicas(1).AddContainer(container).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: testdbaas.DataVolumeName},
				Spec:       testdbaas.NewPVC("1Gi"),
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
		vsYAML := fmt.Sprintf(`
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  labels:
    app.kubernetes.io/component-name: %s
    app.kubernetes.io/created-by: kubeblocks
    app.kubernetes.io/instance: %s
    app.kubernetes.io/managed-by: kubeblocks
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
`, componentName, clusterName)
		vs := snapshotv1.VolumeSnapshot{}
		Expect(yaml.Unmarshal([]byte(vsYAML), &vs)).Should(Succeed())
		return &vs
	}

	Context("with HorizontalScalePolicy set to CloneFromSnapshot and VolumeSnapshot exists", func() {
		It("determines return value of doBackup according to whether VolumeSnapshot is ReadyToUse", func() {
			By("prepare cluster and construct component")
			reqCtx := newReqCtx()
			cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
			component := component.MergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&clusterVersion.Spec.ComponentVersions[0],
				&cluster.Spec.ComponentSpecs[0])
			Expect(component).ShouldNot(BeNil())
			component.HorizontalScalePolicy = &dbaasv1alpha1.HorizontalScalePolicy{
				Type:             dbaasv1alpha1.HScaleDataClonePolicyFromSnapshot,
				VolumeMountsName: "data",
			}

			By("prepare VolumeSnapshot and set ReadyToUse to true")
			vs := newVolumeSnapshot(cluster.Name, mysqlCompName)
			Expect(testCtx.CreateObj(ctx, vs)).Should(Succeed())
			Expect(testdbaas.ChangeObjStatus(&testCtx, vs, func() {
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
			Expect(testdbaas.ChangeObjStatus(&testCtx, vs, func() {
				vs.Status = &snapshotv1.VolumeSnapshotStatus{ReadyToUse: nil}
			})).Should(Succeed())
			shouldRequeue, err = doBackup(reqCtx, k8sClient, cluster, component, sts, &stsProto, snapshotKey)
			Expect(shouldRequeue).Should(BeTrue())
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("utils test", func() {
		It("should successfully delete object with cascade=orphan", func() {
			sts := newStsObj()
			Expect(k8sClient.Create(ctx, sts)).Should(Succeed())
			Expect(deleteObjectOrphan(k8sClient, ctx, sts)).Should(Succeed())
		})
	})
})
