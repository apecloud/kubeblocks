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

package builder

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/leaanthony/debme"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var tlog = ctrl.Log.WithName("builder_testing")

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

var _ = Describe("builder", func() {
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

		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompType).SetReplicas(1).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			AddService(testapps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
			AddService(testapps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
			GetObject()
		key := client.ObjectKeyFromObject(clusterObj)
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterObj)).Should(Succeed())
		}

		return clusterObj, clusterDefObj, clusterVersionObj, key
	}

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
			SetReplicas(1).AddContainer(container).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: testapps.DataVolumeName},
				Spec:       testapps.NewPVC("1Gi"),
			}).GetObject()
	}
	newReqCtx := func() intctrlutil.RequestCtx {
		reqCtx := intctrlutil.RequestCtx{
			Ctx:      testCtx.Ctx,
			Log:      logger,
			Recorder: clusterRecorder,
		}
		return reqCtx
	}
	newAllFieldsComponent := func(clusterDef *appsv1alpha1.ClusterDefinition, clusterVersion *appsv1alpha1.ClusterVersion) *component.SynthesizedComponent {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(clusterDef, clusterVersion, false)
		reqCtx := newReqCtx()
		By("assign every available fields")
		component := component.BuildComponent(
			reqCtx,
			*cluster,
			*clusterDef,
			clusterDef.Spec.ComponentDefs[0],
			cluster.Spec.ComponentSpecs[0],
			&clusterVersion.Spec.ComponentVersions[0])
		Expect(component).ShouldNot(BeNil())
		return component
	}
	newParams := func() *BuilderParams {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(nil, nil, false)
		params := BuilderParams{
			ClusterDefinition: clusterDef,
			ClusterVersion:    clusterVersion,
			Cluster:           cluster,
			Component:         newAllFieldsComponent(clusterDef, clusterVersion),
		}
		return &params
	}

	newParamsWithClusterDef := func(clusterDefObj *appsv1alpha1.ClusterDefinition) *BuilderParams {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(clusterDefObj, nil, false)
		params := BuilderParams{
			ClusterDefinition: clusterDef,
			ClusterVersion:    clusterVersion,
			Cluster:           cluster,
			Component:         newAllFieldsComponent(clusterDef, clusterVersion),
		}
		return &params
	}

	newBackupPolicyTemplate := func() *dataprotectionv1alpha1.BackupPolicyTemplate {
		return testapps.NewBackupPolicyTemplateFactory("backup-policy-template-mysql").
			SetBackupToolName("mysql-xtrabackup").
			SetSchedule("0 2 * * *").
			SetTTL("168h0m0s").
			AddHookPreCommand("touch /data/mysql/.restore;sync").
			AddHookPostCommand("rm -f /data/mysql/.restore;sync").
			Create(&testCtx).GetObject()
	}

	Context("has helper function which builds specific object from cue template", func() {
		It("builds PVC correctly", func() {
			snapshotName := "test-snapshot-name"
			sts := newStsObj()
			params := newParams()
			pvcKey := types.NamespacedName{
				Namespace: "default",
				Name:      "data-mysql-01-replicasets-0",
			}
			pvc, err := BuildPVCFromSnapshot(sts, params.Component.VolumeClaimTemplates[0], pvcKey, snapshotName, params.Component)
			Expect(err).Should(BeNil())
			Expect(pvc).ShouldNot(BeNil())
			Expect(pvc.Spec.AccessModes).Should(Equal(sts.Spec.VolumeClaimTemplates[0].Spec.AccessModes))
			Expect(pvc.Spec.Resources).Should(Equal(params.Component.VolumeClaimTemplates[0].Spec.Resources))
			Expect(pvc.Labels[constant.VolumeTypeLabelKey]).ShouldNot(BeEmpty())
		})

		It("builds Service correctly", func() {
			params := newParams()
			svcList, err := BuildSvcList(*params)
			Expect(err).Should(BeNil())
			Expect(svcList).ShouldNot(BeEmpty())
		})

		It("builds Conn. Credential correctly", func() {
			params := newParamsWithClusterDef(testapps.NewClusterDefFactoryWithConnCredential("conn-cred").GetObject())
			credential, err := BuildConnCredential(*params)
			Expect(err).Should(BeNil())
			Expect(credential).ShouldNot(BeNil())
			Expect(credential.Labels["apps.kubeblocks.io/cluster-type"]).Should(BeEmpty())
			By("setting type")
			characterType := "test-character-type"
			params.ClusterDefinition.Spec.Type = characterType
			credential, err = BuildConnCredential(*params)
			Expect(err).Should(BeNil())
			Expect(credential).ShouldNot(BeNil())
			Expect(credential.Labels["apps.kubeblocks.io/cluster-type"]).Should(Equal(characterType))
			// "username":      "root",
			// "SVC_FQDN":      "$(SVC_FQDN)",
			// "RANDOM_PASSWD": "$(RANDOM_PASSWD)",
			// "tcpEndpoint":   "tcp:$(SVC_FQDN):$(SVC_PORT_mysql)",
			// "paxosEndpoint": "paxos:$(SVC_FQDN):$(SVC_PORT_paxos)",
			// "UUID":          "$(UUID)",
			// "UUID_B64":      "$(UUID_B64)",
			// "UUID_STR_B64":  "$(UUID_STR_B64)",
			// "UUID_HEX":      "$(UUID_HEX)",
			Expect(credential.StringData).ShouldNot(BeEmpty())
			Expect(credential.StringData["username"]).Should(Equal("root"))

			for _, v := range []string{
				"SVC_FQDN",
				"RANDOM_PASSWD",
				"UUID",
				"UUID_B64",
				"UUID_STR_B64",
				"UUID_HEX",
				"HEADLESS_SVC_FQDN",
			} {
				Expect(credential.StringData[v]).ShouldNot(BeEquivalentTo(fmt.Sprintf("$(%s)", v)))
			}
			Expect(credential.StringData["RANDOM_PASSWD"]).Should(HaveLen(8))
			svcFQDN := fmt.Sprintf("%s-%s.%s.svc", params.Cluster.Name, params.Component.Name,
				params.Cluster.Namespace)
			headlessSvcFQDN := fmt.Sprintf("%s-%s-headless.%s.svc", params.Cluster.Name, params.Component.Name,
				params.Cluster.Namespace)
			var mysqlPort corev1.ServicePort
			var paxosPort corev1.ServicePort
			for _, s := range params.Component.Services[0].Spec.Ports {
				switch s.Name {
				case "mysql":
					mysqlPort = s
				case "paxos":
					paxosPort = s
				}
			}
			Expect(credential.StringData["SVC_FQDN"]).Should(Equal(svcFQDN))
			Expect(credential.StringData["HEADLESS_SVC_FQDN"]).Should(Equal(headlessSvcFQDN))
			Expect(credential.StringData["tcpEndpoint"]).Should(Equal(fmt.Sprintf("tcp:%s:%d", svcFQDN, mysqlPort.Port)))
			Expect(credential.StringData["paxosEndpoint"]).Should(Equal(fmt.Sprintf("paxos:%s:%d", svcFQDN, paxosPort.Port)))

		})

		It("builds StatefulSet correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			envConfigName := "test-env-config-name"
			newParams := params

			sts, err := BuildSts(reqCtx, *params, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
			// test  replicas = 0
			newComponent := *params.Component
			newComponent.Replicas = 0
			newParams.Component = &newComponent
			sts, err = BuildSts(reqCtx, *newParams, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
			Expect(*sts.Spec.Replicas).Should(Equal(int32(0)))
			Expect(sts.Spec.VolumeClaimTemplates[0].Labels[constant.VolumeTypeLabelKey]).
				Should(Equal(string(appsv1alpha1.VolumeTypeData)))
			// test workload type replication
			replComponent := *params.Component
			replComponent.Replicas = 2
			replComponent.WorkloadType = appsv1alpha1.Replication
			newParams.Component = &replComponent
			sts, err = BuildSts(reqCtx, *newParams, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
			Expect(*sts.Spec.Replicas).Should(Equal(int32(1)))
		})

		It("builds Deploy correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			deploy, err := BuildDeploy(reqCtx, *params)
			Expect(err).Should(BeNil())
			Expect(deploy).ShouldNot(BeNil())
		})

		It("builds PDB correctly", func() {
			params := newParams()
			pdb, err := BuildPDB(*params)
			Expect(err).Should(BeNil())
			Expect(pdb).ShouldNot(BeNil())
		})

		It("builds Env Config correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			cfg, err := BuildEnvConfig(*params, reqCtx, k8sClient)
			Expect(err).Should(BeNil())
			Expect(cfg).ShouldNot(BeNil())
			Expect(len(cfg.Data) == 3).Should(BeTrue())
		})

		It("builds env config with resources recreate", func() {
			reqCtx := newReqCtx()
			params := newParams()

			By("creating pvc to make it looks like recreation")
			pvcName := "test-pvc"
			testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, pvcName, clusterName,
				params.Component.Name, testapps.DataVolumeName).SetStorage("1Gi").CheckedCreate(&testCtx)

			cfg, err := BuildEnvConfig(*params, reqCtx, k8sClient)
			Expect(err).Should(BeNil())
			Expect(cfg).ShouldNot(BeNil())
			Expect(cfg.Data["KB_"+strings.ToUpper(params.Component.Type)+"_RECREATE"]).Should(Equal("true"))
		})

		It("builds Env Config with ConsensusSet status correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			params.Cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
				params.Component.Name: {
					ConsensusSetStatus: &appsv1alpha1.ConsensusSetStatus{
						Leader: appsv1alpha1.ConsensusMemberStatus{
							Pod: "pod1",
						},
						Followers: []appsv1alpha1.ConsensusMemberStatus{{
							Pod: "pod2",
						}, {
							Pod: "pod3",
						}},
					},
				}}
			cfg, err := BuildEnvConfig(*params, reqCtx, k8sClient)
			Expect(err).Should(BeNil())
			Expect(cfg).ShouldNot(BeNil())
			Expect(len(cfg.Data) == 5).Should(BeTrue())
		})

		It("builds Env Config with Replication component correctly", func() {
			reqCtx := newReqCtx()
			params := newParams()
			params.Component.WorkloadType = appsv1alpha1.Replication

			var cfg *corev1.ConfigMap
			var err error

			checkEnvValues := func() {
				cfg, err = BuildEnvConfig(*params, reqCtx, k8sClient)
				Expect(err).Should(BeNil())
				Expect(cfg).ShouldNot(BeNil())
				Expect(len(cfg.Data) == int(3+params.Component.Replicas)).Should(BeTrue())
				Expect(cfg.Data["KB_"+strings.ToUpper(params.Component.Type)+"_N"]).
					Should(Equal(strconv.Itoa(int(params.Component.Replicas))))
				stsName := fmt.Sprintf("%s-%s", params.Cluster.Name, params.Component.Name)
				svcName := fmt.Sprintf("%s-headless", stsName)
				By("Checking KB_PRIMARY_POD_NAME value be right")
				if int(params.Component.GetPrimaryIndex()) == 0 {
					Expect(cfg.Data["KB_PRIMARY_POD_NAME"]).
						Should(Equal(stsName + "-" + strconv.Itoa(int(params.Component.GetPrimaryIndex())) + "." + svcName))
				} else {
					Expect(cfg.Data["KB_PRIMARY_POD_NAME"]).
						Should(Equal(stsName + "-" + strconv.Itoa(int(params.Component.GetPrimaryIndex())) + "-0." + svcName))
				}
				for i := 0; i < int(params.Component.Replicas); i++ {
					if i == 0 {
						By("Checking the 1st replica's hostname should not have suffix '-0'")
						Expect(cfg.Data["KB_"+strings.ToUpper(params.Component.Type)+"_"+strconv.Itoa(i)+"_HOSTNAME"]).
							Should(Equal(stsName + "-" + strconv.Itoa(0) + "." + svcName))
					} else {
						Expect(cfg.Data["KB_"+strings.ToUpper(params.Component.Type)+"_"+strconv.Itoa(i)+"_HOSTNAME"]).
							Should(Equal(stsName + "-" + strconv.Itoa(int(params.Component.GetPrimaryIndex())) + "-0." + svcName))
					}
				}
			}

			By("Checking env values with primaryIndex=0 ")
			var mockPrimaryIndex = int32(testapps.DefaultReplicationPrimaryIndex)
			params.Component.PrimaryIndex = &mockPrimaryIndex
			checkEnvValues()

			By("Checking env values with primaryIndex=1 ")
			params.Component.Replicas = 2
			var newPrimaryIndex = int32(1)
			params.Component.PrimaryIndex = &newPrimaryIndex
			checkEnvValues()
		})

		It("builds BackupPolicy correctly", func() {
			sts := newStsObj()
			backupPolicyTemplate := newBackupPolicyTemplate()
			backupKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-backup",
			}
			policy, err := BuildBackupPolicy(sts, backupPolicyTemplate, backupKey)
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
			backupJob, err := BuildBackup(sts, backupPolicyName, backupJobKey)
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
			vs, err := BuildVolumeSnapshot(snapshotKey, pvcName, sts)
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
			cronJob, err := BuildCronJob(pvcKey, schedule, sts)
			Expect(err).Should(BeNil())
			Expect(cronJob).ShouldNot(BeNil())
		})

		It("builds ConfigMap with template correctly", func() {
			config := map[string]string{}
			params := newParams()
			tplCfg := appsv1alpha1.ComponentConfigSpec{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "test-config-tpl",
					TemplateRef: "test-config-tpl",
				},
				ConfigConstraintRef: "test-config-constraint",
			}
			configmap, err := BuildConfigMapWithTemplate(config, *params, "test-cm", tplCfg.ConfigConstraintRef, tplCfg.ComponentTemplateSpec)
			Expect(err).Should(BeNil())
			Expect(configmap).ShouldNot(BeNil())
		})

		It("builds config manager sidecar container correctly", func() {
			sidecarRenderedParam := &cfgcm.CfgManagerBuildParams{
				ManagerName:   "cfgmgr",
				CharacterType: "mysql",
				SecreteName:   "test-secret",
				Image:         constant.KBToolsImage,
				Args:          []string{},
				Envs:          []corev1.EnvVar{},
				Volumes:       []corev1.VolumeMount{},
			}
			configmap, err := BuildCfgManagerContainer(sidecarRenderedParam)
			Expect(err).Should(BeNil())
			Expect(configmap).ShouldNot(BeNil())
		})
	})

})
