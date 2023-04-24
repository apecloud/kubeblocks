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

package clusterversion

import (
	"bytes"
	"fmt"
	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	_ "k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	_ "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"net/http"
	_ "net/http"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/constant"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/kubernetes/scheme"
	//cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("clusterversion", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	buf := new(bytes.Buffer)
	const ClusterVersionName = testing.ClusterVersionName
	const ClusterVersionNameV1 = testing.ClusterVersionName + "v1"
	const ClusterDefName = testing.ClusterDefName
	const ClusterDefNameV1 = testing.ClusterDefName + "v1"

	//mockClient := func(data runtime.Object) *cmdtesting.TestFactory {
	//	tf := cmdtesting.NewTestFactory()
	//	defer tf.Cleanup()
	//
	//	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
	//	tf.UnstructuredClient = &fake.RESTClient{
	//		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
	//		Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, data)},
	//	}
	//	return tf
	//}

	BeforeEach(func() {
		streams, _, buf, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory()
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("clusterversion cmd", func() {
		cmd := NewClusterVersionCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
	})

	It("list", func() {
		cmd := NewListCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("list --cluster-definition", func() {
		cv1 := testing.FakeClusterVersion()
		cv2 := testing.FakeClusterVersion()
		cv2.Name = ClusterVersionNameV1

		cv2.SetLabels(map[string]string{
			constant.ClusterDefLabelKey: ClusterDefNameV1,
		})
		cv2.Spec.ClusterDefinitionRef = ClusterDefNameV1
		tf.FakeDynamicClient = testing.FakeDynamicClient(cv1, cv2)

		cvlist := &v1alpha1.ClusterVersionList{
			TypeMeta: metav1.TypeMeta{},
			ListMeta: metav1.ListMeta{},
			Items: []v1alpha1.ClusterVersion{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            ClusterVersionName,
						ResourceVersion: "10",
					}, Spec: v1alpha1.ClusterVersionSpec{
						ClusterDefinitionRef: ClusterVersionNameV1,
						ComponentVersions:    nil,
					},
				},
			},
		}
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.UnstructuredClient = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, cvlist)},
		}

		cmd := NewListCmd(tf, streams)
		cmd.SetOut(buf)
		Expect(cmd).ShouldNot(BeNil())
		//		expect := `NAME                      CLUSTER-DEFINITION    STATUS      AGE
		//
		//`
		cmd.Run(cmd, []string{})
		fmt.Println(buf)
	})
})
