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

package configuration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

const clusterDefName = "test-clusterdef"
const clusterVersionName = "test-clusterversion"
const clusterName = "test-cluster"
const statefulCompDefName = "replicasets"
const statefulCompName = "mysql"
const statefulSetName = "mysql-statefulset"
const configSpecName = "mysql-config-tpl"
const configVolumeName = "mysql-config"
const cmName = "mysql-tree-node-template-8.0"

func mockConfigResource() (*corev1.ConfigMap, *appsv1alpha1.ConfigConstraint) {
	By("Create a config template obj")
	configmap := testapps.CreateCustomizedObj(&testCtx,
		"resources/mysql-config-template.yaml", &corev1.ConfigMap{},
		testCtx.UseDefaultNamespace(),
		testapps.WithLabels(
			constant.AppNameLabelKey, clusterName,
			constant.AppInstanceLabelKey, clusterName,
			constant.KBAppComponentLabelKey, statefulCompName,
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
		&appsv1alpha1.ConfigConstraint{})

	By("check config constraint")
	Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(constraint), func(g Gomega, tpl *appsv1alpha1.ConfigConstraint) {
		g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(appsv1alpha1.AvailablePhase))
	})).Should(Succeed())

	By("Create a configuration obj")
	// test-cluster-mysql-mysql-config-tpl
	configuration := builder.NewConfigurationBuilder(testCtx.DefaultNamespace, core.GenerateComponentConfigurationName(clusterName, statefulCompName)).
		ClusterRef(clusterName).
		Component(statefulCompName).
		AddConfigurationItem(appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
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

func mockReconcileResource() (*corev1.ConfigMap, *appsv1alpha1.ConfigConstraint, *appsv1alpha1.Cluster, *appsv1alpha1.ClusterVersion, *component.SynthesizedComponent) {
	configmap, constraint := mockConfigResource()

	By("Create a clusterDefinition obj")
	clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
		AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
		AddConfigTemplate(configSpecName, configmap.Name, constraint.Name, testCtx.DefaultNamespace, configVolumeName).
		AddLabels(core.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
			core.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
		Create(&testCtx).GetObject()

	By("Create a clusterVersion obj")
	clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
		AddComponentVersion(statefulCompDefName).
		AddLabels(core.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
			core.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
		Create(&testCtx).GetObject()

	By("Creating a cluster")
	clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
		clusterDefObj.Name, clusterVersionObj.Name).
		AddComponent(statefulCompName, statefulCompDefName).Create(&testCtx).GetObject()

	container := *builder.NewContainerBuilder("mock-container").
		AddVolumeMounts(corev1.VolumeMount{
			Name:      configVolumeName,
			MountPath: "/mnt/config",
		}).GetObject()
	_ = testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, statefulSetName, clusterObj.Name, statefulCompName).
		AddConfigmapVolume(configVolumeName, configmap.Name).
		AddContainer(container).
		AddAppNameLabel(clusterName).
		AddAppInstanceLabel(clusterName).
		AddAppComponentLabel(statefulCompName).
		AddAnnotations(core.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name).
		Create(&testCtx).GetObject()

	synthesizedComp, err := component.BuildComponent(intctrlutil.RequestCtx{
		Ctx: ctx,
		Log: log.FromContext(ctx),
	}, nil,
		clusterObj,
		clusterDefObj,
		clusterDefObj.GetComponentDefByName(statefulCompDefName),
		clusterObj.Spec.GetComponentByName(statefulCompName),
		nil,
		clusterVersionObj.Spec.GetDefNameMappingComponents()[statefulCompDefName])
	Expect(err).ShouldNot(HaveOccurred())

	return configmap, constraint, clusterObj, clusterVersionObj, synthesizedComp
}

func initConfiguration(resourceCtx *intctrlutil.ResourceCtx, synthesizedComponent *component.SynthesizedComponent, clusterObj *appsv1alpha1.Cluster, clusterVersionObj *appsv1alpha1.ClusterVersion) error {
	return configuration.NewCreatePipeline(configuration.ReconcileCtx{
		ResourceCtx: resourceCtx,
		Component:   synthesizedComponent,
		Cluster:     clusterObj,
		ClusterVer:  clusterVersionObj,
		PodSpec:     synthesizedComponent.PodSpec,
	}).
		Prepare().
		UpdateConfiguration(). // reconcile Configuration
		Configuration().       // sync Configuration
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

	// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
	testapps.ClearClusterResources(&testCtx)

	// delete rest mocked objects
	inNS := client.InNamespace(testCtx.DefaultNamespace)
	ml := client.HasLabels{testCtx.TestObjLabelKey}
	// non-namespaced
	testapps.ClearResources(&testCtx, generics.ConfigConstraintSignature, ml)
	// namespaced
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StatefulSetSignature, true, inNS, ml)
	testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigurationSignature, false, inNS, ml)
}
