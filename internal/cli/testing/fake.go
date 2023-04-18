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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	ClusterName        = "fake-cluster-name"
	Namespace          = "fake-namespace"
	ClusterVersionName = "fake-cluster-version"
	ClusterDefName     = "fake-cluster-definition"
	ComponentName      = "fake-component-name"
	ComponentDefName   = "fake-component-type"
	NodeName           = "fake-node-name"
	SecretName         = "fake-secret-conn-credential"
	StorageClassName   = "fake-storage-class"
	PVCName            = "fake-pvc"

	KubeBlocksRepoName  = "fake-kubeblocks-repo"
	KubeBlocksChartName = "fake-kubeblocks"
	KubeBlocksChartURL  = "fake-kubeblocks-chart-url"
	BackupToolName      = "fake-backup-tool"
)

func GetRandomStr() string {
	seq, _ := password.Generate(6, 2, 0, true, true)
	return seq
}

func FakeCluster(name, namespace string, conditions ...metav1.Condition) *appsv1alpha1.Cluster {
	var replicas int32 = 1
	return &appsv1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       types.KindCluster,
			APIVersion: fmt.Sprintf("%s/%s", types.AppsAPIGroup, types.AppsAPIVersion),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: appsv1alpha1.ClusterStatus{
			Phase: appsv1alpha1.RunningClusterPhase,
			Components: map[string]appsv1alpha1.ClusterComponentStatus{
				ComponentName: {
					ConsensusSetStatus: &appsv1alpha1.ConsensusSetStatus{
						Leader: appsv1alpha1.ConsensusMemberStatus{
							Name:       "leader",
							AccessMode: appsv1alpha1.ReadWrite,
							Pod:        fmt.Sprintf("%s-pod-0", name),
						},
					},
				},
			},
			Conditions: conditions,
		},
		Spec: appsv1alpha1.ClusterSpec{
			ClusterDefRef:     ClusterDefName,
			ClusterVersionRef: ClusterVersionName,
			TerminationPolicy: appsv1alpha1.WipeOut,
			ComponentSpecs: []appsv1alpha1.ClusterComponentSpec{
				{
					Name:            ComponentName,
					ComponentDefRef: ComponentDefName,
					Replicas:        replicas,
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
					VolumeClaimTemplates: []appsv1alpha1.ClusterComponentVolumeClaimTemplate{
						{
							Name: "data",
							Spec: appsv1alpha1.PersistentVolumeClaimSpec{
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
				{
					Name:            ComponentName + "-1",
					ComponentDefRef: ComponentDefName,
					Replicas:        replicas,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("100Mi"),
						},
					},
					VolumeClaimTemplates: []appsv1alpha1.ClusterComponentVolumeClaimTemplate{
						{
							Name: "data",
							Spec: appsv1alpha1.PersistentVolumeClaimSpec{
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
			constant.AppInstanceLabelKey:    cluster,
			constant.RoleLabelKey:           role,
			constant.KBAppComponentLabelKey: ComponentName,
			constant.AppNameLabelKey:        "mysql-apecloud-mysql",
			constant.AppManagedByLabelKey:   constant.AppName,
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
		constant.AppInstanceLabelKey:  cluster,
		constant.AppManagedByLabelKey: constant.AppName,
	}

	secret.Data = map[string][]byte{
		corev1.ServiceAccountTokenKey: []byte("fake-secret-token"),
		"fake-secret-key":             []byte("fake-secret-value"),
		"username":                    []byte("test-user"),
		"password":                    []byte("test-password"),
	}
	return &corev1.SecretList{Items: []corev1.Secret{secret}}
}

func FakeSecretsWithLabels(namespace string, labels map[string]string) *corev1.SecretList {
	secret := corev1.Secret{}
	secret.Name = GetRandomStr()
	secret.Namespace = namespace
	secret.Labels = labels
	secret.Data = map[string][]byte{
		"username": []byte("test-user"),
		"password": []byte("test-password"),
	}
	return &corev1.SecretList{Items: []corev1.Secret{secret}}
}

func FakeNode() *corev1.Node {
	node := &corev1.Node{}
	node.Name = NodeName
	node.Labels = map[string]string{
		constant.RegionLabelKey: "fake-node-region",
		constant.ZoneLabelKey:   "fake-node-zone",
	}
	return node
}

func FakeClusterDef() *appsv1alpha1.ClusterDefinition {
	clusterDef := &appsv1alpha1.ClusterDefinition{}
	clusterDef.Name = ClusterDefName
	clusterDef.Spec.ComponentDefs = []appsv1alpha1.ClusterComponentDefinition{
		{
			Name:          ComponentDefName,
			CharacterType: "mysql",
			SystemAccounts: &appsv1alpha1.SystemAccountSpec{
				CmdExecutorConfig: &appsv1alpha1.CmdExecutorConfig{
					CommandExecutorEnvItem: appsv1alpha1.CommandExecutorEnvItem{
						Image: "",
					},
					CommandExecutorItem: appsv1alpha1.CommandExecutorItem{
						Command: []string{"mysql"},
						Args:    []string{"-h$(KB_ACCOUNT_ENDPOINT)", "-e $(KB_ACCOUNT_STATEMENT)"},
					},
				},
				PasswordConfig: appsv1alpha1.PasswordConfig{},
				Accounts:       []appsv1alpha1.SystemAccountConfig{},
			},
		},
		{
			Name:          fmt.Sprintf("%s-%d", ComponentDefName, 1),
			CharacterType: "mysql",
		},
	}
	return clusterDef
}

func FakeComponentClassDef(clusterDef *appsv1alpha1.ClusterDefinition, def []byte) *corev1.ConfigMapList {
	result := &corev1.ConfigMapList{}
	cm := &corev1.ConfigMap{}
	cm.Name = fmt.Sprintf("fake-kubeblocks-classes-%s", ComponentName)
	cm.SetLabels(map[string]string{
		constant.KBAppComponentDefRefLabelKey: ComponentDefName,
		types.ClassProviderLabelKey:           "kubeblocks",
		constant.ClusterDefLabelKey:           clusterDef.Name,
	})
	cm.Data = map[string]string{"families-20230223162700": string(def)}
	result.Items = append(result.Items, *cm)
	return result
}

func FakeClusterVersion() *appsv1alpha1.ClusterVersion {
	cv := &appsv1alpha1.ClusterVersion{}
	cv.Name = ClusterVersionName
	cv.SetLabels(map[string]string{
		constant.ClusterDefLabelKey:   ClusterDefName,
		constant.AppManagedByLabelKey: constant.AppName,
	})
	cv.Spec.ClusterDefinitionRef = ClusterDefName
	cv.SetCreationTimestamp(metav1.Now())
	return cv
}

func FakeBackupTool() *dpv1alpha1.BackupTool {
	tool := &dpv1alpha1.BackupTool{}
	tool.Name = BackupToolName
	return tool
}

func FakeBackupPolicy(backupPolicyName, clusterName string) *dpv1alpha1.BackupPolicy {
	template := &dpv1alpha1.BackupPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.DPAPIGroup, types.DPAPIVersion),
			Kind:       types.KindBackupPolicy,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupPolicyName,
			Namespace: Namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey: clusterName,
			},
			Annotations: map[string]string{
				constant.DefaultBackupPolicyAnnotationKey: "true",
			},
		},
		Status: dpv1alpha1.BackupPolicyStatus{
			Phase: dpv1alpha1.PolicyAvailable,
		},
	}
	return template
}

func FakeBackup(backupName string) *dpv1alpha1.Backup {
	backup := &dpv1alpha1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.DPAPIGroup, types.DPAPIVersion),
			Kind:       types.KindBackup,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: Namespace,
		},
	}
	backup.SetCreationTimestamp(metav1.Now())
	return backup
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
					constant.AppInstanceLabelKey:    ClusterName,
					constant.KBAppComponentLabelKey: ComponentName,
					constant.AppManagedByLabelKey:   constant.AppName,
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
			annotations[types.ServiceHAVIPTypeAnnotationKey] = types.ServiceHAVIPTypeAnnotationValue
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
				constant.AppInstanceLabelKey:    ClusterName,
				constant.KBAppComponentLabelKey: ComponentName,
				constant.AppManagedByLabelKey:   constant.AppName,
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

func FakeVolumeSnapshotClass() *snapshotv1.VolumeSnapshotClass {
	return &snapshotv1.VolumeSnapshotClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VolumeSnapshotClass",
			APIVersion: "snapshot.storage.k8s.io/v1",
		},
	}
}

func FakeKBDeploy(version string) *appsv1.Deployment {
	deploy := &appsv1.Deployment{}
	deploy.SetLabels(map[string]string{
		"app.kubernetes.io/name": types.KubeBlocksChartName,
	})
	if len(version) > 0 {
		deploy.Labels["app.kubernetes.io/version"] = version
	}
	return deploy
}

func FakeAddon(name string) *extensionsv1alpha1.Addon {
	addon := &extensionsv1alpha1.Addon{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.ExtensionsAPIGroup, types.ExtensionsAPIVersion),
			Kind:       "Addon",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: Namespace,
		},
		Spec: extensionsv1alpha1.AddonSpec{
			Installable: &extensionsv1alpha1.InstallableSpec{
				Selectors: []extensionsv1alpha1.SelectorRequirement{
					{Key: extensionsv1alpha1.KubeGitVersion, Operator: extensionsv1alpha1.Contains, Values: []string{"k3s"}},
				},
			},
		},
	}
	addon.SetCreationTimestamp(metav1.Now())
	return addon
}

func FakeConfigMap(cmName string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: Namespace,
		},
		Data: map[string]string{
			"fake": "fake",
		},
	}
	return cm
}

func FakeConfigConstraint(ccName string) *appsv1alpha1.ConfigConstraint {
	cm := &appsv1alpha1.ConfigConstraint{
		ObjectMeta: metav1.ObjectMeta{
			Name: ccName,
		},
		Spec: appsv1alpha1.ConfigConstraintSpec{},
	}
	return cm
}
