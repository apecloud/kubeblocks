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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/describe"

	"github.com/apecloud/kubeblocks/internal/dbctl/engine"
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
		Expect(o.Run()).Should(HaveOccurred())
	})
	It("printLogContext test", func() {

		mysqlLogsContext := map[string]engine.LogVariables{
			"stdout": {
				DefaultFilePath: "stdout/stderr",
				Variables:       nil,
				PathVar:         "",
			},
			"error": {
				DefaultFilePath: "/data/mysql/log/mysqld.err",
				Variables:       []string{"log-error"},
				PathVar:         "log-error",
			},
			"slow": {
				DefaultFilePath: "/data/mysql/data/release-name-replicasets-0-slow.log",
				Variables:       []string{"slow_query_log_file", "slow_query_log", "long_query_time", "log_output"},
				PathVar:         "slow_query_log_file",
			},
		}
		w := describe.NewPrefixWriter(os.Stdout)
		Expect(printLogContext(mysqlLogsContext, w)).Should(BeNil())
	})
})
