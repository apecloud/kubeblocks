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
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var _ = Describe("Expose", func() {

	const (
		namespace   = "default"
		svcKind     = "service"
		svcVersion  = "v1"
		svcResource = "services"
		clusterName = "test-cluster"
	)

	newUnstructured := func(apiVersion, kind, namespace, name string, annotations map[string]interface{}, labels map[string]interface{}) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": apiVersion,
				"kind":       kind,
				"metadata": map[string]interface{}{
					"namespace":   namespace,
					"name":        name,
					"annotations": annotations,
					"labels":      labels,
				},
			},
		}
		return obj
	}

	newSvc := func(name string, clusterIP string, exposed bool) *unstructured.Unstructured {
		annotations := make(map[string]interface{})
		if exposed {
			annotations = map[string]interface{}{
				ServiceLBTypeAnnotationKey: ServiceLBTypeAnnotationValue,
			}
		}

		labels := map[string]interface{}{
			"app.kubernetes.io/instance": clusterName,
		}
		obj := newUnstructured(svcVersion, svcKind, svcResource, name, annotations, labels)
		if clusterIP != "" {
			_ = unstructured.SetNestedField(obj.Object, clusterIP, "spec", "clusterIP")
		}
		return obj
	}

	Context("Expose cluster and reverse", func() {
		It("", func() {
			tf := cmdtesting.NewTestFactory().WithNamespace(namespace)
			defer tf.Cleanup()

			var (
				streams, _, _, _ = genericclioptions.NewTestIOStreams()
				o                = &ExposeOptions{IOStreams: streams}
				objs             []runtime.Object
			)
			Expect(o.Complete(tf, []string{clusterName})).Should(Succeed())

			clusterObj := newUnstructured(fmt.Sprintf("%s/%s", types.Group, types.Version), types.KindCluster, namespace, clusterName, nil, nil)
			objs = append(objs, clusterObj)

			cases := []struct {
				exposed  bool
				headless bool
			}{
				// expose on normal service
				{false, false},

				// expose on headless service
				{false, true},
			}

			for idx, item := range cases {
				svcName := fmt.Sprintf("svc-%d", idx)

				var clusterIP string
				if !item.headless {
					clusterIP = fmt.Sprintf("192.168.0.%d", idx)
				}
				obj := newSvc(svcName, clusterIP, item.exposed)
				objs = append(objs, obj)
			}

			o.client = fake.NewSimpleDynamicClient(runtime.NewScheme(), objs...)

			Expect(o.Run()).Should(Succeed())
		})
	})
})
