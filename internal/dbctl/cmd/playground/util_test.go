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

package playground

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
)

var _ = Describe("util", func() {
	clusterName = "dbctl-playground-test"
	dbClusterName = "dbctl-playground-test-cluster"

	mockClient := func(data runtime.Object) *cmdtesting.TestFactory {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()

		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.Client = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, data)},
		}
		return tf
	}

	It("build cluster info", func() {
		var (
			clusterInfo = &ClusterInfo{
				HostIP:        "",
				CloudProvider: "",
				KubeConfig:    util.ConfigPath(clusterName),
				GrafanaPort:   "9100",
				GrafanaUser:   "admin",
				GrafanaPasswd: "prom-operator",
			}
		)

		tf := mockClient(&corev1.ConfigMap{})
		Expect(buildClusterInfo(clusterInfo, "default", dbClusterName)).Should(HaveOccurred())

		// test builder
		builder := &builder{}
		builder.namespace = "default"
		builder.name = dbClusterName
		clientSet, err := tf.KubernetesClientSet()
		Expect(err).Should(BeNil())
		builder.clientSet = clientSet

		dynamicClient, err := tf.DynamicClient()
		Expect(err).Should(BeNil())
		builder.dynamicClient = dynamicClient

		// get cluster
		builder.groupKind = schema.GroupKind{Group: "dbaas.infracreate.com", Kind: "Cluster"}
		Expect(builder.getClusterObject(clusterInfo)).Should(HaveOccurred())

		// get statefulset
		builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", dbClusterName)
		builder.groupKind = schema.GroupKind{Kind: "StatefulSet"}
		Expect(builder.getClusterObject(clusterInfo)).Should(HaveOccurred())

		// get deployment
		builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", dbClusterName)
		builder.groupKind = schema.GroupKind{Kind: "Deployment"}
		Expect(builder.getClusterObject(clusterInfo)).Should(HaveOccurred())

		// get service
		builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", dbClusterName)
		builder.groupKind = schema.GroupKind{Kind: "Service"}
		Expect(builder.getClusterObject(clusterInfo)).Should(HaveOccurred())

		// get secret
		builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", dbClusterName)
		builder.groupKind = schema.GroupKind{Kind: "Secret"}
		Expect(builder.getClusterObject(clusterInfo)).Should(HaveOccurred())

		// get pod
		builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", dbClusterName)
		builder.groupKind = schema.GroupKind{Kind: "Pod"}
		Expect(builder.getClusterObject(clusterInfo)).Should(HaveOccurred())
	})
})
