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
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const clusterDefName = "test-clusterdef"
const clusterVersionName = "test-clusterversion"
const clusterName = "test-cluster"
const mysqlCompDefName = "replicasets"
const scriptConfigName = "test-script-config"
const configSpecName = "test-config-spec"
const mysqlCompName = "mysql"
const mysqlConfigName = "mysql-component-config"
const mysqlConfigConstraintName = "mysql8.0-config-constraints"
const mysqlScriptsConfigName = "apecloud-mysql-scripts"
const testConfigContent = "test-config-content"

func allFieldsClusterDefObj(needCreate bool) *appsv1alpha1.ClusterDefinition {
	clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
		AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
		AddScriptTemplate(scriptConfigName, mysqlScriptsConfigName, testCtx.DefaultNamespace, testapps.ScriptsVolumeName, nil).
		AddConfigTemplate(configSpecName, mysqlConfigName, mysqlConfigConstraintName, testCtx.DefaultNamespace, testapps.ConfVolumeName).
		GetObject()
	if needCreate {
		Expect(testCtx.CreateObj(testCtx.Ctx, clusterDefObj)).Should(Succeed())
	}
	return clusterDefObj
}

func allFieldsClusterVersionObj(needCreate bool) *appsv1alpha1.ClusterVersion {
	clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
		AddComponentVersion(mysqlCompDefName).
		AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
		GetObject()
	if needCreate {
		Expect(testCtx.CreateObj(testCtx.Ctx, clusterVersionObj)).Should(Succeed())
	}
	return clusterVersionObj
}

func newAllFieldsClusterObj(
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

func newAllFieldsComponent(clusterDef *appsv1alpha1.ClusterDefinition, clusterVersion *appsv1alpha1.ClusterVersion) *component.SynthesizedComponent {
	cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(clusterDef, clusterVersion, false)
	component, err := component.BuildComponent(
		intctrlutil.RequestCtx{
			Ctx: testCtx.Ctx,
			Log: logger,
		},
		nil,
		cluster,
		clusterDef,
		&clusterDef.Spec.ComponentDefs[0],
		&cluster.Spec.ComponentSpecs[0],
		nil,
		&clusterVersion.Spec.ComponentVersions[0])
	Expect(err).Should(Succeed())
	Expect(component).ShouldNot(BeNil())
	addTestVolumeMount(component.PodSpec, mysqlCompName)
	return component
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
