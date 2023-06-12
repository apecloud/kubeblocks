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

package kubeblocks

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("kubeblocks objects", func() {

	It("delete objects", func() {
		dynamic := testing.FakeDynamicClient()
		Expect(deleteObjects(dynamic, types.DeployGVR(), nil)).Should(Succeed())

		mockDeploy := func(label map[string]string) *appsv1.Deployment {
			deploy := &appsv1.Deployment{}
			deploy.SetLabels(label)
			deploy.SetNamespace(namespace)
			return deploy
		}

		labels := map[string]string{
			"types.InstanceLabelKey": types.KubeBlocksChartName,
			"release":                types.KubeBlocksChartName,
		}
		for k, v := range labels {
			dynamic = testing.FakeDynamicClient(mockDeploy(map[string]string{
				k: v,
			}))
			objs, _ := getKBObjects(testing.FakeDynamicClient(testing.FakeVolumeSnapshotClass()), namespace, nil)
			Expect(deleteObjects(dynamic, types.DeployGVR(), objs[types.DeployGVR()])).Should(Succeed())
		}
	})

	It("newDeleteOpts", func() {
		opts := newDeleteOpts()
		Expect(*opts.GracePeriodSeconds).Should(Equal(int64(0)))
	})

	It("remove finalizer", func() {
		clusterDef := testing.FakeClusterDef()
		clusterDef.Finalizers = []string{"test"}
		clusterVersion := testing.FakeClusterVersion()
		clusterVersion.Finalizers = []string{"test"}
		backupTool := testing.FakeBackupTool()
		backupTool.Finalizers = []string{"test"}

		testCases := []struct {
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			backupTool     *dpv1alpha1.BackupTool
		}{
			{
				clusterDef:     testing.FakeClusterDef(),
				clusterVersion: testing.FakeClusterVersion(),
				backupTool:     testing.FakeBackupTool(),
			},
			{
				clusterDef:     clusterDef,
				clusterVersion: testing.FakeClusterVersion(),
				backupTool:     testing.FakeBackupTool(),
			},
			{
				clusterDef:     clusterDef,
				clusterVersion: clusterVersion,
				backupTool:     backupTool,
			},
		}

		for _, c := range testCases {
			objects := mockCRD()
			objects = append(objects, testing.FakeVolumeSnapshotClass())
			objects = append(objects, c.clusterDef, c.clusterVersion, c.backupTool)
			client := testing.FakeDynamicClient(objects...)
			objs, _ := getKBObjects(client, "", nil)
			Expect(removeCustomResources(client, objs)).Should(Succeed())
		}
	})

	It("delete crd", func() {
		objects := mockCRD()
		objects = append(objects, testing.FakeVolumeSnapshotClass())
		dynamic := testing.FakeDynamicClient(objects...)
		objs, _ := getKBObjects(dynamic, "", nil)
		Expect(deleteObjects(dynamic, types.CRDGVR(), objs[types.CRDGVR()])).Should(Succeed())
	})

	It("test getKBObjects", func() {
		objects := mockCRD()
		objects = append(objects, mockCRs()...)
		objects = append(objects, testing.FakeVolumeSnapshotClass())
		objects = append(objects, mockRBACResources()...)
		objects = append(objects, mockConfigMaps()...)
		dynamic := testing.FakeDynamicClient(objects...)
		objs, _ := getKBObjects(dynamic, "", nil)

		tmp, err := dynamic.Resource(types.ClusterRoleGVR()).Namespace(metav1.NamespaceAll).
			List(context.TODO(), metav1.ListOptions{LabelSelector: buildKubeBlocksSelectorLabels()})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(tmp.Items).Should(HaveLen(1))
		// verify crds
		Expect(objs[types.CRDGVR()].Items).Should(HaveLen(4))
		// verify crs
		for _, gvr := range []schema.GroupVersionResource{types.ClusterDefGVR(), types.ClusterVersionGVR()} {
			objlist, ok := objs[gvr]
			Expect(ok).Should(BeTrue())
			Expect(objlist.Items).Should(HaveLen(1))
		}

		// verify rbac info
		for _, gvr := range []schema.GroupVersionResource{types.RoleGVR(), types.ClusterRoleBindingGVR(), types.ServiceAccountGVR()} {
			objlist, ok := objs[gvr]
			Expect(ok).Should(BeTrue())
			Expect(objlist.Items).Should(HaveLen(1), gvr.String())
		}
		// verify cofnig tpl
		for _, gvr := range []schema.GroupVersionResource{types.ConfigmapGVR()} {
			objlist, ok := objs[gvr]
			Expect(ok).Should(BeTrue())
			Expect(objlist.Items).Should(HaveLen(1), gvr.String())
		}
	})
})

func mockName() string {
	return uuid.NewString()
}

func mockCRD() []runtime.Object {
	clusterCRD := v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusters.apps.kubeblocks.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.AppsAPIGroup,
		},
		Status: v1.CustomResourceDefinitionStatus{},
	}
	clusterDefCRD := v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterdefinitions.apps.kubeblocks.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.AppsAPIGroup,
		},
		Status: v1.CustomResourceDefinitionStatus{},
	}
	clusterVersionCRD := v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterversions.apps.kubeblocks.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.AppsAPIGroup,
		},
		Status: v1.CustomResourceDefinitionStatus{},
	}

	backupToolCRD := v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "backuptools.dataprotection.kubeblocks.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.DPAPIGroup,
		},
		Status: v1.CustomResourceDefinitionStatus{},
	}
	return []runtime.Object{&clusterCRD, &clusterDefCRD, &clusterVersionCRD, &backupToolCRD}
}

func mockCRs() []runtime.Object {
	allObjects := make([]runtime.Object, 0)
	allObjects = append(allObjects, testing.FakeClusterDef())
	allObjects = append(allObjects, testing.FakeClusterVersion())
	return allObjects
}

func mockRBACResources() []runtime.Object {
	sa := testing.FakeServiceAccount(mockName())

	clusterRole := testing.FakeClusterRole(mockName())
	clusterRoleBinding := testing.FakeClusterRoleBinding(mockName(), sa, clusterRole)

	role := testing.FakeRole(mockName())
	roleBinding := testing.FakeRoleBinding(mockName(), sa, role)

	return []runtime.Object{sa, clusterRole, clusterRoleBinding, role, roleBinding}
}

func mockConfigMaps() []runtime.Object {
	obj := testing.FakeConfigMap(mockName())
	// add a config tpl label
	if obj.ObjectMeta.Labels == nil {
		obj.ObjectMeta.Labels = make(map[string]string)
	}
	obj.ObjectMeta.Labels[constant.CMConfigurationTypeLabelKey] = constant.ConfigTemplateType
	return []runtime.Object{obj}
}
