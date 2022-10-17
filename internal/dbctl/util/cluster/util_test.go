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

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var _ = Describe("cluster util", func() {
	clusterName := "test-cluster"
	namespace := "test"

	mockClient := func(data runtime.Object) *cmdtesting.TestFactory {
		tf := cmdtesting.NewTestFactory().WithNamespace(namespace)
		defer tf.Cleanup()

		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.Client = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, data)},
		}
		return tf
	}

	It("get cluster objects", func() {
		clusterName := "test-cluster"
		objs := &types.ClusterObjects{}
		tf := mockClient(&corev1.ConfigMap{})

		// test builder
		builder := &builder{}
		builder.namespace = namespace
		builder.name = clusterName
		clientSet, err := tf.KubernetesClientSet()
		Expect(err).Should(BeNil())
		builder.clientSet = clientSet

		dynamicClient, err := tf.DynamicClient()
		Expect(err).Should(BeNil())
		builder.dynamicClient = dynamicClient

		// get cluster
		builder.withGK(schema.GroupKind{Group: "dbaas.infracreate.com", Kind: "Cluster"})
		Expect(builder.do(objs)).Should(HaveOccurred())

		// get statefulset
		builder.withLabel(InstanceLabel(clusterName)).withGK(schema.GroupKind{Kind: "StatefulSet"})
		Expect(builder.do(objs)).Should(HaveOccurred())

		// get deployment
		builder.withGK(schema.GroupKind{Kind: "Deployment"})
		Expect(builder.do(objs)).Should(HaveOccurred())

		// get service
		builder.withGK(schema.GroupKind{Kind: "Service"})
		Expect(builder.do(objs)).Should(HaveOccurred())

		// get secret
		builder.withGK(schema.GroupKind{Kind: "Secret"})
		Expect(builder.do(objs)).Should(HaveOccurred())

		// get pod
		builder.withGK(schema.GroupKind{Kind: "Pod"})
		Expect(builder.do(objs)).Should(HaveOccurred())
	})

	It("get all objects", func() {
		objs := &types.ClusterObjects{}
		tf := mockClient(&corev1.ConfigMap{})
		clientSet, _ := tf.KubernetesClientSet()
		dynamicClient, _ := tf.DynamicClient()
		Expect(GetAllObjects(clientSet, dynamicClient, namespace, clusterName, objs)).Should(HaveOccurred())
	})
})
