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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/leaanthony/debme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

const (
	kFake = "fake"
)

var tlog = ctrl.Log.WithName("lifecycle_util_testing")

func TestReadCUETplFromEmbeddedFS(t *testing.T) {
	cueFS, err := debme.FS(cueTemplates, "cue")
	if err != nil {
		t.Error("Expected no error", err)
	}
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("conn_credential_template.cue"))
	if err != nil {
		t.Error("Expected no error", err)
	}
	tlog.Info("", "cueValue", cueTpl)
}

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

	Context("has the mergeMonitorConfig function", func() {
		var component *Component
		var cluster *dbaasv1alpha1.Cluster
		var clusterComp *dbaasv1alpha1.ClusterComponent
		var clusterDef *dbaasv1alpha1.ClusterDefinition
		var clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent

		BeforeEach(func() {
			component = &Component{}
			component.PodSpec = &corev1.PodSpec{}
			cluster = &dbaasv1alpha1.Cluster{}
			cluster.Name = "mysql-instance-3"
			clusterComp = &dbaasv1alpha1.ClusterComponent{}
			clusterComp.Monitor = true
			cluster.Spec.Components = append(cluster.Spec.Components, *clusterComp)
			clusterComp = &cluster.Spec.Components[0]

			clusterDef = &dbaasv1alpha1.ClusterDefinition{}
			clusterDef.Spec.Type = kStateMysql
			clusterDefComp = &dbaasv1alpha1.ClusterDefinitionComponent{}
			clusterDefComp.CharacterType = kMysql
			clusterDefComp.Monitor = &dbaasv1alpha1.MonitorConfig{
				BuiltIn: false,
				Exporter: &dbaasv1alpha1.ExporterConfig{
					ScrapePort: 9144,
					ScrapePath: "/metrics",
				},
			}
			clusterDef.Spec.Components = append(clusterDef.Spec.Components, *clusterDefComp)
			clusterDefComp = &clusterDef.Spec.Components[0]
		})

		It("should disable monitor if ClusterComponent.Monitor is false", func() {
			clusterComp.Monitor = false
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(BeEquivalentTo(0))
			}
		})

		It("should disable builtin monitor if ClusterDefinitionComponent.Monitor.BuiltIn is false and has valid ExporterConfig", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = false
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(9144))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(BeEquivalentTo(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent.Monitor.BuiltIn is false and lacks ExporterConfig", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = false
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent.Monitor.BuiltIn is true and CharacterType isn't recognizable", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent's CharacterType is empty", func() {
			// TODO fixme: seems setting clusterDef.Spec.Type has no effect to mergeMonitorConfig
			clusterComp.Monitor = true
			clusterDef.Spec.Type = kFake
			clusterDefComp.CharacterType = ""
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})
	})

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
		clusterDefObj := testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.MySQLType).
			AddComponent(testdbaas.StatefulMySQLComponent, mysqlCompType).
			AddComponent(testdbaas.StatelessNginxComponent, nginxCompType).
			GetClusterDef()
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
			GetClusterVersion()
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
			GetCluster()
		key := client.ObjectKeyFromObject(clusterObj)
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterObj)).Should(Succeed())
		}

		return clusterObj, clusterDefObj, clusterVersionObj, key
	}

	Context("has the mergeComponents function", func() {
		It("should work as expected with various inputs", func() {
			cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, true)
			By("assign every available fields")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component := mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())

			By("leave clusterVersion.podSpec nil")
			clusterVersion.Spec.Components[0].PodSpec = nil
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())

			clusterVersion = allFieldsClusterVersionObj(false)
			By("new container in clusterVersion not in clusterDefinition")
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[1],
				&cluster.Spec.Components[0])
			Expect(len(component.PodSpec.Containers)).Should(Equal(2))

			By("new init container in clusterVersion not in clusterDefinition")
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[1],
				&cluster.Spec.Components[0])
			Expect(len(component.PodSpec.InitContainers)).Should(Equal(1))

			By("leave clusterComp nil")
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				nil)
			Expect(component).ShouldNot(BeNil())

			By("leave clusterDefComp nil")
			component = mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				nil,
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).Should(BeNil())
		})
	})

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
			}).GetStatefulSet()
	}
	snapshotName := "test-snapshot-name"
	ctx := context.Background()
	newReqCtx := func() intctrlutil.RequestCtx {
		reqCtx := intctrlutil.RequestCtx{
			Ctx:      ctx,
			Log:      logger,
			Recorder: clusterRecorder,
		}
		return reqCtx
	}
	newAllFieldsComponent := func() *Component {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
		reqCtx := newReqCtx()
		By("assign every available fields")
		component := mergeComponents(
			reqCtx,
			cluster,
			clusterDef,
			&clusterDef.Spec.Components[0],
			&clusterVersion.Spec.Components[0],
			&cluster.Spec.Components[0])
		Expect(component).ShouldNot(BeNil())
		return component
	}
	newParams := func() *createParams {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
		params := createParams{
			clusterDefinition: clusterDef,
			clusterVersion:    clusterVersion,
			cluster:           cluster,
			component:         newAllFieldsComponent(),
			applyObjs:         nil,
			cacheCtx:          &map[string]interface{}{},
		}
		return &params
	}
	newBackupPolicyTemplate := func() *dataprotectionv1alpha1.BackupPolicyTemplate {
		return testdbaas.NewBackupPolicyTemplateFactory("backup-policy-template-mysql").
			SetBackupToolName("mysql-xtrabackup").
			SetSchedule("0 2 * * *").
			SetTTL("168h0m0s").
			AddHookPreCommand("touch /data/mysql/.restore;sync").
			AddHookPostCommand("rm -f /data/mysql/.restore;sync").
			Create(&testCtx).GetBackupPolicyTpl()
	}

	Context("has helper function which builds specific object from cue template", func() {
		It("builds PVC correctly", func() {
			sts := newStsObj()
			params := newParams()
			pvcKey := types.NamespacedName{
				Namespace: "default",
				Name:      "data-mysql-01-replicasets-0",
			}
			pvc, err := buildPVCFromSnapshot(sts, params.component, pvcKey, snapshotName)
			Expect(err).Should(BeNil())
			Expect(pvc).ShouldNot(BeNil())
			Expect(pvc.Spec.AccessModes).Should(Equal(sts.Spec.VolumeClaimTemplates[0].Spec.AccessModes))
			Expect(pvc.Spec.Resources).Should(Equal(params.component.VolumeClaimTemplates[0].Spec.Resources))
		})

		It("builds Service correctly", func() {
			params := newParams()
			svc, err := buildSvc(*params, true)
			Expect(err).Should(BeNil())
			Expect(svc).ShouldNot(BeNil())
		})

		It("builds ConnCredential correctly", func() {
			params := newParams()
			credential, err := buildConnCredential(*params)
			Expect(err).Should(BeNil())
			Expect(credential).ShouldNot(BeNil())
		})

		It("builds StatefulSet correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			envConfigName := "test-env-config-name"
			newParams := params
			newComponent := *params.component
			newComponent.Replicas = 0
			newParams.component = &newComponent
			sts, err := buildSts(reqCtx, *newParams, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
			sts, err = buildSts(reqCtx, *params, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
		})

		It("builds Deploy correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			deploy, err := buildDeploy(reqCtx, *params)
			Expect(err).Should(BeNil())
			Expect(deploy).ShouldNot(BeNil())
		})

		It("builds PDB correctly", func() {
			params := newParams()
			pdb, err := buildPDB(*params)
			Expect(err).Should(BeNil())
			Expect(pdb).ShouldNot(BeNil())
		})

		It("builds Env Config correctly", func() {
			params := newParams()
			cfg, err := buildEnvConfig(*params)
			Expect(err).Should(BeNil())
			Expect(cfg).ShouldNot(BeNil())
			Expect(len(cfg.Data) == 2).Should(BeTrue())
		})

		It("builds BackupPolicy correctly", func() {
			sts := newStsObj()
			backupPolicyTemplate := newBackupPolicyTemplate()
			backupKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-backup",
			}
			policy, err := buildBackupPolicy(sts, backupPolicyTemplate, backupKey)
			Expect(err).Should(BeNil())
			Expect(policy).ShouldNot(BeNil())
		})

		It("builds BackupJob correctly", func() {
			sts := newStsObj()
			backupJobKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-backup-job",
			}
			backupPolicyName := "test-backup-policy"
			backupJob, err := buildBackup(sts, backupPolicyName, backupJobKey)
			Expect(err).Should(BeNil())
			Expect(backupJob).ShouldNot(BeNil())
		})

		It("builds VolumeSnapshot correctly", func() {
			sts := newStsObj()
			snapshotKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-snapshot",
			}
			pvcName := "test-pvc-name"
			vs, err := buildVolumeSnapshot(snapshotKey, pvcName, sts)
			Expect(err).Should(BeNil())
			Expect(vs).ShouldNot(BeNil())
		})

		It("builds CronJob correctly", func() {
			sts := newStsObj()
			pvcKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-pvc",
			}
			schedule := "* * * * *"
			cronJob, err := buildCronJob(pvcKey, schedule, sts)
			Expect(err).Should(BeNil())
			Expect(cronJob).ShouldNot(BeNil())
		})
	})

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
    app.kubernetes.io/name: state.mysql-apecloud-mysql
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
			component := mergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
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
