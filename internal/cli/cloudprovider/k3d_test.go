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

package cloudprovider

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("playground", func() {
	var (
		provider    = NewLocalCloudProvider(os.Stdout, os.Stderr)
		clusterName = "k3d-tb-est"
	)

	It("k3d util function", func() {
		config, err := buildClusterRunConfig("test")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(config.Name).Should(ContainSubstring("test"))
		Expect(setUpK3d(context.Background(), nil)).Should(HaveOccurred())
		Expect(provider.DeleteK8sCluster(&K8sClusterInfo{ClusterName: clusterName})).Should(HaveOccurred())
	})
})
