package clusterdefinition

import (
	"bytes"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/scheme"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("clusterdefinition list components", func() {
	var (
		cmd     *cobra.Command
		streams genericclioptions.IOStreams
		out     *bytes.Buffer
		tf      *cmdtesting.TestFactory
	)

	const (
		namespace             = testing.Namespace
		clusterdefinitionName = testing.ClusterDefName
	)

	mockClient := func(data runtime.Object) *cmdtesting.TestFactory {
		tf := testing.NewTestFactory(namespace)
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.UnstructuredClient = &clientfake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, data)},
		}
		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(data)
		return tf
	}

	BeforeEach(func() {
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		clusterDef := testing.FakeClusterDef()
		tf = mockClient(clusterDef)
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		cmd = NewListComponentsCmd(tf, streams)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("create list-components cmd", func() {
		cmd := NewListComponentsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("list-components requires a clusterdefinition Name", func() {
		Expect(validate([]string{})).Should(HaveOccurred())
	})

	It("list-components ", func() {
		cmd.Run(cmd, []string{clusterdefinitionName})
		expected := `NAME                    WORKLOADTYPE   CHARACTERTYPE   
fake-component-type                    mysql           
fake-component-type-1                  mysql           
`
		Expect(expected).Should(Equal(out.String()))
	})
})
