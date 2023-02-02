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

package testing

import (
	"fmt"
	"time"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	ClusterName        = "fake-cluster-name"
	Namespace          = "fake-namespace"
	ClusterVersionName = "fake-cluster-version"
	ClusterDefName     = "fake-cluster-definition"
	ComponentName      = "fake-component-name"
	ComponentType      = "fake-component-type"
	NodeName           = "fake-node-name"
	SecretName         = "fake-secret-name"
	StorageClassName   = "fake-storage-class"
	PVCName            = "fake-pvc"

	KubeBlocksChartName = "fake-kubeblocks"
	KubeBlocksChartURL  = "fake-kubeblocks-chart-url"
	BackupToolName      = "fake-backup-tool"
)

func GetRandomStr() string {
	seq, _ := password.Generate(6, 2, 0, true, true)
	return seq
}

func FakeCluster(name string, namespace string) *dbaasv1alpha1.Cluster {
	return &dbaasv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: dbaasv1alpha1.ClusterStatus{
			Phase: dbaasv1alpha1.RunningPhase,
			Components: map[string]dbaasv1alpha1.ClusterStatusComponent{
				ComponentName: {
					ConsensusSetStatus: &dbaasv1alpha1.ConsensusSetStatus{
						Leader: dbaasv1alpha1.ConsensusMemberStatus{
							Name:       "leader",
							AccessMode: dbaasv1alpha1.ReadWrite,
							Pod:        fmt.Sprintf("%s-pod-0", name),
						},
					},
				},
			},
		},
		Spec: dbaasv1alpha1.ClusterSpec{
			ClusterDefRef:     ClusterDefName,
			ClusterVersionRef: ClusterVersionName,
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

func FakePods(replicas int, namespace string, cluster string) *corev1.PodList {
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
			types.InstanceLabelKey:  cluster,
			types.RoleLabelKey:      role,
			types.ComponentLabelKey: ComponentName,
			types.NameLabelKey:      "state.mysql-apecloud-mysql",
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

func FakeSecrets(namespace string, cluster string) *corev1.SecretList {
	secret := corev1.Secret{}
	secret.Name = SecretName
	secret.Namespace = namespace
	secret.Type = corev1.SecretTypeServiceAccountToken
	secret.Labels = map[string]string{
		types.InstanceLabelKey:           cluster,
		intctrlutil.AppManagedByLabelKey: intctrlutil.AppName,
	}

	secret.Data = map[string][]byte{
		corev1.ServiceAccountTokenKey: []byte("fake-secret-token"),
		"fake-secret-key":             []byte("fake-secret-value"),
		"username":                    []byte("test-user"),
		"password":                    []byte("test-password"),
	}
	return &corev1.SecretList{Items: []corev1.Secret{secret}}
}

func FakeNode() *corev1.Node {
	node := &corev1.Node{}
	node.Name = NodeName
	node.Labels = map[string]string{
		types.RegionLabelKey: "fake-node-region",
		types.ZoneLabelKey:   "fake-node-zone",
	}
	return node
}

func FakeClusterDef() *dbaasv1alpha1.ClusterDefinition {
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	clusterDef.Name = ClusterDefName
	clusterDef.Spec.Components = []dbaasv1alpha1.ClusterDefinitionComponent{
		{
			TypeName:        ComponentType,
			DefaultReplicas: 3,
		},
		{
			TypeName:        fmt.Sprintf("%s-%d", ComponentType, 1),
			DefaultReplicas: 2,
		},
	}
	clusterDef.Spec.Type = "state.mysql"
	return clusterDef
}

func FakeClusterVersion() *dbaasv1alpha1.ClusterVersion {
	cv := &dbaasv1alpha1.ClusterVersion{}
	cv.Name = ClusterVersionName
	cv.SetLabels(map[string]string{types.ClusterDefLabelKey: ClusterDefName})
	cv.Spec.ClusterDefinitionRef = ClusterDefName
	cv.SetCreationTimestamp(metav1.Now())
	return cv
}

func FakeBackupTool() *dpv1alpha1.BackupTool {
	tool := &dpv1alpha1.BackupTool{}
	tool.Name = BackupToolName
	return tool
}

func FakeServices() *corev1.ServiceList {
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
				Name:      fmt.Sprintf("svc-%d", idx),
				Namespace: Namespace,
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

func FakePVCs() *corev1.PersistentVolumeClaimList {
	pvcs := &corev1.PersistentVolumeClaimList{}
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      PVCName,
			Labels: map[string]string{
				types.InstanceLabelKey:  ClusterName,
				types.ComponentLabelKey: ComponentName,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: pointer.String(StorageClassName),
			AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}
	pvcs.Items = append(pvcs.Items, pvc)
	return pvcs
}

func FakeEvents() *corev1.EventList {
	eventList := &corev1.EventList{}
	fakeEvent := func(name string, createTime metav1.Time) corev1.Event {
		e := corev1.Event{}
		e.Name = name
		e.Type = "Warning"
		e.SetCreationTimestamp(createTime)
		e.LastTimestamp = createTime
		return e
	}

	parseTime := func(t string) time.Time {
		time, _ := time.Parse(time.RFC3339, t)
		return time
	}

	for _, e := range []struct {
		name       string
		createTime metav1.Time
	}{
		{
			name:       "e1",
			createTime: metav1.NewTime(parseTime("2023-01-04T00:00:00.000Z")),
		},
		{
			name:       "e2",
			createTime: metav1.NewTime(parseTime("2023-01-04T01:00:00.000Z")),
		},
	} {
		eventList.Items = append(eventList.Items, fakeEvent(e.name, e.createTime))
	}
	return eventList
}
