/*
Copyright 2022 The KubeBlocks Authors

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

package playground

import (
	"context"

	config "github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("playground", func() {
	installer := &installer{
		ctx:         context.Background(),
		cfg:         config.ClusterConfig{},
		clusterName: "k3d-test",
		namespace:   ClusterNamespace,
		dbCluster:   "k3d-test-dbcluster",
		wesql: Wesql{
			serverVersion: wesqlVersion,
			replicas:      1,
		},
	}

	It("kubeconfig", func() {
		Expect(installer.genKubeconfig()).Should(HaveOccurred())
		Expect(installer.setKubeconfig()).Should(Succeed())
	})

	It("print guide", func() {
		Expect(installer.printGuide("", "")).Should(Succeed())
	})

	It("k3d util function", func() {
		config, err := buildClusterRunConfig("test")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(config.Name).Should(ContainSubstring("test"))
		Expect(installer.installDeps()).Should(HaveOccurred())
		Expect(setUpK3d(installer.ctx, nil)).Should(HaveOccurred())
		Expect(installer.uninstall()).Should(HaveOccurred())
	})
})
