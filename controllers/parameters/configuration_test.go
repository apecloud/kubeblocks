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

package parameters

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/pkg/parameters/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
	"github.com/apecloud/kubeblocks/test/testdata"
)

const (
	compDefName      = "test-compdef"
	clusterName      = "test-cluster"
	defaultCompName  = "mysql"
	shardingCompName = "sharding-test"
	defaultITSName   = "mysql-statefulset"
	configSpecName   = "mysql-config-tpl"
	configVolumeName = "mysql-config"
	cmName           = "mysql-tree-node-template-8.0"
	paramsDefName    = "mysql-params-def"
	pdcrName         = "config-test-pdcr"
	envTestFileKey   = "env_test"
)

func mockSchemaData() string {
	cue, _ := testdata.GetTestDataFileContent("cue_testdata/wesql.cue")
	return string(cue)
}

type mockResourceConfiguration struct {
	Template func(*corev1.ConfigMap)
	Vars     []appsv1.EnvVar
}

func mockConfigResource(options ...mockResourceConfiguration) (*corev1.ConfigMap, *parametersv1alpha1.ParametersDefinition) {
	By("Create a config template obj")
	factory := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
		AddLabels(
			constant.AppNameLabelKey, clusterName,
			constant.AppInstanceLabelKey, clusterName,
			constant.KBAppComponentLabelKey, defaultCompName,
			constant.CMConfigurationTemplateNameLabelKey, configSpecName,
			constant.CMConfigurationConstraintsNameLabelKey, cmName,
			constant.CMConfigurationSpecProviderLabelKey, configSpecName,
			constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
		).
		AddAnnotations(
			constant.KBParameterUpdateSourceAnnotationKey, constant.ReconfigureManagerSource,
			constant.ConfigurationRevision, "1",
			constant.CMInsEnableRerenderTemplateKey, "true").
		AddConfigFile(envTestFileKey, "abcde=1234")
	for _, option := range options {
		if option.Template != nil {
			option.Template(factory.GetObject())
		}
	}
	configmap := factory.Create(&testCtx).GetObject()

	By("Create a parameters definition obj")
	paramsdef := testparameters.NewParametersDefinitionFactory(paramsDefName).
		SetReloadAction(testparameters.WithNoneAction()).
		Schema(mockSchemaData()).
		Create(&testCtx).
		GetObject()

	Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(paramsdef), func(g Gomega, def *parametersv1alpha1.ParametersDefinition) {
		g.Expect(def.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
	})).Should(Succeed())
	return configmap, paramsdef
}

func mockReconcileResource(options ...mockResourceConfiguration) (*corev1.ConfigMap, *parametersv1alpha1.ParametersDefinition, *appsv1.Cluster, *appsv1.Component, *component.SynthesizedComponent) {
	configmap, paramsDef := mockConfigResource(options...)

	By("Create a component definition obj and mock to available")
	compDefFactory := testapps.NewComponentDefinitionFactory(compDefName).
		WithRandomName().
		SetDefaultSpec().
		AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, true)
	for _, option := range options {
		for _, v := range option.Vars {
			compDefFactory.AddVar(v)
		}
	}
	compDefObj := compDefFactory.Create(&testCtx).GetObject()
	Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
		obj.Status.Phase = appsv1.AvailablePhase
	})()).Should(Succeed())

	pdcr := testparameters.NewParamConfigRendererFactory(pdcrName).
		SetParametersDefs(paramsDef.GetName()).
		SetComponentDefinition(compDefObj.GetName()).
		SetTemplateName(configSpecName).
		Create(&testCtx).
		GetObject()
	Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pdcr), func(obj *parametersv1alpha1.ParamConfigRenderer) {
		obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
	})()).Should(Succeed())

	By("Creating a cluster")
	clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
		AddComponent(defaultCompName, compDefObj.GetName()).
		AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
		AddSharding(shardingCompName, "", compDefObj.GetName()).
		SetShards(5).
		Create(&testCtx).
		GetObject()

	By("Create a component obj")
	fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
	compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
		AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
		AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
		AddLabelsInMap(constant.GetClusterLabels(clusterName)).
		SetUID(types.UID(fmt.Sprintf("%s-%s", clusterObj.Name, "test-uid"))).
		SetReplicas(1).
		Create(&testCtx).
		GetObject()

	container := *builder.NewContainerBuilder("mock-container").
		AddVolumeMounts(corev1.VolumeMount{
			Name:      configVolumeName,
			MountPath: "/mnt/config",
		}).GetObject()
	_ = testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, defaultITSName, clusterObj.Name, defaultCompName).
		AddConfigmapVolume(configVolumeName, configmap.Name).
		AddContainer(container).
		AddAppNameLabel(clusterName).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(defaultCompName).
		Create(&testCtx).GetObject()

	synthesizedComp, err := component.BuildSynthesizedComponent(testCtx.Ctx, testCtx.Cli, compDefObj, compObj)
	Expect(err).ShouldNot(HaveOccurred())

	return configmap, paramsDef, clusterObj, compObj, synthesizedComp
}

func cleanEnv() {
	// must wait till resources deleted and no longer existed before the testcases start,
	// otherwise if later it needs to create some new resource objects with the same name,
	// in race conditions, it will find the existence of old objects, resulting failure to
	// create the new objects.
	By("clean resources")

	// delete cluster(and all dependent sub-resources), cluster definition
	testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

	// delete rest mocked objects
	inNS := client.InNamespace(testCtx.DefaultNamespace)
	ml := client.HasLabels{testCtx.TestObjLabelKey}
	// non-namespaced
	testapps.ClearResources(&testCtx, generics.ParametersDefinitionSignature, ml)
	testapps.ClearResources(&testCtx, generics.ParamConfigRendererSignature, ml)
	testapps.ClearResources(&testCtx, generics.ComponentDefinitionSignature, ml)
	// namespaced
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.SecretSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentParameterSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ParameterSignature, true, inNS, ml)
}

func resourceTestComponent(name, def string) *appsv1.Component {
	return &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "test-" + name, Namespace: "ns",
		Labels: constant.GetClusterLabels("test")}, Spec: appsv1.ComponentSpec{CompDef: def,
		Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
		}}},
	}}
}

func storageVar(def string) appsv1.EnvVar {
	return appsv1.EnvVar{Name: "DISK", ValueFrom: &appsv1.VarSource{ResourceVarRef: &appsv1.ResourceVarSelector{
		ClusterObjectReference: appsv1.ClusterObjectReference{CompDef: def},
		ResourceVars:           appsv1.ResourceVars{Storage: &appsv1.NamedVar{Name: "data"}},
	}}}
}

var _ = Describe("Automatic resource rendering", func() {
	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	waitFinished := func(comp *appsv1.Component, content string, afterGeneration ...int64) int64 {
		key := client.ObjectKey{Namespace: comp.Namespace, Name: core.GenerateComponentConfigurationName(clusterName, defaultCompName)}
		var generation int64
		Eventually(func(g Gomega) {
			cp := &parametersv1alpha1.ComponentParameter{}
			g.Expect(testCtx.Cli.Get(testCtx.Ctx, key, cp)).To(Succeed())
			if len(afterGeneration) > 0 {
				g.Expect(cp.Generation).To(BeNumerically(">", afterGeneration[0]))
			}
			g.Expect(cp.Status.ObservedGeneration).To(Equal(cp.Generation))
			item := parameters.GetItemStatus(&cp.Status, configSpecName)
			g.Expect(item).NotTo(BeNil())
			g.Expect(item.Phase).To(Equal(parametersv1alpha1.CFinishedPhase))
			g.Expect(item.UpdateRevision).To(Equal(strconv.FormatInt(cp.Generation, 10)))
			cm := &corev1.ConfigMap{}
			cmKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
			g.Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
			g.Expect(cm.Data[testparameters.MysqlConfigFile]).To(ContainSubstring(content))
			g.Expect(cm.Annotations[constant.ConfigurationRevision]).To(Equal(item.UpdateRevision))
			generation = cp.Generation
		}).Should(Succeed())
		return generation
	}

	It("rerenders storage vars and built-in functions without trigger declarations", func() {
		_, _, _, comp, _ := mockReconcileResource(mockResourceConfiguration{
			Vars: []appsv1.EnvVar{storageVar("")},
			Template: func(cm *corev1.ConfigMap) {
				cm.Data[testparameters.MysqlConfigFile] = strings.ReplaceAll(cm.Data[testparameters.MysqlConfigFile], "server-id=1", "server-id={{ div (int64 $.DISK) 1073741824 }}")
				cm.Data[envTestFileKey] = `capacity={{ getComponentPVCSizeByName $.component "data" }}`
			},
		})
		waitFinished(comp, "server-id=0")
		expand := func(size string) {
			// This suite runs parameter controllers; emulate the Component update that
			// the apps controller normally propagates from a VolumeExpansion operation.
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(c *appsv1.Component) {
				c.Spec.VolumeClaimTemplates = resourceTestComponent("db", "db").Spec.VolumeClaimTemplates
				c.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(size)
			})()).To(Succeed())
		}
		expand("2Gi")
		first := waitFinished(comp, "server-id=2")
		cm := &corev1.ConfigMap{}
		cmKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
		Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
		Expect(cm.Data[envTestFileKey]).To(Equal("capacity=2147483648"))
		expand("3Gi")
		second := waitFinished(comp, "server-id=3")
		Expect(second).To(BeNumerically(">", first))
		expand("3072Mi")
		Consistently(func() int64 {
			cp := &parametersv1alpha1.ComponentParameter{}
			Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKey{Namespace: comp.Namespace, Name: comp.Name}, cp)).To(Succeed())
			return cp.Generation
		}, time.Second*2, time.Millisecond*100).Should(Equal(second))
	})

	It("applies storage-derived configuration through restart and preserves explicit parameters", func() {
		_, pd, cluster, comp, _ := mockReconcileResource(mockResourceConfiguration{
			Vars: []appsv1.EnvVar{storageVar("")},
			Template: func(cm *corev1.ConfigMap) {
				cm.Data[testparameters.MysqlConfigFile] = strings.ReplaceAll(cm.Data[testparameters.MysqlConfigFile], "server-id=1", "server-id={{ div (int64 $.DISK) 1073741824 }}")
			},
		})
		waitFinished(comp, "server-id=0")
		By("mount the rendered configuration and select the real restart policy")
		cmKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
		its := &workloads.InstanceSet{}
		itsKey := client.ObjectKey{Namespace: comp.Namespace, Name: defaultITSName}
		Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(obj *workloads.InstanceSet) {
			obj.Spec.Template.Spec.Volumes[0].ConfigMap.Name = cmKey.Name
		})()).To(Succeed())
		Expect(testCtx.Cli.Get(testCtx.Ctx, itsKey, its)).To(Succeed())
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(pd), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Spec.ReloadAction = nil
		})()).To(Succeed())

		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: its.Name + "-0", Namespace: its.Namespace, Labels: its.Spec.Selector.MatchLabels,
		}, Spec: *its.Spec.Template.Spec.DeepCopy()}
		pod.Spec.Containers[0].Image = "test:latest"
		Expect(testCtx.Cli.Create(testCtx.Ctx, pod)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(testCtx.Cli.Delete(testCtx.Ctx, pod))).To(Succeed()) })
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pod), func(obj *corev1.Pod) {
			obj.Status.Phase = corev1.PodRunning
			obj.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		})()).To(Succeed())
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(comp), func(obj *appsv1.Component) {
			obj.Status.Phase = appsv1.RunningComponentPhase
		})()).To(Succeed())

		restartKey := core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configSpecName)
		completeRestart := func(content, previousVersion string) string {
			var version string
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
				g.Expect(cm.Data[testparameters.MysqlConfigFile]).To(ContainSubstring(content))
				hash, err := util.ComputeHash(cm.Data)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(hash).NotTo(Equal(previousVersion))
				g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
				g.Expect(cluster.Spec.ComponentSpecs[0].Annotations[restartKey]).To(Equal(hash))
				version = hash
			}).Should(Succeed())
			cp := &parametersv1alpha1.ComponentParameter{}
			Eventually(func(g Gomega) {
				g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), cp)).To(Succeed())
				item := parameters.GetItemStatus(&cp.Status, configSpecName)
				g.Expect(item).NotTo(BeNil())
				g.Expect(item.UpdateRevision).To(Equal(strconv.FormatInt(cp.Generation, 10)))
				g.Expect(item.Phase).NotTo(Equal(parametersv1alpha1.CFinishedPhase))
			}).Should(Succeed())
			// Parameter controllers issue the restart. Emulate the workload
			// controller completing it by acknowledging the version on the Pod.
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(pod), func(obj *corev1.Pod) {
				obj.Annotations = map[string]string{restartKey: version}
			})()).To(Succeed())
			waitFinished(comp, content)
			Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), cp)).To(Succeed())
			detail := parameters.GetItemStatus(&cp.Status, configSpecName).ReconcileDetail
			Expect(detail).NotTo(BeNil())
			Expect(detail.Policy).To(Equal(string(parametersv1alpha1.RestartPolicy)))
			Expect(detail.SucceedCount).To(BeNumerically("==", 1))
			Expect(detail.ExpectedCount).To(BeNumerically("==", 1))
			cm := &corev1.ConfigMap{}
			Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
			applied, err := json.Marshal(cm.Data)
			Expect(err).NotTo(HaveOccurred())
			Expect(cm.Annotations[constant.LastAppliedConfigAnnotationKey]).To(Equal(string(applied)))
			return version
		}

		expand := func(size string) {
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(c *appsv1.Component) {
				c.Spec.VolumeClaimTemplates = resourceTestComponent("db", "db").Spec.VolumeClaimTemplates
				c.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(size)
			})()).To(Succeed())
		}
		expand("2Gi")
		version := completeRestart("server-id=2", "")
		By("preserve a user override through subsequent expansions")
		override := "9"
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(cp *parametersv1alpha1.ComponentParameter) {
			item := parameters.GetConfigTemplateItem(&cp.Spec, configSpecName)
			item.ConfigFileParams = map[string]parametersv1alpha1.ParametersInFile{testparameters.MysqlConfigFile: {Parameters: map[string]*string{"server-id": &override}}}
		})()).To(Succeed())
		version = completeRestart("server-id=9", version)
		before := waitFinished(comp, "server-id=9")
		cm := &corev1.ConfigMap{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
		data := cm.DeepCopy().Data
		expand("3Gi")
		after := waitFinished(comp, "server-id=9", before)
		Expect(after).To(BeNumerically(">", before))
		Expect(testCtx.Cli.Get(testCtx.Ctx, cmKey, cm)).To(Succeed())
		Expect(cm.Data).To(Equal(data))
		Expect(fmt.Sprint(after)).To(Equal(cm.Annotations[constant.ConfigurationRevision]))
		cp := &parametersv1alpha1.ComponentParameter{}
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), cp)).To(Succeed())
		detail := parameters.GetItemStatus(&cp.Status, configSpecName).ReconcileDetail
		Expect(detail).NotTo(BeNil())
		Expect(detail.ErrMessage).To(Equal(configurationNoChangedMessage))
		Expect(detail.Policy).To(BeEmpty())
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
		Expect(cluster.Spec.ComponentSpecs[0].Annotations[restartKey]).To(Equal(version))
	})
	It("refreshes sharding payload when only Cluster shard count changes", func() {
		_, _, _, comp, _ := mockReconcileResource()
		waitFinished(comp, "server-id=1")
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(c *appsv1.Component) {
			c.Labels[constant.KBAppShardingNameLabelKey] = shardingCompName
		})()).To(Succeed())
		waitShards := func(shards string) int64 {
			var generation int64
			Eventually(func(g Gomega) {
				cp := &parametersv1alpha1.ComponentParameter{}
				g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), cp)).To(Succeed())
				item := parameters.GetConfigTemplateItem(&cp.Spec, configSpecName)
				g.Expect(item).NotTo(BeNil())
				g.Expect(string(item.Payload[constant.ShardingPayload])).To(ContainSubstring(`"shards":"` + shards + `"`))
				g.Expect(cp.Status.ObservedGeneration).To(Equal(cp.Generation))
				generation = cp.Generation
			}).Should(Succeed())
			return generation
		}
		before := waitShards("5")
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), comp)).To(Succeed())
		componentGeneration := comp.Generation
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKey{Namespace: comp.Namespace, Name: clusterName}, func(c *appsv1.Cluster) {
			c.Spec.Shardings[0].Shards = 6
		})()).To(Succeed())
		Expect(waitShards("6")).To(BeNumerically(">", before))
		Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(comp), comp)).To(Succeed())
		Expect(comp.Generation).To(Equal(componentGeneration))
	})

})
