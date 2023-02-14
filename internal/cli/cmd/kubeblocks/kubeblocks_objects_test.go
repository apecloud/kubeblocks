/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubeblocks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("kubeblocks objects", func() {
	It("deleteDeploys", func() {
		client := testing.FakeClientSet()
		Expect(deleteDeploys(client, nil)).Should(Succeed())

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
			client = testing.FakeClientSet(mockDeploy(map[string]string{
				k: v,
			}))
			objs, _ := getKBObjects(client, testing.FakeDynamicClient(testing.FakeVolumeSnapshotClass()), namespace)
			Expect(deleteDeploys(client, objs.deploys)).Should(Succeed())
		}
	})

	It("deleteServices", func() {
		client := testing.FakeClientSet()
		Expect(deleteServices(client, nil)).Should(Succeed())

		mockService := func(label map[string]string) *corev1.Service {
			svc := &corev1.Service{}
			svc.SetLabels(label)
			svc.SetNamespace(namespace)
			return svc
		}

		labels := map[string]string{
			"types.InstanceLabelKey": types.KubeBlocksChartName,
			"release":                types.KubeBlocksChartName,
		}
		for k, v := range labels {
			client = testing.FakeClientSet(mockService(map[string]string{
				k: v,
			}))
			objs, _ := getKBObjects(client, testing.FakeDynamicClient(testing.FakeVolumeSnapshotClass()), namespace)
			Expect(deleteServices(client, objs.svcs)).Should(Succeed())
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
			clusterDef     *dbaasv1alpha1.ClusterDefinition
			clusterVersion *dbaasv1alpha1.ClusterVersion
			backupTool     *dpv1alpha1.BackupTool
			expected       string
		}{
			{
				clusterDef:     testing.FakeClusterDef(),
				clusterVersion: testing.FakeClusterVersion(),
				backupTool:     testing.FakeBackupTool(),
				expected:       "Unable to remove nonexistent key: finalizers",
			},
			{
				clusterDef:     clusterDef,
				clusterVersion: testing.FakeClusterVersion(),
				backupTool:     testing.FakeBackupTool(),
				expected:       "Unable to remove nonexistent key: finalizers",
			},
			{
				clusterDef:     clusterDef,
				clusterVersion: clusterVersion,
				backupTool:     backupTool,
				expected:       "",
			},
		}

		for _, c := range testCases {
			client := mockDynamicClientWithCRD(c.clusterDef, c.clusterVersion, c.backupTool)
			objs, _ := getKBObjects(testing.FakeClientSet(), client, "")
			if c.expected != "" {
				Expect(removeFinalizers(client, objs)).Should(MatchError(MatchRegexp(c.expected)))
			} else {
				Expect(removeFinalizers(client, objs)).Should(Succeed())
			}
		}
	})

	It("delete crd", func() {
		client := mockDynamicClientWithCRD()
		objs, _ := getKBObjects(testing.FakeClientSet(), client, "")
		Expect(deleteCRDs(client, objs.crds)).Should(Succeed())
	})
})

func mockDynamicClientWithCRD(objects ...runtime.Object) dynamic.Interface {
	clusterCRD := v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusters.dbaas.kubeblocks.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.Group,
		},
		Status: v1.CustomResourceDefinitionStatus{},
	}
	clusterDefCRD := v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterdefinitions.dbaas.kubeblocks.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.Group,
		},
		Status: v1.CustomResourceDefinitionStatus{},
	}
	clusterVersionCRD := v1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "clusterversions.dbaas.kubeblocks.io",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: types.Group,
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
			Group: types.DPGroup,
		},
		Status: v1.CustomResourceDefinitionStatus{},
	}

	allObjs := []runtime.Object{&clusterCRD, &clusterDefCRD, &clusterVersionCRD, &backupToolCRD,
		testing.FakeVolumeSnapshotClass()}
	allObjs = append(allObjs, objects...)
	return testing.FakeDynamicClient(allObjs...)
}
