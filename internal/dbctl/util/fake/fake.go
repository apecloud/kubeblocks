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

package fake

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

const (
	ClusterName    = "fake-cluster-name"
	Namespace      = "fake-namespace"
	AppVersionName = "fake-appversion"
	ClusterDefName = "fake-cluster-definition"
	ComponentName  = "fake-component-name"
	ComponentType  = "fake-component-type"
	NodeName       = "fake-node-name"
	SecretName     = "fake-secret-name"
)

func Cluster(name string, namespace string) *dbaasv1alpha1.Cluster {
	return &dbaasv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: dbaasv1alpha1.ClusterStatus{
			Phase: dbaasv1alpha1.RunningPhase,
			Components: map[string]*dbaasv1alpha1.ClusterStatusComponent{
				ComponentName: {
					ConsensusSetStatus: &dbaasv1alpha1.ConsensusSetStatus{
						Leader: dbaasv1alpha1.ConsensusMemberStatus{
							Name:       "leader",
							AccessMode: dbaasv1alpha1.ReadWrite,
							Pod:        "leader-pod",
						},
					},
				},
			},
		},
		Spec: dbaasv1alpha1.ClusterSpec{
			ClusterDefRef:     ClusterDefName,
			AppVersionRef:     AppVersionName,
			TerminationPolicy: dbaasv1alpha1.WipeOut,
			Components: []dbaasv1alpha1.ClusterComponent{
				{
					Name: ComponentName,
					Type: ComponentType,
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

func Pods(replicas int, namespace string, cluster string) *corev1.PodList {
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
			types.ComponentLabelKey:        ComponentName,
		}
		pod.Spec.NodeName = NodeName
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

func Secrets(namespace string, cluster string) *corev1.SecretList {
	secret := corev1.Secret{}
	secret.Name = SecretName
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

func Node() *corev1.Node {
	node := &corev1.Node{}
	node.Name = NodeName
	node.Labels = map[string]string{
		types.RegionLabelKey: "fake-node-region",
		types.ZoneLabelKey:   "fake-node-zone",
	}
	return node
}

func ClusterDef() *dbaasv1alpha1.ClusterDefinition {
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	clusterDef.Name = ClusterDefName
	clusterDef.Spec.Components = []dbaasv1alpha1.ClusterDefinitionComponent{
		{
			TypeName:        ComponentType,
			DefaultReplicas: 3,
		},
	}
	return clusterDef
}

func Appversion() *dbaasv1alpha1.AppVersion {
	appversion := &dbaasv1alpha1.AppVersion{}
	appversion.Name = AppVersionName
	appversion.Spec.ClusterDefinitionRef = ClusterDefName
	return appversion
}

func Services() *corev1.ServiceList {
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
					types.InstanceLabelKey:  ClusterName,
					types.ComponentLabelKey: ComponentName,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Port: 3306}},
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

func NewClientSet(objects ...runtime.Object) *kubefakeclient.Clientset {
	return kubefakeclient.NewSimpleClientset(objects...)
}

func NewDynamicClient(objects ...runtime.Object) *dynamicfakeclient.FakeDynamicClient {
	_ = dbaasv1alpha1.AddToScheme(scheme.Scheme)
	return dynamicfakeclient.NewSimpleDynamicClient(scheme.Scheme, objects...)
}
