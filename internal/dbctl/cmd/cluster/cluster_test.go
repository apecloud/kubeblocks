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

package cluster

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("Cluster", func() {
	var streams genericclioptions.IOStreams
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
	})

	Context("create", func() {
		It("without name", func() {
			o := &CreateOptions{IOStreams: streams}
			Expect(o.Validate([]string{})).To(MatchError("missing cluster name"))
		})

		It("without cluster definition", func() {
			o := &CreateOptions{IOStreams: streams}
			Expect(o.Validate([]string{"test"})).To(MatchError("cluster-definition can not be empty"))
		})

		It("without app-version", func() {
			o := &CreateOptions{
				IOStreams:     streams,
				ClusterDefRef: "wesql",
			}
			Expect(o.Validate([]string{"test"})).To(MatchError("app-version can not be empty"))
		})

		It("new command", func() {
			tf := cmdtesting.NewTestFactory().WithNamespace("default")
			defer tf.Cleanup()
			cmd := NewCreateCmd(tf, streams)
			Expect(cmd != nil).To(BeTrue())
		})

		It("run", func() {
			tf := cmdtesting.NewTestFactory().WithNamespace("default")
			defer tf.Cleanup()
			tf.ClientConfigVal = cfg

			o := &CreateOptions{
				IOStreams:       streams,
				ClusterDefRef:   "wesql",
				AppVersionRef:   "app-version",
				PodAntiAffinity: "Preferred",
				TopologyKeys:    []string{"kubernetes.io/hostname"},
				NodeLabels:      map[string]string{"testLabelKey": "testLabelValue"},
				Components:      "",
			}
			Expect(o.Complete(tf, []string{"test"})).Should(Succeed())
			Expect(o.Namespace).To(Equal("default"))
			Expect(o.Name).To(Equal("test"))

			Expect(o.Run()).Should(Succeed())

			del := &DeleteOptions{}
			Expect(del.Validate([]string{})).To(MatchError("missing cluster name"))
			Expect(del.Complete(tf, []string{"test"})).Should(Succeed())
			Expect(del.Namespace).To(Equal("default"))
			Expect(del.Run()).Should(Succeed())
		})
	})

	It("delete", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()
		cmd := NewDeleteCmd(tf)
		Expect(cmd != nil).To(BeTrue())

		del := &DeleteOptions{}
		Expect(del.Validate([]string{})).To(MatchError("missing cluster name"))
		Expect(del.Complete(tf, []string{"test"})).Should(Succeed())
		Expect(del.Namespace).To(Equal("default"))
	})

	It("describe", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()
		cmd := NewDescribeCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())
	})

	It("list", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()
		cmd := NewListCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())
	})

	It("cluster", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()
		cmd := NewClusterCmd(tf, streams)
		Expect(cmd != nil).To(BeTrue())
		Expect(cmd.HasSubCommands()).To(BeTrue())
	})
})
