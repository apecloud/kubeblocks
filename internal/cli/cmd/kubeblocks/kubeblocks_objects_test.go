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
			objs, _ := getKBObjects(client, testing.FakeDynamicClient(), namespace)
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
			objs, _ := getKBObjects(client, testing.FakeDynamicClient(), namespace)
			Expect(deleteServices(client, objs.svcs)).Should(Succeed())
		}
	})

	It("newDeleteOpts", func() {
		opts := newDeleteOpts()
		Expect(*opts.GracePeriodSeconds).Should(Equal(int64(0)))
	})
})
