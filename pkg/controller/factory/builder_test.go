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

package factory

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("builder", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterName = "test-cluster"
	const mysqlCompDefName = "replicasets"
	const proxyCompDefName = "proxy"
	const mysqlCompName = "mysql"
	const mysqlCharacterType = "mysql"

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

	newExtraEnvs := func() map[string]string {
		jsonStr, _ := json.Marshal(map[string]string{
			"mock-key": "mock-value",
		})
		return map[string]string{
			constant.ExtraEnvAnnotationKey: string(jsonStr),
		}
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
			AddAnnotationsInMap(newExtraEnvs()).
			AddComponent(mysqlCompName, mysqlCompDefName).SetReplicas(1).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			AddComponentService(testapps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
			AddComponentService(testapps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
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
	newAllFieldsComponent := func(clusterDef *appsv1alpha1.ClusterDefinition,
		clusterVer *appsv1alpha1.ClusterVersion, cluster *appsv1alpha1.Cluster) *component.SynthesizedComponent {
		reqCtx := newReqCtx()
		By("assign every available fields")
		synthesizeComp, err := component.BuildSynthesizedComponentWrapper4Test(reqCtx, testCtx.Cli,
			clusterDef, clusterVer, cluster, &cluster.Spec.ComponentSpecs[0])
		Expect(err).Should(Succeed())
		Expect(synthesizeComp).ShouldNot(BeNil())
		return synthesizeComp
	}
	newClusterObjs := func(clusterDefObj *appsv1alpha1.ClusterDefinition) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.Cluster, *component.SynthesizedComponent) {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(clusterDefObj, nil, false)
		synthesizedComponent := newAllFieldsComponent(clusterDef, clusterVersion, cluster)
		return clusterDef, cluster, synthesizedComponent
	}

	Context("has helper function which builds specific object from cue template", func() {
		It("builds PVC correctly", func() {
			snapshotName := "test-snapshot-name"
			sts := newStsObj()
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			pvcKey := types.NamespacedName{
				Namespace: "default",
				Name:      "data-mysql-01-replicasets-0",
			}
			pvc := BuildPVC(cluster, synthesizedComponent, &synthesizedComponent.VolumeClaimTemplates[0], pvcKey, snapshotName)
			Expect(pvc).ShouldNot(BeNil())
			Expect(pvc.Spec.AccessModes).Should(Equal(sts.Spec.VolumeClaimTemplates[0].Spec.AccessModes))
			Expect(pvc.Spec.Resources).Should(Equal(synthesizedComponent.VolumeClaimTemplates[0].Spec.Resources))
			Expect(pvc.Spec.StorageClassName).Should(Equal(synthesizedComponent.VolumeClaimTemplates[0].Spec.StorageClassName))
			Expect(pvc.Labels[constant.VolumeTypeLabelKey]).ShouldNot(BeEmpty())
		})

		It("builds Conn. Credential correctly", func() {
			var (
				clusterDefObj                             = testapps.NewClusterDefFactoryWithConnCredential("conn-cred", mysqlCompDefName).GetObject()
				clusterDef, cluster, synthesizedComponent = newClusterObjs(clusterDefObj)
			)
			credential := BuildConnCredential(clusterDef, cluster, synthesizedComponent)
			Expect(credential).ShouldNot(BeNil())
			Expect(credential.Labels[constant.KBAppClusterDefTypeLabelKey]).Should(BeEmpty())
			By("setting type")
			characterType := "test-character-type"
			clusterDef.Spec.Type = characterType
			credential = BuildConnCredential(clusterDef, cluster, synthesizedComponent)
			Expect(credential).ShouldNot(BeNil())
			Expect(credential.Labels[constant.KBAppClusterDefTypeLabelKey]).Should(Equal(characterType))
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
			svcFQDN := fmt.Sprintf("%s-%s", cluster.Name, synthesizedComponent.Name)
			headlessSvcFQDN := fmt.Sprintf("%s-%s-headless", cluster.Name, synthesizedComponent.Name)
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

		It("builds Conn. Credential during restoring from backup", func() {
			originalPassword := "test-passw0rd"
			encryptionKey := "encryptionKey"
			viper.Set(constant.CfgKeyDPEncryptionKey, encryptionKey)
			var (
				clusterDefObj                             = testapps.NewClusterDefFactoryWithConnCredential("conn-cred", mysqlCompDefName).GetObject()
				clusterDef, cluster, synthesizedComponent = newClusterObjs(clusterDefObj)
			)
			e := intctrlutil.NewEncryptor(encryptionKey)
			ciphertext, _ := e.Encrypt([]byte(originalPassword))
			cluster.Annotations[constant.RestoreFromBackupAnnotationKey] = fmt.Sprintf(`{"%s":{"%s":"%s"}}`,
				synthesizedComponent.Name, constant.ConnectionPassword, ciphertext)
			credential := BuildConnCredential(clusterDef, cluster, synthesizedComponent)
			Expect(credential).ShouldNot(BeNil())
			Expect(credential.StringData["RANDOM_PASSWD"]).Should(Equal(originalPassword))
		})

		It("builds RSM correctly", func() {
			clusterDef, cluster, synthesizedComponent := newClusterObjs(nil)

			rsm, err := BuildRSM(cluster, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(rsm).ShouldNot(BeNil())

			By("set replicas = 0")
			newComponent := *synthesizedComponent
			newComponent.Replicas = 0
			rsm, err = BuildRSM(cluster, &newComponent)
			Expect(err).Should(BeNil())
			Expect(rsm).ShouldNot(BeNil())
			Expect(*rsm.Spec.Replicas).Should(Equal(int32(0)))
			Expect(rsm.Spec.VolumeClaimTemplates[0].Labels[constant.VolumeTypeLabelKey]).
				Should(Equal(string(appsv1alpha1.VolumeTypeData)))

			By("set workload type to Replication")
			clusterDef.Spec.ComponentDefs[0].WorkloadType = appsv1alpha1.Replication
			cluster.Spec.ComponentSpecs[0].Replicas = 2
			replComponent := newAllFieldsComponent(clusterDef, nil, cluster)
			rsm, err = BuildRSM(cluster, replComponent)
			Expect(err).Should(BeNil())
			Expect(rsm).ShouldNot(BeNil())
			Expect(*rsm.Spec.Replicas).Should(BeEquivalentTo(2))
			// test extra envs
			Expect(rsm.Spec.Template.Spec.Containers).ShouldNot(BeEmpty())
			for _, container := range rsm.Spec.Template.Spec.Containers {
				isContainEnv := false
				for _, env := range container.Env {
					if env.Name == "mock-key" && env.Value == "mock-value" {
						isContainEnv = true
						break
					}
				}
				Expect(isContainEnv).Should(BeTrue())
			}

			// test roles
			Expect(rsm.Spec.Roles).Should(HaveLen(2))
			for _, roleName := range []string{constant.Primary, constant.Secondary} {
				Expect(slices.IndexFunc(rsm.Spec.Roles, func(role workloads.ReplicaRole) bool {
					return role.Name == roleName
				})).Should(BeNumerically(">", -1))
			}

			// test role probe
			Expect(rsm.Spec.RoleProbe).ShouldNot(BeNil())

			// test member update strategy
			Expect(rsm.Spec.MemberUpdateStrategy).ShouldNot(BeNil())
			Expect(*rsm.Spec.MemberUpdateStrategy).Should(BeEquivalentTo(workloads.SerialUpdateStrategy))

			By("set workload type to Consensus")
			clusterDef.Spec.ComponentDefs[0].WorkloadType = appsv1alpha1.Consensus
			clusterDef.Spec.ComponentDefs[0].CharacterType = mysqlCharacterType
			clusterDef.Spec.ComponentDefs[0].ConsensusSpec = appsv1alpha1.NewConsensusSetSpec()
			clusterDef.Spec.ComponentDefs[0].ConsensusSpec.UpdateStrategy = appsv1alpha1.BestEffortParallelStrategy
			cluster.Spec.ComponentSpecs[0].Replicas = 3
			csComponent := newAllFieldsComponent(clusterDef, nil, cluster)
			rsm, err = BuildRSM(cluster, csComponent)
			Expect(err).Should(BeNil())
			Expect(rsm).ShouldNot(BeNil())

			// test roles
			Expect(rsm.Spec.Roles).Should(HaveLen(1))
			Expect(rsm.Spec.Roles[0].Name).Should(Equal(appsv1alpha1.DefaultLeader.Name))

			// test role probe
			Expect(rsm.Spec.RoleProbe).ShouldNot(BeNil())

			// test member update strategy
			Expect(rsm.Spec.MemberUpdateStrategy).ShouldNot(BeNil())
			Expect(*rsm.Spec.MemberUpdateStrategy).Should(BeEquivalentTo(workloads.BestEffortParallelUpdateStrategy))
		})

		It("builds PDB correctly", func() {
			_, _, synthesizedComponent := newClusterObjs(nil)
			pdb := BuildPDB(synthesizedComponent)
			Expect(pdb).ShouldNot(BeNil())
		})

		It("builds BackupJob correctly", func() {
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			backupJobKey := types.NamespacedName{
				Namespace: "default",
				Name:      "test-backup-job",
			}
			backupPolicyName := "test-backup-policy"
			backupJob := BuildBackup(cluster, synthesizedComponent, backupPolicyName, backupJobKey, "snapshot")
			Expect(backupJob).ShouldNot(BeNil())
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
			configmap := BuildConfigMapWithTemplate(cluster, synthesizedComponent, config,
				"test-cm", tplCfg.ComponentTemplateSpec)
			Expect(configmap).ShouldNot(BeNil())
		})

		It("builds config manager sidecar container correctly", func() {
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			sidecarRenderedParam := &cfgcm.CfgManagerBuildParams{
				ManagerName:   "cfgmgr",
				SecreteName:   "test-secret",
				ComponentName: synthesizedComponent.Name,
				CharacterType: synthesizedComponent.CharacterType,
				Image:         constant.KBToolsImage,
				Args:          []string{},
				Envs:          []corev1.EnvVar{},
				Volumes:       []corev1.VolumeMount{},
				Cluster:       cluster,
			}
			configmap, err := BuildCfgManagerContainer(sidecarRenderedParam, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(configmap).ShouldNot(BeNil())
			Expect(configmap.SecurityContext).Should(BeNil())
		})

		It("builds config manager sidecar container correctly", func() {
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			sidecarRenderedParam := &cfgcm.CfgManagerBuildParams{
				ManagerName:           "cfgmgr",
				CharacterType:         mysqlCharacterType,
				SecreteName:           "test-secret",
				Image:                 constant.KBToolsImage,
				ShareProcessNamespace: true,
				Args:                  []string{},
				Envs:                  []corev1.EnvVar{},
				Volumes:               []corev1.VolumeMount{},
				Cluster:               cluster,
			}
			configmap, err := BuildCfgManagerContainer(sidecarRenderedParam, synthesizedComponent)
			Expect(err).Should(BeNil())
			Expect(configmap).ShouldNot(BeNil())
			Expect(configmap.SecurityContext).ShouldNot(BeNil())
			Expect(configmap.SecurityContext.RunAsUser).ShouldNot(BeNil())
			Expect(*configmap.SecurityContext.RunAsUser).Should(BeEquivalentTo(int64(0)))
		})

		It("builds restore job correctly", func() {
			key := types.NamespacedName{Name: "restore", Namespace: "default"}
			volumes := []corev1.Volume{}
			volumeMounts := []corev1.VolumeMount{}
			env := []corev1.EnvVar{}
			component := &component.SynthesizedComponent{
				Name: mysqlCompName,
			}
			cluster := &appsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace},
				Spec: appsv1alpha1.ClusterSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "testKey",
							Value:    "testValue",
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			}
			job, err := BuildRestoreJob(cluster, component, key.Name, "", []string{"sh"}, volumes, volumeMounts, env, nil)
			Expect(err).Should(BeNil())
			Expect(job).ShouldNot(BeNil())
			Expect(job.Name).Should(Equal(key.Name))
			Expect(len(job.Spec.Template.Spec.Tolerations) > 0).Should(BeTrue())
			Expect(job.Spec.Template.Spec.Tolerations[0].Key).Should(Equal("testKey"))
		})

		It("builds volume snapshot class correctly", func() {
			className := "vsc-test"
			driverName := "csi-driver-test"
			obj := BuildVolumeSnapshotClass(className, driverName)
			Expect(obj).ShouldNot(BeNil())
			Expect(obj.Name).Should(Equal(className))
			Expect(obj.Driver).Should(Equal(driverName))
		})

		It("builds cfg manager tools  correctly", func() {
			_, cluster, synthesizedComponent := newClusterObjs(nil)
			cfgManagerParams := &cfgcm.CfgManagerBuildParams{
				ManagerName:               constant.ConfigSidecarName,
				SecreteName:               constant.GenerateDefaultConnCredential(cluster.Name),
				Image:                     viper.GetString(constant.KBToolsImage),
				Cluster:                   cluster,
				ConfigLazyRenderedVolumes: make(map[string]corev1.VolumeMount),
			}
			toolContainers := []appsv1alpha1.ToolConfig{
				{Name: "test-tool", Image: "test-image", Command: []string{"sh"}},
			}

			obj, err := BuildCfgManagerToolsContainer(cfgManagerParams, synthesizedComponent, toolContainers, map[string]cfgcm.ConfigSpecMeta{})
			Expect(err).Should(BeNil())
			Expect(obj).ShouldNot(BeEmpty())
		})

		It("builds serviceaccount correctly", func() {
			_, cluster, _ := newClusterObjs(nil)
			expectName := fmt.Sprintf("kb-%s", cluster.Name)
			sa := BuildServiceAccount(cluster, expectName)
			Expect(sa).ShouldNot(BeNil())
			Expect(sa.Name).Should(Equal(expectName))
		})

		It("builds rolebinding correctly", func() {
			_, cluster, _ := newClusterObjs(nil)
			expectName := fmt.Sprintf("kb-%s", cluster.Name)
			rb := BuildRoleBinding(cluster, expectName)
			Expect(rb).ShouldNot(BeNil())
			Expect(rb.Name).Should(Equal(expectName))
		})

		It("builds clusterrolebinding correctly", func() {
			_, cluster, _ := newClusterObjs(nil)
			expectName := fmt.Sprintf("kb-%s", cluster.Name)
			crb := BuildClusterRoleBinding(cluster, expectName)
			Expect(crb).ShouldNot(BeNil())
			Expect(crb.Name).Should(Equal(expectName))
		})
	})
})
