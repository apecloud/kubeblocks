package reconcile

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/internal/constant"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
)

var (
	env = []corev1.EnvVar{
		{
			Name:  "HOST_IP",
			Value: "0.0.0.0",
		},
		{
			Name:  "HOST_ROOT",
			Value: "/host/root",
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}

	volumeMounts = []corev1.VolumeMount{
		{
			Name:      OTeldName,
			MountPath: "/var/log/oteld",
		},
		{
			Name:      "root",
			MountPath: "/host/root",
			ReadOnly:  true,
		},
		{
			Name:      "config-volume",
			MountPath: "/etc/oteld/config",
		},
	}

	hostPathType = corev1.HostPathDirectoryOrCreate
	volumes      = []corev1.Volume{
		{
			Name: OTeldName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/oteld",
					Type: &hostPathType,
				},
			},
		},
		{
			Name: "root",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/",
				},
			},
		},
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: OTeldName,
				},
			},
		},
	}
)

func buildPodSpecForOteld(config *types.Config) *corev1.PodSpec {
	container := corev1.Container{
		Name:            OTeldName,
		Image:           config.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"--config=/etc/oteld/config/config.yaml",
		},
		Env: env,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: 1234,
				//HostPort:      1234,
			},
		},
		Resources:    *config.Resources,
		VolumeMounts: volumeMounts,
	}

	return &builder.NewPodBuilder("", "").
		AddSerciveAccount("oteld-controller").
		AddContainer(container).
		SetVolumes(volumes...).
		GetObject().Spec
}

func buildSvcForOtel(namespace string) *corev1.Service {
	var (
		svcPort = corev1.ServicePort{
			Name:       "http",
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(1234),
			Port:       1234,
		}

		annos = map[string]string{
			constant.MonitorScrapeKey: "true",
			constant.MonitorPathKey:   "/metrics",
			constant.MonitorSchemaKey: "http",
		}

		svcTypes = corev1.ServiceTypeClusterIP
		labels   = map[string]string{
			constant.AppManagedByLabelKey: constant.AppName,
			constant.MonitorManagedByKey:  "agamotto",
		}
		selectors = map[string]string{
			constant.AppInstanceLabelKey: "apecloudoteld",
			constant.AppNameLabelKey:     "apecloudoteld",
		}
	)

	return builder.NewServiceBuilder(namespace, OTeldName).
		AddLabelsInMap(labels).
		AddSelectorsInMap(selectors).
		AddPorts(svcPort).
		SetType(svcTypes).
		SetAnnotations(annos).
		GetObject()
}
