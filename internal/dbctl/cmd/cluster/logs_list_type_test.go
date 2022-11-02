/*
Copyright ApeCloud Inc.

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
	"net/http"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("logs_list_type test", func() {
	It("logs_list_type", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		ns := scheme.Codecs.WithoutConversion()
		tf.Client = &fake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: "", Version: "v1"},
			NegotiatedSerializer: ns,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				body := cmdtesting.ObjBody(codec, execPod())
				return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: body}, nil
			}),
		}
		tf.ClientConfigVal = &restclient.Config{APIPath: "/api", ContentConfig: restclient.ContentConfig{NegotiatedSerializer: scheme.Codecs, GroupVersion: &schema.GroupVersion{Version: "v1"}}}

		stream := genericclioptions.NewTestIOStreamsDiscard()
		o := &LogsListOptions{
			factory:   tf,
			IOStreams: stream,
		}

		// validate without args
		Expect(o.Validate([]string{})).Should(MatchError("must specify the cluster name"))

		// validate with args
		Expect(o.Validate([]string{"cluster-name"})).Should(BeNil())
		Expect(o.Complete(o.factory, []string{"cluster-name"})).Should(BeNil())
		Expect(o.clusterName).Should(Equal("cluster-name"))
	})
	It("printLogContext test", func() {
		dataObj := &types.ClusterObjects{
			Cluster:    &dbaasv1alpha1.Cluster{},
			ClusterDef: &dbaasv1alpha1.ClusterDefinition{},
			Pods:       &corev1.PodList{},
		}
		dataObj.ClusterDef.Spec.Type = "state.mysql"
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{},
		}
		pod.Name = "instName"
		pod.Labels = map[string]string{types.ComponentLabelKey: "component-name"}
		dataObj.Pods.Items = append(dataObj.Pods.Items, pod)
		Expect(printLogContext(dataObj, os.Stdout, "instName")).Should(BeNil())
	})
})
