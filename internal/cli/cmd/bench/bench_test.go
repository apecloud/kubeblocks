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

package bench

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("bench", func() {
	const (
		namespace   = "default"
		clusterName = "test"
	)

	var (
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
		cluster = testing.FakeCluster(clusterName, namespace)
		pods    = testing.FakePods(3, namespace, clusterName)
	)
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		tf.Client = &clientfake.RESTClient{}
		tf.FakeDynamicClient = testing.FakeDynamicClient()
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		httpResp := func(obj runtime.Object) *http.Response {
			return &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, obj)}
		}

		tf.UnstructuredClient = &clientfake.RESTClient{
			GroupVersion:         schema.GroupVersion{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion},
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: clientfake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				urlPrefix := "/api/v1/namespaces/" + namespace
				mapping := map[string]*http.Response{
					"/api/v1/nodes/" + testing.NodeName:   httpResp(testing.FakeNode()),
					urlPrefix + "/services":               httpResp(&corev1.ServiceList{}),
					urlPrefix + "/events":                 httpResp(&corev1.EventList{}),
					urlPrefix + "/persistentvolumeclaims": httpResp(&corev1.PersistentVolumeClaimList{}),
					urlPrefix + "/pods":                   httpResp(pods),
				}
				return mapping[req.URL.Path], nil
			}),
		}

		tf.Client = tf.UnstructuredClient
		tf.FakeDynamicClient = testing.FakeDynamicClient(cluster, testing.FakeClusterDef(), testing.FakeClusterVersion())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("bench command", func() {
		cmd := NewBenchCmd(tf, streams)
		Expect(cmd != nil).Should(BeTrue())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
	})

	It("sysbench command", func() {
		cmd := NewSysBenchCmd(tf, streams)
		Expect(cmd != nil).Should(BeTrue())
	})

	It("test sysbench run", func() {
		o := &SysBenchOptions{
			BenchBaseOptions: BenchBaseOptions{
				Driver:      "mysql",
				Database:    "test",
				Host:        "svc-1",
				Port:        3306,
				User:        "test",
				Password:    "test",
				ClusterName: "test",
				factory:     tf,
				namespace:   namespace,
				IOStreams:   streams,
			},
			Type:     []string{"oltp_read_only"},
			Tables:   1,
			Size:     100,
			Duration: 60,
		}
		o.dynamic, _ = tf.DynamicClient()
		o.client, _ = tf.KubernetesClientSet()
		Expect(o.Run()).Should(BeNil())
	})

	It("pgbench command", func() {
		cmd := NewPgBenchCmd(tf, streams)
		Expect(cmd != nil).Should(BeTrue())
	})

	It("test pgbench run", func() {
		o := &PgBenchOptions{
			BenchBaseOptions: BenchBaseOptions{
				Driver:      pgBenchDriver,
				Database:    "test",
				Host:        "svc-1",
				Port:        3306,
				User:        "test",
				Password:    "test",
				ClusterName: "test",
				factory:     tf,
				namespace:   namespace,
				IOStreams:   streams,
			},
			Scale:   100,
			Clients: []int{1},
		}
		o.dynamic, _ = tf.DynamicClient()
		o.client, _ = tf.KubernetesClientSet()
		Expect(o.Run()).Should(BeNil())
	})

	It("ycsb command", func() {
		cmd := NewYcsbCmd(tf, streams)
		Expect(cmd != nil).Should(BeTrue())
	})

	It("test ycsb run", func() {
		o := &YcsbOptions{
			BenchBaseOptions: BenchBaseOptions{
				Driver:      "mysql",
				Database:    "test",
				Host:        "svc-1",
				Port:        3306,
				User:        "test",
				Password:    "test",
				ClusterName: "test",
				factory:     tf,
				namespace:   namespace,
				IOStreams:   streams,
			},
			RecordCount:    1000,
			OperationCount: 1000,
			Threads:        []int{1},
		}
		o.dynamic, _ = tf.DynamicClient()
		o.client, _ = tf.KubernetesClientSet()
		Expect(o.Run()).Should(BeNil())
	})

	It("parse driver and endpoint", func() {
		driver, host, port, err := getDriverAndHostAndPort(cluster, testing.FakeServices())
		Expect(err).Should(BeNil())
		Expect(driver).Should(Equal(testing.ComponentName))
		Expect(host).Should(Equal(fmt.Sprintf("svc-1.%s.svc.cluster.local", testing.Namespace)))
		Expect(port).Should(Equal(3306))
	})

	It("parse tolerations", func() {
		o := &BenchBaseOptions{
			TolerationsRaw: []string{"dev=true:NoSchedule,large:NoSchedule"},
		}
		err := o.BaseComplete()
		Expect(err).Should(BeNil())
		Expect(o.Tolerations).Should(Equal([]corev1.Toleration{
			{Key: "dev", Operator: corev1.TolerationOpEqual, Value: "true", Effect: corev1.TaintEffectNoSchedule},
			{Key: "large", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
		}))
	})
})
