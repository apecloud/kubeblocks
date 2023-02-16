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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

const (
	ApeCloudMySQLImage        = "docker.io/apecloud/apecloud-mysql-server:latest"
	NginxImage                = "nginx"
	DefaultNginxContainerName = "nginx"
	DefaultMySQLContainerName = "mysql"
	DataVolumeName            = "data"
	LogVolumeName             = "log"
	ScriptsVolumeName         = "scripts"
)

var (
	statelessNginxComponent = appsv1alpha1.ClusterDefinitionComponent{
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
		"username": "root",
		"password": "$(RANDOM_PASSWD)",
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
						Name: "$(CONN_CREDENTIAL_SECRET_NAME)",
					},
					Key: "password",
				},
			},
		}},
		Command: []string{"/scripts/setup.sh"},
	}

	statefulMySQLComponent = appsv1alpha1.ClusterDefinitionComponent{
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

	consensusMySQLComponent = appsv1alpha1.ClusterDefinitionComponent{
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
)
