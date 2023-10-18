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

package cloudprovider

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("playground", func() {
	var (
		provider    = newLocalCloudProvider(os.Stdout, os.Stderr)
		clusterName = "k3d-kb-test"
	)

	It("k3d util function", func() {
		config, err := buildClusterRunConfig("test")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(config.Name).Should(ContainSubstring("test"))
		Expect(setUpK3d(context.Background(), nil)).Should(HaveOccurred())
		Expect(provider.DeleteK8sCluster(&K8sClusterInfo{ClusterName: clusterName})).Should(HaveOccurred())
	})
})
