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

package cluster

import (
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	cmdexec "k8s.io/kubectl/pkg/cmd/exec"
	cmdlogs "k8s.io/kubectl/pkg/cmd/logs"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("logs", func() {
	It("isStdoutForContainer Test", func() {
		o := &LogsOptions{}
		Expect(o.isStdoutForContainer()).Should(BeTrue())
		o.fileType = "stdout"
		Expect(o.isStdoutForContainer()).Should(BeTrue())
		o.fileType = "slow"
		Expect(o.isStdoutForContainer()).Should(BeFalse())
		o.filePath = "/var/log/yum.log"
		Expect(o.isStdoutForContainer()).Should(BeFalse())
	})

	It("prefixingWriter Test", func() {
		pw := &prefixingWriter{
			prefix: []byte("prefix"),
			writer: os.Stdout,
		}
		n, _ := pw.Write([]byte(""))
		Expect(n).Should(Equal(0))
		num, _ := pw.Write([]byte("test"))
		Expect(num).Should(Equal(4))
	})

	It("assembleTailCommand Test", func() {
		command := assembleTail(true, 1, 100)
		Expect(command).ShouldNot(BeNil())
		Expect(command).Should(Equal("tail -f --lines=1 --bytes=100"))
	})

	It("addPrefixIfNeeded Test", func() {
		l := &LogsOptions{
			ExecOptions: &exec.ExecOptions{
				StreamOptions: cmdexec.StreamOptions{
					ContainerName: "container",
				},
			},
		}
		// no set prefix
		w := l.addPrefixIfNeeded(corev1.ObjectReference{}, os.Stdout)
		Expect(w).Should(Equal(os.Stdout))
		// set prefix
		o := corev1.ObjectReference{
			Name:      "name",
			FieldPath: "FieldPath",
		}
		l.logOptions.Prefix = true
		w = l.addPrefixIfNeeded(o, os.Stdout)
		_, ok := w.(*prefixingWriter)
		Expect(ok).Should(BeTrue())
	})

	It("new logs command Test", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		ns := scheme.Codecs.WithoutConversion()
		tf.Client = &fake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: "", Version: "v1"},
			NegotiatedSerializer: ns,
			Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				body := cmdtesting.ObjBody(codec, mockPod())
				return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: body}, nil
			}),
		}
		tf.ClientConfigVal = &restclient.Config{APIPath: "/api", ContentConfig: restclient.ContentConfig{NegotiatedSerializer: scheme.Codecs, GroupVersion: &schema.GroupVersion{Version: "v1"}}}

		stream := genericclioptions.NewTestIOStreamsDiscard()
		l := &LogsOptions{
			ExecOptions: exec.NewExecOptions(tf, stream),
			logOptions: cmdlogs.LogsOptions{
				IOStreams: stream,
			},
		}

		cmd := NewLogsCmd(tf, stream)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.Use).ShouldNot(BeNil())
		Expect(cmd.Example).ShouldNot(BeNil())

		// Complete without args
		Expect(l.complete([]string{})).Should(MatchError("cluster name or instance name should be specified"))
		// Complete with args
		l.PodName = "foo"
		l.Client, _ = l.Factory.KubernetesClientSet()
		l.filePath = "/var/log"
		Expect(l.complete([]string{"cluster-name"})).Should(HaveOccurred())
		Expect(l.clusterName).Should(Equal("cluster-name"))
		// Validate stdout
		l.filePath = ""
		l.fileType = ""
		l.Namespace = "test"
		l.logOptions.SinceSeconds = time.Minute
		Expect(l.complete([]string{"cluster-name"})).Should(Succeed())
		Expect(l.validate()).Should(Succeed())
		Expect(l.logOptions.Options).ShouldNot(BeNil())

	})

	It("createFileTypeCommand Test", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foo",
				Namespace:       "test",
				ResourceVersion: "10",
				Labels: map[string]string{
					"app.kubernetes.io/name":        "mysql-apecloud-mysql",
					constant.KBAppComponentLabelKey: "component-name",
				},
			},
		}
		obj := cluster.NewClusterObjects()
		l := &LogsOptions{}
		// corner case
		cmd, err := l.createFileTypeCommand(pod, obj)
		Expect(cmd).Should(Equal(""))
		Expect(err).Should(HaveOccurred())
		// normal case
		obj.Cluster = &appsv1alpha1.Cluster{
			Spec: appsv1alpha1.ClusterSpec{
				ComponentSpecs: []appsv1alpha1.ClusterComponentSpec{
					{
						Name:            "component-name",
						ComponentDefRef: "component-type",
					},
				},
			},
		}
		obj.ClusterDef = &appsv1alpha1.ClusterDefinition{
			Spec: appsv1alpha1.ClusterDefinitionSpec{
				ComponentDefs: []appsv1alpha1.ClusterComponentDefinition{
					{
						Name: "component-type",
						LogConfigs: []appsv1alpha1.LogConfig{
							{
								Name:            "slow",
								FilePathPattern: "/log/mysql/*slow.log",
							},
							{
								Name:            "error",
								FilePathPattern: "/log/mysql/*.err",
							},
						},
					},
				},
			},
		}
		l.fileType = "slow"
		cmd, err = l.createFileTypeCommand(pod, obj)
		Expect(err).Should(BeNil())
		Expect(cmd).Should(Equal("ls /log/mysql/*slow.log | xargs tail --lines=0"))
		// error case
		l.fileType = "slow-error"
		cmd, err = l.createFileTypeCommand(pod, obj)
		Expect(err).Should(HaveOccurred())
		Expect(cmd).Should(Equal(""))
	})
})
