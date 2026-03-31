/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
	"github.com/apecloud/kubeblocks/test/testdata"
)

const (
	compDefName      = "test-compdef"
	clusterName      = "test-cluster"
	defaultCompName  = "mysql"
	shardingCompName = "sharding-test"
	configSpecName   = "mysql-config-tpl"
	configVolumeName = "mysql-config"
	cmName           = "mysql-tree-node-template-8.0"
	paramsDefName    = "mysql-params-def"
	pdcrName         = "config-test-pdcr"
)

var parameterViewSignature = func(_ parametersv1alpha1.ParameterView, _ *parametersv1alpha1.ParameterView, _ parametersv1alpha1.ParameterViewList, _ *parametersv1alpha1.ParameterViewList) {
}

func mockSchemaData() string {
	cue, _ := testdata.GetTestDataFileContent("cue_testdata/wesql.cue")
	return string(cue)
}

func mockConfigResource() (*corev1.ConfigMap, *parametersv1alpha1.ParametersDefinition) {
	By("Create a config template obj")
	configmap := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
		AddLabels(
			constant.AppNameLabelKey, clusterName,
			constant.AppInstanceLabelKey, clusterName,
			constant.KBAppComponentLabelKey, defaultCompName,
			constant.CMConfigurationTemplateNameLabelKey, configSpecName,
			constant.CMConfigurationConstraintsNameLabelKey, cmName,
			constant.CMConfigurationSpecProviderLabelKey, configSpecName,
			constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
		).
		AddAnnotations(constant.ConfigurationRevision, "1").
		Create(&testCtx).
		GetObject()

	By("Create a parameters definition obj")
	paramsdef := testparameters.NewParametersDefinitionFactory(paramsDefName).
		Schema(mockSchemaData()).
		Create(&testCtx).
		GetObject()

	Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(paramsdef), func(g Gomega, def *parametersv1alpha1.ParametersDefinition) {
		g.Expect(def.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
	})).Should(Succeed())
	return configmap, paramsdef
}

func mockReconcileResource() (*corev1.ConfigMap, *appsv1.Cluster, *appsv1.Component, *component.SynthesizedComponent, *workloads.InstanceSet) {
	configmap, paramsDef := mockConfigResource()

	By("Create a component definition obj and mock to available")
	compDefObj := testapps.NewComponentDefinitionFactory(compDefName).
		WithRandomName().
		SetDefaultSpec().
		AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, true).
		Create(&testCtx).
		GetObject()
	Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
		obj.Status.Phase = appsv1.AvailablePhase
	})()).Should(Succeed())
	Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
		obj.Spec.ComponentDef = compDefObj.GetName()
		obj.Spec.TemplateName = configSpecName
		obj.Spec.FileFormatConfig = &parametersv1alpha1.FileFormatConfig{
			Format: parametersv1alpha1.Ini,
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{SectionName: "mysqld"},
			},
		}
	})()).Should(Succeed())
	By("wait until the current cache can resolve parameter bindings for the component definition")
	Eventually(func(g Gomega) {
		cmpd := &appsv1.ComponentDefinition{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(compDefObj), cmpd)).Should(Succeed())
		configDescs, paramsDefs, err := parameters.ResolveCmpdParametersDefs(testCtx.Ctx, testCtx.Cli, cmpd)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(configDescs).Should(HaveLen(1))
		g.Expect(paramsDefs).Should(HaveLen(1))
		g.Expect(configDescs[0].TemplateName).Should(BeEquivalentTo(configSpecName))
		g.Expect(paramsDefs[0].Name).Should(BeEquivalentTo(paramsDef.Name))
	}).Should(Succeed())

	By("Creating a cluster")
	clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
		AddComponent(defaultCompName, compDefObj.GetName()).
		SetConfig(appsv1.ClusterComponentConfig{
			Name: ptr.To(configSpecName),
		}).
		SetReplicas(1).
		AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
		AddSharding(shardingCompName, "", compDefObj.GetName()).
		SetShards(3).
		SetShardingConfig(appsv1.ClusterComponentConfig{
			Name: ptr.To(configSpecName),
		}).
		AddAnnotations(constant.LegacyConfigManagerRequiredAnnotationKey, "true").
		Create(&testCtx).
		GetObject()

	By("Create a component obj")
	fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
	compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
		AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
		AddAnnotations(constant.CRDAPIVersionAnnotationKey, appsv1.GroupVersion.String()).
		AddLabels(constant.AppInstanceLabelKey, clusterName).
		SetUID(types.UID(fmt.Sprintf("%s-%s", clusterObj.Name, "test-uid"))).
		SetReplicas(1).
		Create(&testCtx).
		GetObject()

	By("Create a ITS obj")
	itsObj := mockCreateITSObject(testCtx.DefaultNamespace, fullCompName, clusterObj.Name, defaultCompName)

	synthesizedComp, err := component.BuildSynthesizedComponent(testCtx.Ctx, testCtx.Cli, compDefObj, compObj)
	Expect(err).ShouldNot(HaveOccurred())

	return configmap, clusterObj, compObj, synthesizedComp, itsObj
}

func mockCreateITSObject(namespace, name, clusterName, compName string) *workloads.InstanceSet {
	container := *builder.NewContainerBuilder("mock-container").
		AddVolumeMounts(corev1.VolumeMount{
			Name:      configVolumeName,
			MountPath: "/mnt/config",
		}).GetObject()
	configManagerContainer := *builder.NewContainerBuilder("config-manager").
		AddPorts(corev1.ContainerPort{
			Name:          "config-manager",
			ContainerPort: 9901,
		}).GetObject()
	itsObj := testapps.NewInstanceSetFactory(namespace, name, clusterName, compName).
		AddContainer(container).
		AddContainer(configManagerContainer).
		SetReplicas(1).
		AddAppNameLabel(clusterName).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(compName).
		Create(&testCtx).
		GetObject()
	return itsObj
}

func mockReconfigureDone(namespace, itsName, configName, configHash string) {
	itsKey := client.ObjectKey{
		Namespace: namespace,
		Name:      itsName,
	}
	Expect(testapps.GetAndChangeObjStatus(&testCtx, itsKey, func(its *workloads.InstanceSet) {
		its.Status.Replicas = int32(1)
		its.Status.InstanceStatus = []workloads.InstanceStatus{
			{
				PodName: fmt.Sprintf("%s-0", its.Name),
				Configs: []workloads.InstanceConfigStatus{
					{
						Name:       configName,
						ConfigHash: ptr.To(configHash),
					},
				},
			},
		}
	})()).Should(Succeed())
}

func waitRenderedConfigHash(namespace, clusterName, componentName, configName string, substrings ...string) string {
	cfgKey := client.ObjectKey{
		Namespace: namespace,
		Name:      parameterscore.GetComponentCfgName(clusterName, componentName, configName),
	}
	var configHash string
	Eventually(testapps.CheckObj(&testCtx, cfgKey, func(g Gomega, cfg *corev1.ConfigMap) {
		content := cfg.Data[testparameters.MysqlConfigFile]
		for _, substring := range substrings {
			g.Expect(content).Should(ContainSubstring(substring))
		}
		hash := computeTargetConfigHash(nil, cfg.Data)
		g.Expect(hash).ShouldNot(BeNil())
		g.Expect(*hash).ShouldNot(BeEmpty())
		configHash = *hash
	})).Should(Succeed())
	return configHash
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
	testapps.ClearResources(&testCtx, generics.ParamConfigRendererSignature)
	testapps.ClearResources(&testCtx, generics.ComponentDefinitionSignature, ml)
	// namespaced
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.SecretSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentParameterSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, parameterViewSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ParameterSignature, true, inNS, ml)
}
