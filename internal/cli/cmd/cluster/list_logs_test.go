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

package cluster

import (
	"net/http"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("listLogs test", func() {
	It("listLogs", func() {
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
		o := &ListLogsOptions{
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
	It("printContext test", func() {
		dataObj := &cluster.ClusterObjects{
			Cluster: &appsv1alpha1.Cluster{
				Spec: appsv1alpha1.ClusterSpec{
					ComponentSpecs: []appsv1alpha1.ClusterComponentSpec{
						{
							Name:            "component-name",
							ComponentDefRef: "component-type",
							EnabledLogs:     []string{"slow"},
						},
					},
				},
			},
			ClusterDef: &appsv1alpha1.ClusterDefinition{
				Spec: appsv1alpha1.ClusterDefinitionSpec{
					ComponentDefs: []appsv1alpha1.ClusterComponentDefinition{
						{
							Name: "component-type",
							LogConfigs: []appsv1alpha1.LogConfig{
								{
									Name:            "slow",
									FilePathPattern: "",
								},
							},
						},
					},
				},
			},
			Pods: &corev1.PodList{},
		}
		pod := corev1.Pod{
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
		dataObj.Pods.Items = append(dataObj.Pods.Items, pod)
		o := &ListLogsOptions{
			exec: &exec.ExecOptions{},
			IOStreams: genericclioptions.IOStreams{
				Out:    os.Stdout,
				ErrOut: os.Stdout,
			},
		}
		Expect(o.printListLogs(dataObj)).Should(BeNil())
	})

	It("convertToLogFileInfo test", func() {
		// empty case
		fileInfo := ""
		logFileList := convertToLogFileInfo(fileInfo, "type", "inst", "component")
		Expect(len(logFileList)).Should(Equal(0))
		// normal case, and size is 1
		fileInfo1 := "-rw-r----- 1 mysql mysql 6.1K Nov 23, 2022 21:37 (UTC+08:00) mysqld.err\n"
		logFileList1 := convertToLogFileInfo(fileInfo1, "type", "inst", "component")
		Expect(len(logFileList1)).Should(Equal(1))
		// normal case, and size is 2
		fileInfo2 := "-rw-r----- 1 mysql mysql 6.1K Nov 23, 2022 21:37 (UTC+08:00) mysqld.err\n-rw-r----- 1 root  root  1.7M Dec 08, 2022 10:07 (UTC+08:00) mysqld.err.1"
		logFileList2 := convertToLogFileInfo(fileInfo2, "type", "inst", "component")
		Expect(len(logFileList2)).Should(Equal(2))
	})
})
