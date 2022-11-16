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

package describe

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
)

const (
	clusterName = "cluster-test"
	namespace   = "test"
)

var _ = Describe("Describer", func() {
	tf := cmdtesting.NewTestFactory().WithNamespace("test")
	defer tf.Cleanup()

	It("describer map", func() {
		describer, err := DescriberFn(tf, &meta.RESTMapping{
			Resource:         types.ClusterGVR(),
			GroupVersionKind: types.ClusterGK().WithVersion(types.Version),
		})
		Expect(describer).ShouldNot(BeNil())
		Expect(err).Should(Succeed())

		describer, err = DescriberFn(tf, &meta.RESTMapping{
			Resource: schema.GroupVersionResource{
				Group:    types.Group,
				Version:  types.Version,
				Resource: "tests",
			},
			GroupVersionKind: schema.GroupVersionKind{
				Group:   types.Group,
				Version: types.Version,
				Kind:    "test",
			}})
		Expect(describer).ShouldNot(BeNil())
		Expect(err).Should(Succeed())
	})

	Context("describe cluster", func() {
		It("describe return error", func() {
			fake := kubefakeclient.NewSimpleClientset(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: "foo",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "fooaccount",
				},
			})
			scheme := runtime.NewScheme()
			dynamicClient := dynamicfakeclient.NewSimpleDynamicClient(scheme)

			describer := &ClusterDescriber{
				client:  fake,
				dynamic: dynamicClient,
			}
			describerSettings := describe.DescriberSettings{ShowEvents: true, ChunkSize: cmdutil.DefaultChunkSize}
			res, err := describer.Describe("test", "test", describerSettings)
			Expect(res).Should(Equal(""))
			Expect(err).Should(HaveOccurred())
		})

		It("mock cluster and check", func() {
			describer := ClusterDescriber{ClusterObjects: fakeClusterObjs()}
			res, err := describer.describeCluster(nil)
			Expect(res).ShouldNot(BeNil())
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

const (
	appVersionName = "fake-cluster-appversion"
	clusterDefName = "fake-cluster-definition"
	componentName  = "fake-component-name"
	componentType  = "fake-component-type"
	nodeName       = "fake-node-name"
	secretName     = "fake-secret-name"
)

func fakeClusterObjs() *types.ClusterObjects {
	clusterObjs := cluster.NewClusterObjects()
	clusterObjs.Cluster = fakeCluster(clusterName, namespace)
	clusterObjs.ClusterDef = fakeClusterDef()
	clusterObjs.Pods = fakePods(3, namespace, clusterName)
	clusterObjs.Secrets = fakeSecrets(namespace, clusterName)
	clusterObjs.Nodes = []*corev1.Node{fakeNode()}
	clusterObjs.Services = fakeServices()
	return clusterObjs
}

func fakeCluster(name string, namespace string) *dbaasv1alpha1.Cluster {
	return &dbaasv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: dbaasv1alpha1.ClusterStatus{
			Phase: dbaasv1alpha1.RunningPhase,
		},
		Spec: dbaasv1alpha1.ClusterSpec{
			ClusterDefRef:     clusterDefName,
			AppVersionRef:     appVersionName,
			TerminationPolicy: dbaasv1alpha1.WipeOut,
			Components: []dbaasv1alpha1.ClusterComponent{
				{
					Name: componentName,
					Type: componentType,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
					VolumeClaimTemplates: []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate{
						{
							Name: "data",
							Spec: &corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("1Gi"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func fakePods(replicas int, namespace string, cluster string) *corev1.PodList {
	pods := &corev1.PodList{}
	for i := 0; i < replicas; i++ {
		role := "follower"
		pod := corev1.Pod{}
		pod.Name = fmt.Sprintf("%s-pod-%d", cluster, i)
		pod.Namespace = namespace

		if i == 0 {
			role = "leader"
		}

		pod.Labels = map[string]string{
			types.InstanceLabelKey:         cluster,
			types.ConsensusSetRoleLabelKey: role,
			types.ComponentLabelKey:        componentName,
		}
		pod.Spec.NodeName = nodeName
		pod.Spec.Containers = []corev1.Container{
			{
				Name:  "fake-container",
				Image: "fake-container-image",
			},
		}
		pod.Status.Phase = corev1.PodRunning
		pods.Items = append(pods.Items, pod)
	}
	return pods
}

func fakeSecrets(namespace string, cluster string) *corev1.SecretList {
	secret := corev1.Secret{}
	secret.Name = secretName
	secret.Namespace = namespace
	secret.Type = corev1.SecretTypeServiceAccountToken
	secret.Labels = map[string]string{
		types.InstanceLabelKey: cluster,
	}

	secret.Data = map[string][]byte{
		corev1.ServiceAccountTokenKey: []byte("fake-secret-token"),
		"fake-secret-key":             []byte("fake-secret-value"),
	}
	return &corev1.SecretList{Items: []corev1.Secret{secret}}
}

func fakeNode() *corev1.Node {
	node := &corev1.Node{}
	node.Name = nodeName
	node.Labels = map[string]string{
		types.RegionLabelKey: "fake-node-region",
		types.ZoneLabelKey:   "fake-node-zone",
	}
	return node
}

func fakeClusterDef() *dbaasv1alpha1.ClusterDefinition {
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	clusterDef.Name = clusterDefName
	clusterDef.Spec.Components = []dbaasv1alpha1.ClusterDefinitionComponent{
		{
			TypeName:        componentType,
			DefaultReplicas: 3,
		},
	}
	return clusterDef
}

func fakeServices() *corev1.ServiceList {
	cases := []struct {
		exposed    bool
		clusterIP  string
		floatingIP string
	}{
		{false, "", ""},
		{false, "192.168.0.1", ""},
		{true, "192.168.0.1", ""},
		{true, "192.168.0.1", "172.31.0.4"},
	}

	var services []corev1.Service
	for idx, item := range cases {
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("svc-%d", idx),
				Labels: map[string]string{
					types.InstanceLabelKey:  clusterName,
					types.ComponentLabelKey: componentName,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}

		if item.clusterIP == "" {
			svc.Spec.ClusterIP = "None"
		} else {
			svc.Spec.ClusterIP = item.clusterIP
		}

		annotations := make(map[string]string)
		if item.floatingIP != "" {
			annotations[types.ServiceFloatingIPAnnotationKey] = item.floatingIP
		}
		if item.exposed {
			annotations[types.ServiceLBTypeAnnotationKey] = types.ServiceLBTypeAnnotationValue
		}
		svc.ObjectMeta.SetAnnotations(annotations)

		services = append(services, svc)
	}
	return &corev1.ServiceList{Items: services}
}
