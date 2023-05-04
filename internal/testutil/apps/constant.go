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
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	KubeBlocks          = "kubeblocks"
	LogVolumeName       = "log"
	ConfVolumeName      = "conf"
	DataVolumeName      = "data"
	ScriptsVolumeName   = "scripts"
	ServiceDefaultName  = ""
	ServiceHeadlessName = "headless"
	ServiceVPCName      = "vpc-lb"
	ServiceInternetName = "internet-lb"

	ReplicationPodRoleVolume       = "pod-role"
	ReplicationRoleLabelFieldPath  = "metadata.labels['kubeblocks.io/role']"
	DefaultReplicationPrimaryIndex = 0
	DefaultReplicationReplicas     = 2

	MySQLType                 = "state.mysql"
	ApeCloudMySQLImage        = "docker.io/apecloud/apecloud-mysql-server:latest"
	DefaultMySQLContainerName = "mysql"

	NginxImage                = "nginx"
	DefaultNginxContainerName = "nginx"

	RedisType                     = "state.redis"
	DefaultRedisCompDefName       = "redis"
	DefaultRedisCompName          = "redis-rsts"
	DefaultRedisImageName         = "redis:7.0.5"
	DefaultRedisContainerName     = "redis"
	DefaultRedisInitContainerName = "redis-init-container"

	Class1c1gName                 = "general-1c1g"
	Class2c4gName                 = "general-2c4g"
	DefaultResourceConstraintName = "kb-resource-constraint"
)

var (
	statelessNginxComponent = appsv1alpha1.ClusterComponentDefinition{
		WorkloadType:  appsv1alpha1.Stateless,
		CharacterType: "stateless",
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: DefaultNginxContainerName,
			}},
		},
		Service: &appsv1alpha1.ServiceSpec{
			Ports: []appsv1alpha1.ServicePort{{
				Protocol: corev1.ProtocolTCP,
				Port:     80,
			}},
		},
	}

	defaultConnectionCredential = map[string]string{
		"username":          "root",
		"SVC_FQDN":          "$(SVC_FQDN)",
		"HEADLESS_SVC_FQDN": "$(HEADLESS_SVC_FQDN)",
		"RANDOM_PASSWD":     "$(RANDOM_PASSWD)",
		"tcpEndpoint":       "tcp:$(SVC_FQDN):$(SVC_PORT_mysql)",
		"paxosEndpoint":     "paxos:$(SVC_FQDN):$(SVC_PORT_paxos)",
		"UUID":              "$(UUID)",
		"UUID_B64":          "$(UUID_B64)",
		"UUID_STR_B64":      "$(UUID_STR_B64)",
		"UUID_HEX":          "$(UUID_HEX)",
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
		ImagePullPolicy: corev1.PullIfNotPresent,
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
		Env: []corev1.EnvVar{{
			Name: "MYSQL_ROOT_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: constant.ConnCredentialPlaceHolder,
					},
					Key: "password",
				},
			},
		}},
		Command: []string{"/scripts/setup.sh"},
	}

	statefulMySQLComponent = appsv1alpha1.ClusterComponentDefinition{
		WorkloadType:  appsv1alpha1.Stateful,
		CharacterType: "mysql",
		Service:       &defaultMySQLService,
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
		UpdateStrategy: appsv1alpha1.BestEffortParallelStrategy,
	}

	defaultMySQLService = appsv1alpha1.ServiceSpec{
		Ports: []appsv1alpha1.ServicePort{{
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
		Service: &defaultMySQLService,
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{defaultMySQLContainer},
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
		ImagePullPolicy: corev1.PullIfNotPresent,
		VolumeMounts:    defaultReplicationRedisVolumeMounts,
		Command:         []string{"/scripts/init.sh"},
	}

	defaultRedisContainer = corev1.Container{
		Name:            DefaultRedisContainerName,
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
	}

	replicationRedisComponent = appsv1alpha1.ClusterComponentDefinition{
		WorkloadType:  appsv1alpha1.Replication,
		CharacterType: "redis",
		Service:       &defaultRedisService,
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
		Volumes: []appsv1alpha1.Volume{
			{
				Name: "data",
				Size: resource.MustParse("20Gi"),
			},
			{
				Name: "log",
				Size: resource.MustParse("10Gi"),
			},
		},
	}

	Class2c4g = appsv1alpha1.ComponentClass{
		Name:   Class2c4gName,
		CPU:    resource.MustParse("2"),
		Memory: resource.MustParse("4Gi"),
		Volumes: []appsv1alpha1.Volume{
			{
				Name: "data",
				Size: resource.MustParse("20Gi"),
			},
			{
				Name: "log",
				Size: resource.MustParse("10Gi"),
			},
		},
	}

	DefaultClasses = map[string]appsv1alpha1.ComponentClass{
		Class1c1gName: Class1c1g,
		Class2c4gName: Class2c4g,
	}
)
