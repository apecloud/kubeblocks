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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

const (
	compDefName              = "test-compdef"
	clusterName              = "test-cluster"
	paramsDefName            = "mysql-params-def"
	pdcrName                 = "mysql-pdcr"
	configTemplateName       = "test-config-template"
	scriptTemplateName       = "test-script-template"
	mysqlCompName            = "mysql"
	mysqlConfigName          = "mysql-component-config"
	mysqlScriptsTemplateName = "apecloud-mysql-scripts"
)

func allFieldsCompDefObj(create bool) *appsv1.ComponentDefinition {
	compDef := testapps.NewComponentDefinitionFactory(compDefName).
		SetDefaultSpec().
		AddConfigTemplate(configTemplateName, mysqlConfigName, testCtx.DefaultNamespace, testapps.ConfVolumeName).
		AddScriptTemplate(scriptTemplateName, mysqlScriptsTemplateName, testCtx.DefaultNamespace, testapps.ScriptsVolumeName, nil).
		GetObject()
	if create {
		Expect(testCtx.CreateObj(testCtx.Ctx, compDef)).Should(Succeed())
	}
	compDef.Status.Phase = appsv1.AvailablePhase
	return compDef
}

func newAllFieldsClusterObj(compDef *appsv1.ComponentDefinition, create bool) (*appsv1.Cluster, *appsv1.ComponentDefinition, types.NamespacedName) {
	// setup Cluster obj requires default ComponentDefinition object
	if compDef == nil {
		compDef = allFieldsCompDefObj(create)
	}
	pvcSpec := testapps.NewPVCSpec("1Gi")
	clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
		AddComponent(mysqlCompName, compDef.Name).
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

func newAllFieldsSynthesizedComponent(compDef *appsv1.ComponentDefinition, cluster *appsv1.Cluster) *component.SynthesizedComponent {
	comp, err := component.BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
	if err != nil {
		panic(fmt.Sprintf("build component object error: %v", err))
	}
	synthesizeComp, err := component.BuildSynthesizedComponent(testCtx.Ctx, testCtx.Cli, compDef, comp, cluster)
	Expect(err).Should(Succeed())
	Expect(synthesizeComp).ShouldNot(BeNil())
	addTestVolumeMount(synthesizeComp.PodSpec, mysqlCompName)
	return synthesizeComp
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
