/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package addon

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	restfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

const (
	testNamespace = "test"
)

var _ = Describe("Manage applications related to KubeBlocks", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testNamespace)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	When("Iterate addon sub-cmds", func() {
		It("do sanity check", func() {
			addonCmd := NewAddonCmd(tf, streams)
			Expect(addonCmd).ShouldNot(BeNil())
			Expect(addonCmd.HasSubCommands()).Should(BeTrue())

			listCmd := newListCmd(tf, streams)
			Expect(listCmd).ShouldNot(BeNil())
			Expect(listCmd.HasSubCommands()).ShouldNot(BeTrue())

			enableCmd := newEnableCmd(tf, streams)
			Expect(enableCmd).ShouldNot(BeNil())
			Expect(enableCmd.HasSubCommands()).ShouldNot(BeTrue())

			disableCmd := newDisableCmd(tf, streams)
			Expect(disableCmd).ShouldNot(BeNil())
			Expect(disableCmd.HasSubCommands()).ShouldNot(BeTrue())

			describeCmd := newDescribeCmd(tf, streams)
			Expect(describeCmd).ShouldNot(BeNil())
			Expect(describeCmd.HasSubCommands()).ShouldNot(BeTrue())
		})
	})

	When("Validate at enable an addon", func() {
		It("should return error", func() {
			o := &addonCmdOpts{
				Options:          patch.NewOptions(tf, streams, types.AddonGVR()),
				Factory:          tf,
				IOStreams:        streams,
				addonEnableFlags: &addonEnableFlags{},
				complete:         addonEnableDisableHandler,
			}
			addonObj := testing.FakeAddon("addon-test")
			o.addon = *addonObj
			codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
			httpResp := func(obj runtime.Object) *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
			}
			tf.Client = &restfake.RESTClient{
				GroupVersion:         schema.GroupVersion{Group: "version", Version: ""},
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: restfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					urlPrefix := "/version"
					return map[string]*http.Response{
						urlPrefix: httpResp(testing.FakeServices()),
					}[req.URL.Path], nil
				}),
			}
			tf.FakeDynamicClient = fake.NewSimpleDynamicClient(
				scheme.Scheme, addonObj)
			Expect(o.validate()).Should(HaveOccurred())
		})
	})

	// When("Enable an addon", func() {
	// 	It("should set addon.spec.install.enabled=true", func() {
	// 		By("Checking install helm chart by fake helm action config")
	// 		enableCmd := newEnableCmd(tf, streams)
	// 		enableCmd.Run(enableCmd, []string{"my-addon"})
	// 	})
	// })
	//
	// When("Disable an addon", func() {
	// 	It("should set addon.spec.install.enabled=false", func() {
	// 		By("Checking install helm chart by fake helm action config")
	// 		disableCmd := newDisableCmd(tf, streams)
	// 		disableCmd.Run(disableCmd, []string{"my-addon"})
	// 	})
	// })
})
