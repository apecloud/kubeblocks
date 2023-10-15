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

package testing

import (
	"fmt"
	"time"

	chaosmeshv1alpha1 "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/util/storage"
	"k8s.io/utils/pointer"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	storagev1alpha1 "github.com/apecloud/kubeblocks/apis/storage/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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
	ServiceRefName     = "fake-serviceRef"

	KubeBlocksRepoName  = "fake-kubeblocks-repo"
	KubeBlocksChartName = "fake-kubeblocks"
	KubeBlocksChartURL  = "fake-kubeblocks-chart-url"
	BackupMethodName    = "fake-backup-method"
	ActionSetName       = "fake-action-set"
	BackupName          = "fake-backup-name"

	IsDefault    = true
	IsNotDefault = false
)

var (
	ExtraComponentDefName = fmt.Sprintf("%s-%d", ComponentDefName, 1)
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
			UID:       "b262b889-a27f-42d8-b066-2978561c8167",
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

// FakeSecret for test cluster create
func FakeSecret(namespace string, cluster string) *corev1.Secret {
	secret := corev1.Secret{}
	secret.Name = SecretName
	secret.Namespace = namespace
	secret.Type = corev1.SecretTypeServiceAccountToken
	secret.Labels = map[string]string{
		constant.AppInstanceLabelKey: cluster,
		"name":                       types.KubeBlocksChartName,
		"owner":                      "helm",
	}

	secret.Data = map[string][]byte{
		corev1.ServiceAccountTokenKey: []byte("fake-secret-token"),
		"fake-secret-key":             []byte("fake-secret-value"),
		"username":                    []byte("test-user"),
		"password":                    []byte("test-password"),
	}
	return &secret
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
			ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{
				{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:        "mysql-consensusset-config",
						TemplateRef: "mysql8.0-config-template",
						Namespace:   Namespace,
						VolumeName:  "mysql-config",
					},
					ConfigConstraintRef: "mysql8.0-config-constraints",
				},
			},
			ServiceRefDeclarations: []appsv1alpha1.ServiceRefDeclaration{
				FakeServiceRef(ServiceRefName),
			},
		},
		{
			Name:          ExtraComponentDefName,
			CharacterType: "mysql",
			ConfigSpecs: []appsv1alpha1.ComponentConfigSpec{
				{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:        "mysql-consensusset-config",
						TemplateRef: "mysql8.0-config-template",
						Namespace:   Namespace,
						VolumeName:  "mysql-config",
					},
					ConfigConstraintRef: "mysql8.0-config-constraints",
				},
			},
			SwitchoverSpec: &appsv1alpha1.SwitchoverSpec{
				WithCandidate: &appsv1alpha1.SwitchoverAction{
					CmdExecutorConfig: &appsv1alpha1.CmdExecutorConfig{
						CommandExecutorEnvItem: appsv1alpha1.CommandExecutorEnvItem{
							Image: "",
						},
						CommandExecutorItem: appsv1alpha1.CommandExecutorItem{
							Command: []string{"mysql"},
							Args:    []string{"-h$(KB_CONSENSUS_LEADER_POD_FQDN)", "-e $(KB_SWITCHOVER_ACTION)"},
						},
					},
				},
			},
		},
	}
	return clusterDef
}

func FakeComponentClassDef(name string, clusterDefRef string, componentDefRef string) *appsv1alpha1.ComponentClassDefinition {
	testapps.NewComponentResourceConstraintFactory(testapps.DefaultResourceConstraintName).
		AddConstraints(testapps.GeneralResourceConstraint).
		GetObject()

	componentClassDefinition := testapps.NewComponentClassDefinitionFactory(name, clusterDefRef, componentDefRef).
		AddClasses([]appsv1alpha1.ComponentClass{testapps.Class1c1g, testapps.Class2c4g}).
		GetObject()

	return componentClassDefinition
}

func FakeClusterVersion() *appsv1alpha1.ClusterVersion {
	cv := &appsv1alpha1.ClusterVersion{}
	gvr := types.ClusterVersionGVR()
	cv.TypeMeta.APIVersion = gvr.GroupVersion().String()
	cv.TypeMeta.Kind = types.KindClusterVersion
	cv.Name = ClusterVersionName
	cv.SetLabels(map[string]string{
		constant.ClusterDefLabelKey:   ClusterDefName,
		constant.AppManagedByLabelKey: constant.AppName,
	})
	cv.Spec.ClusterDefinitionRef = ClusterDefName
	cv.SetCreationTimestamp(metav1.Now())
	return cv
}

func FakeActionSet() *dpv1alpha1.ActionSet {
	as := &dpv1alpha1.ActionSet{}
	as.Name = ActionSetName
	return as
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
				dptypes.DefaultBackupPolicyAnnotationKey: "true",
			},
		},
		Spec: dpv1alpha1.BackupPolicySpec{
			BackupMethods: []dpv1alpha1.BackupMethod{
				{
					Name:            BackupMethodName,
					SnapshotVolumes: boolptr.False(),
					ActionSetName:   ActionSetName,
				},
			},
			Target: &dpv1alpha1.BackupTarget{
				PodSelector: &dpv1alpha1.PodSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							constant.AppInstanceLabelKey:    ClusterName,
							constant.KBAppComponentLabelKey: ComponentName,
							constant.AppManagedByLabelKey:   constant.AppName},
					},
				},
			},
		},
		Status: dpv1alpha1.BackupPolicyStatus{
			Phase: dpv1alpha1.AvailablePhase,
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

func FakeBackupSchedule(backupScheduleName, backupPolicyName string) *dpv1alpha1.BackupSchedule {
	backupSchedule := &dpv1alpha1.BackupSchedule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.DPAPIGroup, types.DPAPIVersion),
			Kind:       types.KindBackupSchedule,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupScheduleName,
			Namespace: Namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey: ClusterName,
			},
		},
		Spec: dpv1alpha1.BackupScheduleSpec{
			BackupPolicyName: backupPolicyName,
			Schedules: []dpv1alpha1.SchedulePolicy{
				{
					Enabled:         boolptr.True(),
					CronExpression:  "0 0 * * *",
					BackupMethod:    BackupMethodName,
					RetentionPeriod: dpv1alpha1.RetentionPeriod("1d"),
				},
			},
		},
		Status: dpv1alpha1.BackupScheduleStatus{
			Phase: dpv1alpha1.BackupSchedulePhaseAvailable,
		},
	}
	return backupSchedule
}

func FakeBackupPolicyTemplate(backupPolicyTemplateName string, clusterDef string) *appsv1alpha1.BackupPolicyTemplate {
	backupPolicyTemplate := &appsv1alpha1.BackupPolicyTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.AppsAPIGroup, types.AppsAPIVersion),
			Kind:       types.KindBackupPolicyTemplate,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupPolicyTemplateName,
			Namespace: Namespace,
			Labels: map[string]string{
				constant.ClusterDefLabelKey: ClusterDefName,
			},
		},
		Spec: appsv1alpha1.BackupPolicyTemplateSpec{
			ClusterDefRef: clusterDef,
			Identifier:    "fake-identifier",
		},
	}
	return backupPolicyTemplate
}

func FakeBackupWithCluster(cluster *appsv1alpha1.Cluster, backupName string) *dpv1alpha1.Backup {
	backup := &dpv1alpha1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.DPAPIGroup, types.DPAPIVersion),
			Kind:       types.KindBackup,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: Namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:             cluster.Name,
				dptypes.DataProtectionLabelClusterUIDKey: string(cluster.UID),
			},
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
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
	}
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

func FakeConfigMap(cmName string, namespace string, data map[string]string) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: Namespace,
		},
		Data: data,
	}
	if namespace != "" {
		cm.Namespace = namespace
	}
	return cm
}

func FakeConfigConstraint(ccName string) *appsv1alpha1.ConfigConstraint {
	cm := &appsv1alpha1.ConfigConstraint{
		ObjectMeta: metav1.ObjectMeta{
			Name: ccName,
		},
		Spec: appsv1alpha1.ConfigConstraintSpec{
			FormatterConfig: &appsv1alpha1.FormatterConfig{},
		},
	}
	return cm
}

func FakeStorageClass(name string, isDefault bool) *storagev1.StorageClass {
	storageClassObj := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "storage.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if isDefault {
		storageClassObj.ObjectMeta.Annotations = make(map[string]string)
		storageClassObj.ObjectMeta.Annotations[storage.IsDefaultStorageClassAnnotation] = "true"
	}
	return storageClassObj
}

func FakeServiceAccount(name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: Namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
				constant.AppNameLabelKey:     KubeBlocksChartName},
		},
	}
}

func FakeClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
				constant.AppNameLabelKey:     KubeBlocksChartName},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}
}

func FakeClusterRoleBinding(name string, sa *corev1.ServiceAccount, clusterRole *rbacv1.ClusterRole) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
				constant.AppNameLabelKey:     KubeBlocksChartName},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: clusterRole.Kind,
			Name: clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
	}
}

func FakeRole(name string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
				constant.AppNameLabelKey:     KubeBlocksChartName},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}
}

func FakeRoleBinding(name string, sa *corev1.ServiceAccount, role *rbacv1.Role) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: Namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
				constant.AppNameLabelKey:     KubeBlocksChartName},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: role.Kind,
			Name: role.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
	}
}

func FakeDeploy(name string, namespace string, extraLabels map[string]string) *appsv1.Deployment {
	labels := map[string]string{
		constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
	}
	// extraLabels will override the labels above if there is a conflict
	for k, v := range extraLabels {
		labels[k] = v
	}
	labels["app"] = name

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
	}
}

func FakeStatefulSet(name string, namespace string, extraLabels map[string]string) *appsv1.StatefulSet {
	labels := map[string]string{
		constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
	}
	// extraLabels will override the labels above if there is a conflict
	for k, v := range extraLabels {
		labels[k] = v
	}
	labels["app"] = name
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas: 1,
		},
	}
}

func FakePodForSts(sts *appsv1.StatefulSet) *corev1.PodList {
	pods := &corev1.PodList{}
	for i := 0; i < int(*sts.Spec.Replicas); i++ {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", sts.Name, i),
				Namespace: sts.Namespace,
				Labels:    sts.Spec.Template.Labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  sts.Name,
						Image: "fake-image",
					},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}
		pods.Items = append(pods.Items, *pod)
	}
	return pods
}

func FakeJob(name string, namespace string, extraLabels map[string]string) *batchv1.Job {
	labels := map[string]string{
		constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
	}
	// extraLabels will override the labels above if there is a conflict
	for k, v := range extraLabels {
		labels[k] = v
	}
	labels["app"] = name

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Completions: pointer.Int32(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
		Status: batchv1.JobStatus{
			Active: 1,
			Ready:  pointer.Int32(1),
		},
	}
}

func FakeCronJob(name string, namespace string, extraLabels map[string]string) *batchv1.CronJob {
	labels := map[string]string{
		constant.AppInstanceLabelKey: types.KubeBlocksReleaseName,
	}
	// extraLabels will override the labels above if there is a conflict
	for k, v := range extraLabels {
		labels[k] = v
	}
	labels["app"] = name

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/1 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
			},
		},
	}
}

func FakeResourceNotFound(versionResource schema.GroupVersionResource, name string) *metav1.Status {
	return &metav1.Status{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Status",
			APIVersion: "v1",
		},
		Status:  "Failure",
		Message: fmt.Sprintf("%s.%s \"%s\" not found", versionResource.Resource, versionResource.Group, name),
		Reason:  "NotFound",
		Details: nil,
		Code:    404,
	}
}

func FakePodChaos(name, namespace string) *chaosmeshv1alpha1.PodChaos {
	return &chaosmeshv1alpha1.PodChaos{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodChaos",
			APIVersion: "chaos-mesh.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: chaosmeshv1alpha1.PodChaosSpec{
			ContainerSelector: chaosmeshv1alpha1.ContainerSelector{
				PodSelector: chaosmeshv1alpha1.PodSelector{
					Selector: chaosmeshv1alpha1.PodSelectorSpec{
						GenericSelectorSpec: chaosmeshv1alpha1.GenericSelectorSpec{
							Namespaces: []string{namespace},
						},
					},
				},
			},
			Action: chaosmeshv1alpha1.PodKillAction,
		},
	}
}

func FakeEventForObject(name string, namespace string, object string) *corev1.Event {
	return &corev1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Name: object,
		},
	}
}

func FakeStorageProvider(name string) *storagev1alpha1.StorageProvider {
	storageProvider := &storagev1alpha1.StorageProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.StorageAPIGroup, types.StorageAPIVersion),
			Kind:       "StorageProvider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: storagev1alpha1.StorageProviderSpec{
			CSIDriverName: "fake-csi-s3",
			CSIDriverSecretTemplate: `
accessKeyId: {{ index .Parameters "accessKeyId" }}
secretAccessKey: {{ index .Parameters "secretAccessKey" }}
`,
			StorageClassTemplate: `
bucket: {{ index .Parameters "bucket" }}
region: {{ index .Parameters "region" }}
endpoint: {{ index .Parameters "endpoint" }}
mountOptions: {{ index .Parameters "mountOptions" | default "" }}
`,
			ParametersSchema: &storagev1alpha1.ParametersSchema{
				OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "object",
					Properties: map[string]apiextensionsv1.JSONSchemaProps{
						"accessKeyId":     {Type: "string"},
						"secretAccessKey": {Type: "string"},
						"bucket":          {Type: "string"},
						"region": {
							Type: "string",
							Enum: []apiextensionsv1.JSON{{Raw: []byte(`""`)}, {Raw: []byte(`"us-east-1"`)}, {Raw: []byte(`"us-west-1"`)}},
						},
						"endpoint":     {Type: "string"},
						"mountOptions": {Type: "string"},
					},
					Required: []string{
						"accessKeyId",
						"secretAccessKey",
					},
				},
				CredentialFields: []string{
					"accessKeyId",
					"secretAccessKey",
				},
			},
		},
	}
	storageProvider.SetCreationTimestamp(metav1.Now())
	return storageProvider
}

func FakeBackupRepo(name string, isDefault bool) *dpv1alpha1.BackupRepo {
	backupRepo := &dpv1alpha1.BackupRepo{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", types.DPAPIGroup, types.DPAPIVersion),
			Kind:       "BackupRepo",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: dpv1alpha1.BackupRepoSpec{
			StorageProviderRef: "fake-storage-provider",
			PVReclaimPolicy:    "Retain",
		},
	}
	if isDefault {
		backupRepo.Annotations = map[string]string{
			dptypes.DefaultBackupRepoAnnotationKey: "true",
		}
	}
	return backupRepo
}

func FakeClusterList() *appsv1alpha1.ClusterList {
	clusters := &appsv1alpha1.ClusterList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "15",
		},
		Items: []appsv1alpha1.Cluster{
			*FakeCluster(ClusterName, Namespace),
			*FakeCluster(ClusterName+"-other", Namespace),
		},
	}
	clusters.Items = append(clusters.Items, *FakeCluster(ClusterName, Namespace))
	return clusters
}

func FakeServiceRef(serviceRefName string) appsv1alpha1.ServiceRefDeclaration {
	return appsv1alpha1.ServiceRefDeclaration{
		Name: serviceRefName,
		ServiceRefDeclarationSpecs: []appsv1alpha1.ServiceRefDeclarationSpec{
			{
				ServiceKind:    "mysql",
				ServiceVersion: "8.0.\\d{1,2}$",
			},
		},
	}
}
