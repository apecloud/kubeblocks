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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("cluster util", func() {
	client := testing.FakeClientSet(
		testing.FakePods(3, testing.Namespace, testing.ClusterName),
		testing.FakeNode(),
		testing.FakeSecrets(testing.Namespace, testing.ClusterName),
		testing.FakeServices(),
		testing.FakePVCs())

	dynamic := testing.FakeDynamicClient(
		testing.FakeCluster(testing.ClusterName, testing.Namespace),
		testing.FakeClusterDef(),
		testing.FakeClusterVersion())

	It("get cluster objects", func() {
		clusterName := testing.ClusterName
		getter := ObjectsGetter{
			Client:    client,
			Dynamic:   dynamic,
			Name:      clusterName,
			Namespace: testing.Namespace,
			GetOptions: GetOptions{
				WithClusterDef:     true,
				WithClusterVersion: true,
				WithConfigMap:      true,
				WithService:        true,
				WithSecret:         true,
				WithPVC:            true,
				WithPod:            true,
			},
		}

		objs, err := getter.Get()
		Expect(err).Should(Succeed())
		Expect(objs).ShouldNot(BeNil())
		Expect(objs.Cluster.Name).Should(Equal(clusterName))
		Expect(objs.ClusterDef.Name).Should(Equal(testing.ClusterDefName))
		Expect(objs.ClusterVersion.Name).Should(Equal(testing.ClusterVersionName))

		Expect(len(objs.Pods.Items)).Should(Equal(3))
		Expect(len(objs.Nodes)).Should(Equal(1))
		Expect(len(objs.Secrets.Items)).Should(Equal(1))
		Expect(len(objs.Services.Items)).Should(Equal(4))
		Expect(len(objs.PVCs.Items)).Should(Equal(1))
		Expect(len(objs.GetComponentInfo())).Should(Equal(1))
	})
})
