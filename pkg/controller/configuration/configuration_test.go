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

package configuration

import (
	"fmt"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

const (
	compDefName               = "test-compdef"
	clusterName               = "test-cluster"
	configTemplateName        = "test-config-template"
	scriptTemplateName        = "test-script-template"
	mysqlCompName             = "mysql"
	mysqlConfigName           = "mysql-component-config"
	mysqlConfigConstraintName = "mysql8.0-config-constraints"
	mysqlScriptsTemplateName  = "apecloud-mysql-scripts"
	testConfigContent         = "test-config-content"
)

func allFieldsCompDefObj(create bool) *appsv1alpha1.ComponentDefinition {
	compDef := testapps.NewComponentDefinitionFactory(compDefName).
		SetDefaultSpec().
		AddConfigTemplate(configTemplateName, mysqlConfigName, mysqlConfigConstraintName, testCtx.DefaultNamespace, testapps.ConfVolumeName).
		AddScriptTemplate(scriptTemplateName, mysqlScriptsTemplateName, testCtx.DefaultNamespace, testapps.ScriptsVolumeName, nil).
		GetObject()
	if create {
		Expect(testCtx.CreateObj(testCtx.Ctx, compDef)).Should(Succeed())
	}
	return compDef
}

func newAllFieldsClusterObj(compDef *appsv1alpha1.ComponentDefinition, create bool) (*appsv1alpha1.Cluster, *appsv1alpha1.ComponentDefinition, types.NamespacedName) {
	// setup Cluster obj requires default ComponentDefinition object
	if compDef == nil {
		compDef = allFieldsCompDefObj(create)
	}
	pvcSpec := testapps.NewPVCSpec("1Gi")
	clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
		AddComponentV2(mysqlCompName, compDef.Name).
		SetReplicas(1).
		AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
		AddComponentService(testapps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
		AddComponentService(testapps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
		GetObject()
	key := client.ObjectKeyFromObject(clusterObj)
	if create {
		Expect(testCtx.CreateObj(testCtx.Ctx, clusterObj)).Should(Succeed())
	}
	return clusterObj, compDef, key
}

func newAllFieldsSynthesizedComponent(compDef *appsv1alpha1.ComponentDefinition, cluster *appsv1alpha1.Cluster) *component.SynthesizedComponent {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: testCtx.Ctx,
		Log: logger,
	}
	comp, err := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
	if err != nil {
		panic(fmt.Sprintf("build component object error: %v", err))
	}
	synthesizeComp, err := component.BuildSynthesizedComponent(reqCtx, testCtx.Cli, cluster, compDef, comp)
	Expect(err).Should(Succeed())
	Expect(synthesizeComp).ShouldNot(BeNil())
	addTestVolumeMount(synthesizeComp.PodSpec, mysqlCompName)
	if len(synthesizeComp.ConfigTemplates) > 0 {
		configSpec := &synthesizeComp.ConfigTemplates[0]
		configSpec.ReRenderResourceTypes = []appsv1alpha1.RerenderResourceType{appsv1alpha1.ComponentVScaleType, appsv1alpha1.ComponentHScaleType}
	}
	return synthesizeComp
}

func newAllFieldsComponent(cluster *appsv1alpha1.Cluster) *appsv1alpha1.Component {
	comp, _ := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
	return comp
}

func addTestVolumeMount(spec *corev1.PodSpec, containerName string) {
	for i := range spec.Containers {
		container := &spec.Containers[i]
		if container.Name != containerName {
			continue
		}
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      testapps.ScriptsVolumeName,
				MountPath: "/scripts",
			}, corev1.VolumeMount{
				Name:      testapps.ConfVolumeName,
				MountPath: "/etc/config",
			})
		return
	}
}
