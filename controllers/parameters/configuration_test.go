/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

const (
	compDefName      = "test-compdef"
	clusterName      = "test-cluster"
	defaultCompName  = "mysql"
	defaultITSName   = "mysql-statefulset"
	configSpecName   = "mysql-config-tpl"
	configVolumeName = "mysql-config"
	cmName           = "mysql-tree-node-template-8.0"
)

func mockConfigResource() (*corev1.ConfigMap, *appsv1beta1.ConfigConstraint) {
	By("Create a config template obj")
	configmap := testapps.CreateCustomizedObj(&testCtx,
		"resources/mysql-config-template.yaml", &corev1.ConfigMap{},
		testCtx.UseDefaultNamespace(),
		testapps.WithLabels(
			constant.AppNameLabelKey, clusterName,
			constant.AppInstanceLabelKey, clusterName,
			constant.KBAppComponentLabelKey, defaultCompName,
			constant.CMConfigurationTemplateNameLabelKey, configSpecName,
			constant.CMConfigurationConstraintsNameLabelKey, cmName,
			constant.CMConfigurationSpecProviderLabelKey, configSpecName,
			constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
		),
		testapps.WithAnnotations(
			constant.KBParameterUpdateSourceAnnotationKey, constant.ReconfigureManagerSource,
			constant.ConfigurationRevision, "1",
			constant.CMInsEnableRerenderTemplateKey, "true"))

	By("Create a config constraint obj")
	constraint := testapps.CreateCustomizedObj(&testCtx,
		"resources/mysql-config-constraint.yaml",
		&appsv1beta1.ConfigConstraint{})

	By("check config constraint")
	Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(constraint), func(g Gomega, tpl *appsv1beta1.ConfigConstraint) {
		g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(appsv1alpha1.AvailablePhase))
	})).Should(Succeed())

	By("Create a configuration obj")
	// test-cluster-mysql-mysql-config-tpl
	configuration := builder.NewConfigurationBuilder(testCtx.DefaultNamespace, core.GenerateComponentConfigurationName(clusterName, defaultCompName)).
		ClusterRef(clusterName).
		Component(defaultCompName).
		AddConfigurationItem(appsv1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
				Name:        configSpecName,
				TemplateRef: configmap.Name,
				Namespace:   configmap.Namespace,
				VolumeName:  configVolumeName,
			},
			ConfigConstraintRef: constraint.Name,
		}).
		GetObject()
	Expect(testCtx.CreateObj(testCtx.Ctx, configuration)).Should(Succeed())

	return configmap, constraint
}

func mockReconcileResource() (*corev1.ConfigMap, *appsv1beta1.ConfigConstraint, *appsv1.Cluster, *appsv1.Component, *component.SynthesizedComponent) {
	configmap, constraint := mockConfigResource()

	By("Create a component definition obj and mock to available")
	compDefObj := testapps.NewComponentDefinitionFactory(compDefName).
		WithRandomName().
		SetDefaultSpec().
		AddConfigTemplate(configSpecName, configmap.Name, constraint.Name, testCtx.DefaultNamespace, configVolumeName).
		AddLabels(core.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
			core.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
		Create(&testCtx).
		GetObject()
	Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
		obj.Status.Phase = appsv1.AvailablePhase
	})()).Should(Succeed())

	By("Creating a cluster")
	clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
		AddComponent(defaultCompName, compDefObj.GetName()).
		Create(&testCtx).
		GetObject()

	By("Create a component obj")
	fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
	compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
		AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
		AddLabels(constant.AppInstanceLabelKey, clusterName).
		SetUID(types.UID(fmt.Sprintf("%s-%s", clusterObj.Name, "test-uid"))).
		SetReplicas(1).
		Create(&testCtx).GetObject()

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

	synthesizedComp, err := component.BuildSynthesizedComponent(testCtx.Ctx, testCtx.Cli, compDefObj, compObj, clusterObj)
	Expect(err).ShouldNot(HaveOccurred())

	return configmap, constraint, clusterObj, compObj, synthesizedComp
}

func initConfiguration(resourceCtx *configctrl.ResourceCtx,
	synthesizedComponent *component.SynthesizedComponent,
	clusterObj *appsv1.Cluster,
	componentObj *appsv1.Component) error {
	return configctrl.NewCreatePipeline(configctrl.ReconcileCtx{
		ResourceCtx:          resourceCtx,
		Component:            componentObj,
		SynthesizedComponent: synthesizedComponent,
		Cluster:              clusterObj,
		PodSpec:              synthesizedComponent.PodSpec,
	}).
		Prepare().
		UpdateConfiguration(). // reconcile Configuration
		Configuration(). // sync Configuration
		CreateConfigTemplate().
		UpdateConfigRelatedObject().
		UpdateConfigurationStatus().
		Complete()
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
	testapps.ClearResources(&testCtx, generics.ConfigConstraintSignature, ml)
	// namespaced
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.SecretSignature, true, inNS)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigurationSignature, false, inNS, ml)
}
