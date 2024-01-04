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

package apps

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

const (
	KubeBlocks          = "kubeblocks"
	LogVolumeName       = "log"
	ConfVolumeName      = "conf"
	DataVolumeName      = "data"
	ScriptsVolumeName   = "scripts"
	ServiceDefaultName  = "default"
	ServiceHeadlessName = "headless"
	ServiceVPCName      = "vpc-lb"
	ServiceInternetName = "internet-lb"

	ReplicationPodRoleVolume         = "pod-role"
	ReplicationRoleLabelFieldPath    = "metadata.labels['kubeblocks.io/role']"
	DefaultReplicationCandidateIndex = 0
	DefaultReplicationReplicas       = 2

	ApeCloudMySQLImage        = "docker.io/apecloud/apecloud-mysql-server:latest"
	DefaultMySQLContainerName = "mysql"

	NginxImage                = "nginx"
	DefaultNginxContainerName = "nginx"

	DefaultRedisCompDefName       = "redis"
	DefaultRedisCompSpecName      = "redis-rsts"
	DefaultRedisImageName         = "redis:7.0.5"
	DefaultRedisContainerName     = "redis"
	DefaultRedisInitContainerName = "redis-init-container"

	Class1c1gName                 = "general-1c1g"
	Class2c4gName                 = "general-2c4g"
	DefaultResourceConstraintName = "kb-resource-constraint"

	StorageClassName = "test-sc"
	EnvKeyImageTag   = "IMAGE_TAG"
	DefaultImageTag  = "test"

	DefaultConfigSpecName          = "config-cm"
	DefaultConfigSpecTplRef        = "env-from-config-tpl"
	DefaultConfigSpecVolumeName    = "volume"
	DefaultConfigSpecConstraintRef = "env-from-config-test"
	DefaultScriptSpecName          = "script-cm"
	DefaultScriptSpecTplRef        = "env-from-config-tpl"
	DefaultScriptSpecVolumeName    = "script-volume"
)

var (
	defaultBuiltinHandler         = appsv1alpha1.MySQLBuiltinActionHandler
	defaultLifecycleActionHandler = &appsv1alpha1.LifecycleActionHandler{
		BuiltinHandler: &defaultBuiltinHandler,
	}

	zeroResRequirements = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("0"),
			corev1.ResourceMemory: resource.MustParse("0"),
		},
	}

	statelessNginxComponent = appsv1alpha1.ClusterComponentDefinition{
		WorkloadType:  appsv1alpha1.Stateless,
		CharacterType: "stateless",
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleProbe: &appsv1alpha1.ClusterDefinitionProbe{
				FailureThreshold: 3,
				PeriodSeconds:    1,
				TimeoutSeconds:   5,
			},
		},
		VolumeProtectionSpec: &appsv1alpha1.VolumeProtectionSpec{},
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:      DefaultNginxContainerName,
				Image:     NginxImage,
				Resources: zeroResRequirements,
			}},
		},
		Service: &appsv1alpha1.ServiceSpec{
			Ports: []appsv1alpha1.ServicePort{{
				Protocol: corev1.ProtocolTCP,
				Port:     80,
			}},
		},
	}

	// defaultSvc value are corresponding to defaultMySQLContainer.Ports name mapping and
	// corresponding to defaultConnectionCredential variable placeholder
	defaultSvcSpec = appsv1alpha1.ServiceSpec{
		Ports: []appsv1alpha1.ServicePort{
			{
				Name: "mysql",
				TargetPort: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "mysql",
				},
				Port: 3306,
			},
			{
				Name: "paxos",
				TargetPort: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "paxos",
				},
				Port: 13306,
			},
		},
	}

	defaultMySQLContainer = corev1.Container{
		Name:            DefaultMySQLContainerName,
		Image:           ApeCloudMySQLImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Resources:       zeroResRequirements,
		Ports: []corev1.ContainerPort{
			{
				Name:          "mysql",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: 3306,
			},
			{
				Name:          "paxos",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: 13306,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      DataVolumeName,
				MountPath: "/var/lib/mysql",
			},
			{
				Name:      LogVolumeName,
				MountPath: "/var/log",
			},
			{
				Name:      ScriptsVolumeName,
				MountPath: "/scripts",
			},
		},
		Env:     []corev1.EnvVar{{}},
		Command: []string{"/scripts/setup.sh"},
	}

	statefulMySQLComponent = appsv1alpha1.ClusterComponentDefinition{
		WorkloadType:  appsv1alpha1.Stateful,
		CharacterType: "mysql",
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleProbe: &appsv1alpha1.ClusterDefinitionProbe{
				FailureThreshold: 3,
				PeriodSeconds:    1,
				TimeoutSeconds:   5,
			},
		},
		VolumeProtectionSpec: &appsv1alpha1.VolumeProtectionSpec{},
		Service:              &defaultMySQLService,
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{defaultMySQLContainer},
		},
		VolumeTypes: []appsv1alpha1.VolumeTypeSpec{{
			Name: DataVolumeName,
			Type: appsv1alpha1.VolumeTypeData,
		}},
	}

	defaultConsensusSpec = appsv1alpha1.ConsensusSetSpec{
		Leader: appsv1alpha1.ConsensusMember{
			Name:       "leader",
			AccessMode: appsv1alpha1.ReadWrite,
		},
		Followers: []appsv1alpha1.ConsensusMember{{
			Name:       "follower",
			AccessMode: appsv1alpha1.Readonly,
		}},
		StatefulSetSpec: appsv1alpha1.StatefulSetSpec{
			UpdateStrategy: appsv1alpha1.BestEffortParallelStrategy,
		},
	}

	defaultMySQLService = appsv1alpha1.ServiceSpec{
		Ports: []appsv1alpha1.ServicePort{{
			Name:     "mysql",
			Protocol: corev1.ProtocolTCP,
			Port:     3306,
		}},
	}

	consensusMySQLComponent = appsv1alpha1.ClusterComponentDefinition{
		WorkloadType:  appsv1alpha1.Consensus,
		CharacterType: "mysql",
		ConsensusSpec: &defaultConsensusSpec,
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleProbe: &appsv1alpha1.ClusterDefinitionProbe{
				FailureThreshold: 3,
				PeriodSeconds:    1,
				TimeoutSeconds:   5,
			},
		},
		VolumeProtectionSpec: &appsv1alpha1.VolumeProtectionSpec{},
		Service:              &defaultMySQLService,
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{defaultMySQLContainer},
		},
		VolumeTypes: []appsv1alpha1.VolumeTypeSpec{{
			Name: DataVolumeName,
			Type: appsv1alpha1.VolumeTypeData,
		}},
	}

	defaultComponentDefSpec = appsv1alpha1.ComponentDefinitionSpec{
		Provider:       "kubeblocks.io",
		Description:    "ApeCloud MySQL is a database that is compatible with MySQL syntax and achieves high availability\n  through the utilization of the RAFT consensus protocol.",
		ServiceKind:    "mysql",
		ServiceVersion: "8.0.30",
		Runtime: corev1.PodSpec{
			Containers: []corev1.Container{
				defaultMySQLContainer,
			},
		},
		Volumes: []appsv1alpha1.ComponentVolume{
			{
				Name:         DataVolumeName,
				NeedSnapshot: true,
			},
			{
				Name:         LogVolumeName,
				NeedSnapshot: true,
			},
		},
		Services: []appsv1alpha1.ComponentService{
			{
				Service: appsv1alpha1.Service{
					Name:        "rw",
					ServiceName: "rw",
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Protocol: corev1.ProtocolTCP,
								Port:     3306,
								TargetPort: intstr.IntOrString{
									Type:   intstr.String,
									StrVal: "mysql",
								},
							},
						},
					},
					RoleSelector: "leader",
				},
			},
			{
				Service: appsv1alpha1.Service{
					Name:        "ro",
					ServiceName: "ro",
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Protocol: corev1.ProtocolTCP,
								Port:     3306,
								TargetPort: intstr.IntOrString{
									Type:   intstr.String,
									StrVal: "mysql",
								},
							},
						},
					},
					RoleSelector: "follower",
				},
			},
		},
		SystemAccounts: []appsv1alpha1.SystemAccount{
			{
				Name:        "root",
				InitAccount: true,
				PasswordGenerationPolicy: appsv1alpha1.PasswordConfig{
					Length:     16,
					NumDigits:  8,
					NumSymbols: 8,
					LetterCase: appsv1alpha1.MixedCases,
				},
			},
			{
				Name:      "admin",
				Statement: "CREATE USER $(USERNAME) IDENTIFIED BY '$(PASSWORD)'; GRANT ALL PRIVILEGES ON *.* TO $(USERNAME);",
				PasswordGenerationPolicy: appsv1alpha1.PasswordConfig{
					Length:     10,
					NumDigits:  5,
					NumSymbols: 0,
					LetterCase: appsv1alpha1.MixedCases,
				},
			},
		},
		Roles: []appsv1alpha1.ReplicaRole{
			{
				Name:        "leader",
				Serviceable: true,
				Writable:    true,
				Votable:     true,
			},
			{
				Name:        "follower",
				Serviceable: true,
				Writable:    false,
				Votable:     true,
			},
			{
				Name:        "learner",
				Serviceable: false,
				Writable:    false,
				Votable:     false,
			},
		},
		LifecycleActions: &appsv1alpha1.ComponentLifecycleActions{
			PostProvision: defaultLifecycleActionHandler,
			PreTerminate:  defaultLifecycleActionHandler,
			RoleProbe: &appsv1alpha1.RoleProbe{
				LifecycleActionHandler: *defaultLifecycleActionHandler,
				FailureThreshold:       3,
				PeriodSeconds:          1,
				TimeoutSeconds:         5,
			},
			Switchover:       nil,
			MemberJoin:       defaultLifecycleActionHandler,
			MemberLeave:      defaultLifecycleActionHandler,
			Readonly:         defaultLifecycleActionHandler,
			Readwrite:        defaultLifecycleActionHandler,
			DataPopulate:     defaultLifecycleActionHandler,
			DataAssemble:     defaultLifecycleActionHandler,
			Reconfigure:      defaultLifecycleActionHandler,
			AccountProvision: defaultLifecycleActionHandler,
		},
	}

	DefaultCompDefConfigs = []appsv1alpha1.ComponentConfigSpec{
		{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        DefaultConfigSpecName,
				TemplateRef: DefaultConfigSpecTplRef,
				VolumeName:  DefaultConfigSpecVolumeName,
			},
			ConfigConstraintRef: DefaultConfigSpecConstraintRef,
		},
	}

	DefaultCompDefScripts = []appsv1alpha1.ComponentTemplateSpec{
		{
			Name:        DefaultScriptSpecName,
			TemplateRef: DefaultScriptSpecTplRef,
			VolumeName:  DefaultScriptSpecVolumeName,
		},
	}

	defaultRedisService = appsv1alpha1.ServiceSpec{
		Ports: []appsv1alpha1.ServicePort{{
			Protocol: corev1.ProtocolTCP,
			Port:     6379,
		}},
	}

	defaultReplicationRedisVolumeMounts = []corev1.VolumeMount{
		{
			Name:      DataVolumeName,
			MountPath: "/data",
		},
		{
			Name:      ScriptsVolumeName,
			MountPath: "/scripts",
		},
		{
			Name:      ConfVolumeName,
			MountPath: "/etc/conf",
		},
		{
			Name:      ReplicationPodRoleVolume,
			MountPath: "/etc/conf/role",
		},
	}

	defaultRedisInitContainer = corev1.Container{
		Name:            DefaultRedisInitContainerName,
		Image:           DefaultRedisImageName,
		ImagePullPolicy: corev1.PullIfNotPresent,
		VolumeMounts:    defaultReplicationRedisVolumeMounts,
		Command:         []string{"/scripts/init.sh"},
		Resources:       zeroResRequirements,
	}

	defaultRedisContainer = corev1.Container{
		Name:            DefaultRedisContainerName,
		Image:           DefaultRedisImageName,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{
			{
				Name:          "redis",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: 6379,
			},
		},
		VolumeMounts: defaultReplicationRedisVolumeMounts,
		Args:         []string{"/etc/conf/redis.conf"},
		Lifecycle: &corev1.Lifecycle{
			PostStart: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/scripts/setup.sh"},
				},
			},
		},
		Resources: zeroResRequirements,
	}

	replicationRedisComponent = appsv1alpha1.ClusterComponentDefinition{
		WorkloadType:  appsv1alpha1.Replication,
		CharacterType: "redis",
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleProbe: &appsv1alpha1.ClusterDefinitionProbe{
				FailureThreshold: 3,
				PeriodSeconds:    1,
				TimeoutSeconds:   5,
			},
		},
		VolumeProtectionSpec: &appsv1alpha1.VolumeProtectionSpec{},
		Service:              &defaultRedisService,
		VolumeTypes: []appsv1alpha1.VolumeTypeSpec{{
			Name: DataVolumeName,
			Type: appsv1alpha1.VolumeTypeData,
		}},
		PodSpec: &corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: ConfVolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: ReplicationPodRoleVolume,
					VolumeSource: corev1.VolumeSource{
						DownwardAPI: &corev1.DownwardAPIVolumeSource{
							Items: []corev1.DownwardAPIVolumeFile{
								{
									Path: "labels",
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: ReplicationRoleLabelFieldPath,
									},
								},
							},
						},
					},
				},
			},
			InitContainers: []corev1.Container{defaultRedisInitContainer},
			Containers:     []corev1.Container{defaultRedisContainer},
		},
	}

	Class1c1g = appsv1alpha1.ComponentClass{
		Name:   Class1c1gName,
		CPU:    resource.MustParse("1"),
		Memory: resource.MustParse("1Gi"),
	}

	Class2c4g = appsv1alpha1.ComponentClass{
		Name:   Class2c4gName,
		CPU:    resource.MustParse("2"),
		Memory: resource.MustParse("4Gi"),
	}

	DefaultClasses = map[string]appsv1alpha1.ComponentClass{
		Class1c1gName: Class1c1g,
		Class2c4gName: Class2c4g,
	}
)
