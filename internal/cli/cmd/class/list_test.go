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

package class

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("list", func() {
	var (
		out     *bytes.Buffer
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
	)

	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		tf.FakeDynamicClient = testing.FakeDynamicClient(&classDef)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("should succeed", func() {
		cmd := NewListCommand(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		_ = cmd.Flags().Set("cluster-definition", "apecloud-mysql")
		cmd.Run(cmd, []string{})
		Expect(out.String()).To(ContainSubstring("general-1c1g"))
		Expect(out.String()).To(ContainSubstring("mysql"))
		Expect(out.String()).To(ContainSubstring(generalResourceConstraint.Name))
	})

	It("memory should be normalized", func() {
		cases := []struct {
			memory     string
			normalized string
		}{
			{
				memory:     "0.2Gi",
				normalized: "0.2Gi",
			},
			{
				memory:     "0.2Mi",
				normalized: "0.2Mi",
			},
			{
				memory:     "0.2Ki",
				normalized: "0.2Ki",
			},
			{
				memory:     "1024Mi",
				normalized: "1Gi",
			},
			{
				memory:     "1025Mi",
				normalized: "1025Mi",
			},
			{
				memory:     "1023Mi",
				normalized: "1023Mi",
			},
			{
				memory:     "1Gi",
				normalized: "1Gi",
			},
			{
				memory:     "512Mi",
				normalized: "512Mi",
			},
		}
		for _, item := range cases {
			Expect(normalizeMemory(resource.MustParse(item.memory))).Should(Equal(item.normalized))
		}
	})
})
