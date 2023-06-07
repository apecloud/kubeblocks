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

package builder

import (
	"fmt"
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
	const mysqlCompDefName = "replicasets"
	const mysqlCompName = "mysql"
	const proxyCompDefName = "proxy"
	var requiredKeys = []string{
		"KB_REPLICA_COUNT",
		"KB_0_HOSTNAME",
		"KB_CLUSTER_UID",
	}

	allFieldsClusterDefObj := func(needCreate bool) *appsv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
			AddComponentDef(testapps.StatelessNginxComponent, proxyCompDefName).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterDefObj)).Should(Succeed())
		}
		return clusterDefObj
	}

	allFieldsClusterVersionObj := func(needCreate bool) *appsv1alpha1.ClusterVersion {
		By("By assure an clusterVersion obj")
		clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponentVersion(mysqlCompDefName).
			AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			AddComponentVersion(proxyCompDefName).
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
		// setup Cluster obj requires default ClusterDefinition and ClusterVersion objects
		if clusterDefObj == nil {
			clusterDefObj = allFieldsClusterDefObj(needCreate)
		}
		if clusterVersionObj == nil {
			clusterVersionObj = allFieldsClusterVersionObj(needCreate)
		}
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompDefName).SetReplicas(1).
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
		component, err := component.BuildComponent(
			reqCtx,
			*cluster,
			*clusterDef,
			clusterDef.Spec.ComponentDefs[0],
			cluster.Spec.ComponentSpecs[0],
			&clusterVersion.Spec.ComponentVersions[0])
		Expect(err).Should(Succeed())
		Expect(component).ShouldNot(BeNil())
		return component
	}
	newClusterObjs := func(clusterDefObj *appsv1alpha1.ClusterDefinition) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.Cluster, *component.SynthesizedComponent) {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(clusterDefObj, nil, false)
		synthesizedComponent := newAllFieldsComponent(clusterDef, clusterVersion)
		return clusterDef, cluster, synthesizedComponent
	}

	Context("has helper function which builds specific object from cue template", func() {
		It("builds PVC correctly", func() {
			snapshotName := "test-snapshot-name"
			sts := newStsObj()
			_, _, synthesizedComponent := newClusterObjs(nil)
			pvcKey := types.NamespacedName{
				Namespace: "default",
				Name:      "data-mysql-01-replicasets-0",
			}
			pvc, err := BuildPVCFromSnapshot(sts, synthesizedComponent.VolumeClaimTemplates[0], pvcKey, snapshotName, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(pvc).ShouldNot(BeNil())
			Expect(pvc.Spec.AccessModes).Should(Equal(sts.Spec.VolumeClaimTemplates[0].Spec.AccessModes))
			Expect(pvc.Spec.Resources).Should(Equal(synthesizedComponent.VolumeClaimTemplates[0].Spec.Resources))
			Expect(pvc.Labels[constant.VolumeTypeLabelKey]).ShouldNot(BeEmpty())
		})

		It("builds Service correctly", func() {
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			svcList, err := BuildSvcListWithCustomAttributes(cluster, synthesizedComponent, nil)
			Expect(err).Should(BeNil())
			Expect(svcList).ShouldNot(BeEmpty())
		})

		It("builds Conn. Credential correctly", func() {
			var (
				clusterDefObj                             = testapps.NewClusterDefFactoryWithConnCredential("conn-cred").GetObject()
				clusterDef, cluster, synthesizedComponent = newClusterObjs(clusterDefObj)
			)
			credential, err := BuildConnCredential(clusterDef, cluster, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(credential).ShouldNot(BeNil())
			Expect(credential.Labels["apps.kubeblocks.io/cluster-type"]).Should(BeEmpty())
			By("setting type")
			characterType := "test-character-type"
			clusterDef.Spec.Type = characterType
			credential, err = BuildConnCredential(clusterDef, cluster, synthesizedComponent)
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
			svcFQDN := fmt.Sprintf("%s-%s.%s.svc", cluster.Name, synthesizedComponent.Name, cluster.Namespace)
			headlessSvcFQDN := fmt.Sprintf("%s-%s-headless.%s.svc", cluster.Name, synthesizedComponent.Name, cluster.Namespace)
			var mysqlPort corev1.ServicePort
			var paxosPort corev1.ServicePort
			for _, s := range synthesizedComponent.Services[0].Spec.Ports {
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
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			envConfigName := "test-env-config-name"

			sts, err := BuildSts(reqCtx, cluster, synthesizedComponent, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
			// test  replicas = 0
			newComponent := *synthesizedComponent
			newComponent.Replicas = 0
			sts, err = BuildSts(reqCtx, cluster, &newComponent, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
			Expect(*sts.Spec.Replicas).Should(Equal(int32(0)))
			Expect(sts.Spec.VolumeClaimTemplates[0].Labels[constant.VolumeTypeLabelKey]).
				Should(Equal(string(appsv1alpha1.VolumeTypeData)))
			// test workload type replication
			replComponent := *synthesizedComponent
			replComponent.Replicas = 2
			replComponent.WorkloadType = appsv1alpha1.Replication
			sts, err = BuildSts(reqCtx, cluster, &replComponent, envConfigName)
			Expect(err).Should(BeNil())
			Expect(sts).ShouldNot(BeNil())
			Expect(*sts.Spec.Replicas).Should(BeEquivalentTo(2))
		})

		It("builds Deploy correctly", func() {
			reqCtx := newReqCtx()
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			deploy, err := BuildDeploy(reqCtx, cluster, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(deploy).ShouldNot(BeNil())
		})

		It("builds PDB correctly", func() {
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			pdb, err := BuildPDB(cluster, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(pdb).ShouldNot(BeNil())
		})

		It("builds Env Config correctly", func() {
			reqCtx := newReqCtx()
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			cfg, err := BuildEnvConfig(reqCtx, k8sClient, cluster, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(cfg).ShouldNot(BeNil())
			for _, k := range requiredKeys {
				_, ok := cfg.Data[k]
				Expect(ok).Should(BeTrue())
			}
		})

		It("builds env config with resources recreate", func() {
			reqCtx := newReqCtx()
			_, cluster, synthesizedComponent := newClusterObjs(nil)

			uuid := "12345"
			By("mock a cluster uuid")
			cluster.UID = types.UID(uuid)

			cfg, err := BuildEnvConfig(reqCtx, k8sClient, cluster, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(cfg).ShouldNot(BeNil())
			Expect(cfg.Data["KB_CLUSTER_UID"]).Should(Equal(uuid))
		})

		It("builds Env Config with ConsensusSet status correctly", func() {
			reqCtx := newReqCtx()
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			cluster.Status.Components = map[string]appsv1alpha1.ClusterComponentStatus{
				synthesizedComponent.Name: {
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
			cfg, err := BuildEnvConfig(reqCtx, k8sClient, cluster, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(cfg).ShouldNot(BeNil())
			toCheckKeys := append(requiredKeys, []string{
				"KB_LEADER",
				"KB_FOLLOWERS",
			}...)
			for _, k := range toCheckKeys {
				_, ok := cfg.Data[k]
				Expect(ok).Should(BeTrue())
			}
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
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			tplCfg := appsv1alpha1.ComponentConfigSpec{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "test-config-tpl",
					TemplateRef: "test-config-tpl",
				},
				ConfigConstraintRef: "test-config-constraint",
			}
			configmap, err := BuildConfigMapWithTemplate(cluster, synthesizedComponent, config,
				"test-cm", tplCfg.ConfigConstraintRef, tplCfg.ComponentTemplateSpec)
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
