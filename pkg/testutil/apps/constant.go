/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const (
	KubeBlocks          = "kubeblocks"
	LogVolumeName       = "log"
	ConfVolumeName      = "conf"
	DataVolumeName      = "data"
	ScriptsVolumeName   = "scripts"
	ServiceDefaultName  = "default"
	ServiceNodePortName = "nodeport"
	ServiceHeadlessName = "headless"
	ServiceVPCName      = "vpc-lb"
	ServiceInternetName = "internet-lb"

	ApeCloudMySQLImage        = "docker.io/apecloud/apecloud-mysql-server:latest"
	DefaultMySQLContainerName = "mysql"

	NginxImage = "nginx"

	DefaultConfigSpecName          = "config-cm"
	DefaultConfigSpecTplRef        = "env-from-config-tpl"
	DefaultConfigSpecVolumeName    = "volume"
	DefaultConfigSpecConstraintRef = "env-from-config-test"
	DefaultScriptSpecName          = "script-cm"
	DefaultScriptSpecTplRef        = "env-from-config-tpl"
	DefaultScriptSpecVolumeName    = "script-volume"
)

var (
	NewLifecycleAction = func(name string) *appsv1.Action {
		return &appsv1.Action{
			Exec: &appsv1.ExecAction{
				Command: []string{"/bin/sh", "-c", fmt.Sprintf("echo %s", name)},
			},
		}
	}

	zeroResRequirements = corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    resource.MustParse("0"),
			corev1.ResourceMemory: resource.MustParse("0"),
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

	defaultComponentDefSpec = appsv1.ComponentDefinitionSpec{
		Provider:       "kubeblocks.io",
		Description:    "ApeCloud MySQL is a database that is compatible with MySQL syntax and achieves high availability\n  through the utilization of the RAFT consensus protocol.",
		ServiceKind:    "mysql",
		ServiceVersion: "8.0.30",
		Runtime: corev1.PodSpec{
			Containers: []corev1.Container{
				defaultMySQLContainer,
			},
		},
		Volumes: []appsv1.ComponentVolume{
			{
				Name:         DataVolumeName,
				NeedSnapshot: true,
			},
			{
				Name:         LogVolumeName,
				NeedSnapshot: true,
			},
		},
		Services: []appsv1.ComponentService{
			{
				Service: appsv1.Service{
					Name: "default",
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
				Service: appsv1.Service{
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
				Service: appsv1.Service{
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
		SystemAccounts: []appsv1.SystemAccount{
			{
				Name:        "root",
				InitAccount: true,
				PasswordGenerationPolicy: appsv1.PasswordConfig{
					Length:     16,
					NumDigits:  8,
					NumSymbols: 8,
					LetterCase: appsv1.MixedCases,
				},
			},
			{
				Name:      "admin",
				Statement: "CREATE USER $(USERNAME) IDENTIFIED BY '$(PASSWORD)'; GRANT ALL PRIVILEGES ON *.* TO $(USERNAME);",
				PasswordGenerationPolicy: appsv1.PasswordConfig{
					Length:     10,
					NumDigits:  5,
					NumSymbols: 0,
					LetterCase: appsv1.MixedCases,
				},
			},
		},
		UpdateStrategy: &[]appsv1.UpdateStrategy{appsv1.BestEffortParallelStrategy}[0],
		Roles: []appsv1.ReplicaRole{
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
		Exporter: &appsv1.Exporter{
			ScrapePath:   "metrics",
			ScrapePort:   "http-metric",
			ScrapeScheme: appsv1.HTTPProtocol,
		},
		LifecycleActions: &appsv1.ComponentLifecycleActions{
			PostProvision: nil,
			PreTerminate:  nil,
			RoleProbe: &appsv1.Probe{
				Action:        *NewLifecycleAction("role-probe"),
				PeriodSeconds: 1,
			},
			Switchover:       nil,
			MemberJoin:       nil,
			MemberLeave:      NewLifecycleAction("member-leave"),
			Readonly:         nil,
			Readwrite:        nil,
			DataDump:         nil,
			DataLoad:         nil,
			Reconfigure:      nil,
			AccountProvision: NewLifecycleAction("account-provision"),
		},
	}

	DefaultCompDefConfigs = []appsv1.ComponentTemplateSpec{
		{
			Name:        DefaultConfigSpecName,
			TemplateRef: DefaultConfigSpecTplRef,
			VolumeName:  DefaultConfigSpecVolumeName,
		},
	}

	DefaultCompDefScripts = []appsv1.ComponentTemplateSpec{
		{
			Name:        DefaultScriptSpecName,
			TemplateRef: DefaultScriptSpecTplRef,
			VolumeName:  DefaultScriptSpecVolumeName,
		},
	}

	defaultComponentVerSpec = func(compDef string) appsv1.ComponentVersionSpec {
		return appsv1.ComponentVersionSpec{
			CompatibilityRules: []appsv1.ComponentVersionCompatibilityRule{
				{
					CompDefs: []string{compDef},
					Releases: []string{"8.0.30-r1"},
				},
			},
			Releases: []appsv1.ComponentVersionRelease{
				{
					Name:           "8.0.30-r1",
					Changes:        "init release",
					ServiceVersion: "8.0.30",
					Images: map[string]string{
						defaultMySQLContainer.Name: defaultMySQLContainer.Image,
					},
				},
			},
		}
	}
)
