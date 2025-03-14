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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/generics"
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
		AddAnnotations(
			constant.KBParameterUpdateSourceAnnotationKey, constant.ReconfigureManagerSource,
			constant.ConfigurationRevision, "1",
			constant.CMInsEnableRerenderTemplateKey, "true").
		AddConfigFile(envTestFileKey, "abcde=1234").
		Create(&testCtx).
		GetObject()

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

func mockReconcileResource() (*corev1.ConfigMap, *parametersv1alpha1.ParametersDefinition, *appsv1.Cluster, *appsv1.Component, *component.SynthesizedComponent) {
	configmap, paramsDef := mockConfigResource()

	By("Create a component definition obj and mock to available")
	compDefObj := testapps.NewComponentDefinitionFactory(compDefName).
		WithRandomName().
		SetDefaultSpec().
		AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName).
		Create(&testCtx).
		GetObject()
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
		AddSharding(shardingCompName, "", compDefObj.GetName()).
		SetShards(5).
		Create(&testCtx).
		GetObject()

	By("Create a component obj")
	fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
	compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
		AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
		AddLabels(constant.AppInstanceLabelKey, clusterName).
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
		AddAnnotations(core.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name).
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
	// namespaced
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.SecretSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentParameterSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ParameterSignature, true, inNS, ml)
}
