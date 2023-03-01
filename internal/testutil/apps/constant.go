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

package apps

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	KubeBlocks        = "kubeblocks"
	LogVolumeName     = "log"
	ConfVolumeName    = "conf"
	DataVolumeName    = "data"
	ScriptsVolumeName = "scripts"

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
	DefaultRedisCompType          = "redis"
	DefaultRedisCompName          = "redis-rsts"
	DefaultRedisImageName         = "redis:7.0.5"
	DefaultRedisContainerName     = "redis"
	DefaultRedisInitContainerName = "redis-init-container"
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
		Service: &corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Protocol: corev1.ProtocolTCP,
				Port:     80,
			}},
		},
	}

	defaultConnectionCredential = map[string]string{
		"username":      "root",
		"svcFQDN":       "$(SVC_FQDN)",
		"password":      "$(RANDOM_PASSWD)",
		"tcpEndpoint":   "tcp:$(SVC_FQDN):$(SVC_PORT_mysql)",
		"paxosEndpoint": "paxos:$(SVC_FQDN):$(SVC_PORT_paxos)",
	}

	// defaultSvc value are corresponding to defaultMySQLContainer.Ports name mapping and
	// corresponding to defaultConnectionCredential variable placeholder
	defaultSvcSpec = corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
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
		PodSpec: &corev1.PodSpec{
			Containers: []corev1.Container{defaultMySQLContainer},
		},
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

	defaultMySQLService = corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{
			Protocol: corev1.ProtocolTCP,
			Port:     3306,
		}},
	}

	consensusMySQLComponent = appsv1alpha1.ClusterComponentDefinition{
		WorkloadType:  appsv1alpha1.Consensus,
		CharacterType: "mysql",
		ConsensusSpec: &defaultConsensusSpec,
		Probes: &appsv1alpha1.ClusterDefinitionProbes{
			RoleChangedProbe: &appsv1alpha1.ClusterDefinitionProbe{
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

	defaultRedisService = corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{
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
)
