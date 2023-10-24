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

package reconcile

import (
	"fmt"
	"path/filepath"

	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

const (
	OteldConfigMapName = "oteld-configmap"

	OteldConfigMapNamePattern       = "oteld-configmap-%s"
	OteldEngineConfigMapNamePattern = "oteld-configmap-engine-%s"
	OteldServiceNamePattern         = "oteld-service-%s"
	OteldSecretNamePattern          = "oteld-secret-%s"
	OteldEngineSecretNamePattern    = "oteld-secret-engine-%s"
)

var (
	defaultMetricsPort      = 8888
	defaultOtelConfigPath   = "/etc/oteld/config/config.yaml"
	defaultEngineConfigPath = "/opt/apecloud/apps/kb_engine.yaml"
)

func buildPodSpecForOteld(oTeld *v1alpha1.OTeld, mode v1alpha1.Mode) corev1.PodSpec {
	oteldSpec := oTeld.Spec
	container := builder.NewContainerBuilder(OTeldName).
		SetImage(oteldSpec.Image).
		SetImagePullPolicy(corev1.PullIfNotPresent).
		AddArgs(fmt.Sprintf("--config=%s", defaultOtelConfigPath)).
		AddEnv(corev1.EnvVar{
			Name:  "HOST_IP",
			Value: "0.0.0.0",
			//ValueFrom: &corev1.EnvVarSource{
			//	FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"},
			//},
		}).
		AddEnv(corev1.EnvVar{
			Name:  "HOST_ROOT_MOUNT_PATH",
			Value: "/host/root",
		}).
		AddEnv(corev1.EnvVar{
			Name:  "OTELD_ENGINE_CONFIG",
			Value: defaultEngineConfigPath,
		}).
		AddEnv(corev1.EnvVar{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		}).
		AddEnv(corev1.EnvVar{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			},
		}).
		AddEnv(corev1.EnvVar{
			Name: "NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
			},
		}).
		SetResources(oteldSpec.Resources).
		SetSecurityContext(corev1.SecurityContext{
			Privileged:             cfgutil.ToPointer(true),
			ReadOnlyRootFilesystem: cfgutil.ToPointer(true),
		}).
		SetReadinessProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "//metrics",
					Port: intstr.FromInt32(int32(defaultMetricsPort)),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		}).
		SetLivenessProbe(corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "//metrics",
					Port: intstr.FromInt32(int32(defaultMetricsPort)),
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       20,
		}).
		AddPorts(corev1.ContainerPort{
			Name:          "http",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: int32(defaultMetricsPort),
		}).
		AddVolumeMounts(corev1.VolumeMount{
			Name:      "oteldlog",
			MountPath: "/var/log/oteld",
		}).
		AddVolumeMounts(corev1.VolumeMount{
			Name:             "root",
			MountPath:        "/host/root",
			MountPropagation: cfgutil.ToPointer(corev1.MountPropagationHostToContainer),
			ReadOnly:         true,
		}).
		AddVolumeMounts(corev1.VolumeMount{
			Name:      "oteld-config-volume",
			MountPath: filepath.Dir(defaultOtelConfigPath),
		}).
		AddVolumeMounts(corev1.VolumeMount{
			Name:      "engine-config-volume",
			MountPath: filepath.Dir(defaultEngineConfigPath),
		}).
		GetObject()

	return builder.NewPodBuilder("", "").
		AddSerciveAccount("oteld-controller").
		AddContainer(*container).
		AddVolumes(corev1.Volume{
			Name: "oteldlog",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/log/oteld",
					Type: cfgutil.ToPointer(corev1.HostPathDirectoryOrCreate),
				}},
		}).
		AddVolumes(corev1.Volume{
			Name: "root",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/"}},
		}).
		AddVolumes(corev1.Volume{
			Name: "oteld-config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: fmt.Sprintf(OteldSecretNamePattern, mode),
				}},
		}).
		AddVolumes(corev1.Volume{
			Name: "engine-config-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: fmt.Sprintf(OteldEngineSecretNamePattern, mode),
				}},
		}).
		SetSecurityContext(corev1.PodSecurityContext{
			RunAsUser:    cfgutil.ToPointer(int64(0)),
			RunAsGroup:   cfgutil.ToPointer(int64(0)),
			FSGroup:      cfgutil.ToPointer(int64(65534)),
			RunAsNonRoot: cfgutil.ToPointer(false),
		}).
		GetObject().Spec
}

func buildSvcForOtel(oteld *v1alpha1.OTeld, namespace string, mode v1alpha1.Mode) (*corev1.Service, error) {
	if oteld == nil {
		return nil, nil
	}

	name := fmt.Sprintf(OteldServiceNamePattern, mode)
	port := oteld.Spec.MetricsPort
	if oteld.Spec.MetricsPort != 0 {
		port = oteld.Spec.MetricsPort
	}

	var (
		svcPort = corev1.ServicePort{
			Name:       "http",
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(1234),
			Port:       1234,
		}

		metricsPort = corev1.ServicePort{
			Name:       "metrics",
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(port),
			Port:       int32(port),
		}

		annos = map[string]string{
			constant.MonitorScrapeKey: "true",
			constant.MonitorPathKey:   "/metrics",
			constant.MonitorSchemaKey: "http",
		}

		svcTypes = corev1.ServiceTypeClusterIP
		labels   = map[string]string{
			constant.AppManagedByLabelKey: constant.AppName,
			constant.AppNameLabelKey:      OTeldName,
		}
		selectors = map[string]string{
			constant.AppInstanceLabelKey: "apecloudoteld",
			constant.AppNameLabelKey:     "apecloudoteld",
		}
	)

	return builder.NewServiceBuilder(namespace, name).
		AddLabelsInMap(labels).
		AddSelectorsInMap(selectors).
		AddPorts(svcPort, metricsPort).
		SetType(svcTypes).
		SetAnnotations(annos).
		GetObject(), nil
}

func buildConfigMapForOteld(instance *types.OteldInstance, namespace string, exporters *types.Exporters, mode v1alpha1.Mode, gc *types.OteldConfigGenerater) (*corev1.ConfigMap, error) {
	if instance == nil || instance.Oteld == nil || !instance.Oteld.Spec.UseConfigMap {
		return nil, nil
	}

	name := fmt.Sprintf(OteldConfigMapNamePattern, mode)

	commonLabels := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppNameLabelKey:      OTeldName,
		constant.AppInstanceLabelKey:  name,
	}

	configData, _ := gc.GenerateOteldConfiguration(instance, exporters.MetricsExporter, exporters.LogsExporter)
	marshal, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	return builder.NewConfigMapBuilder(namespace, name).
		SetData(map[string]string{"config.yaml": string(marshal)}).
		AddLabelsInMap(commonLabels).
		SetOwnerReferences(instance.Oteld.APIVersion, instance.Oteld.Kind, instance.Oteld).
		GetObject(), nil
}

func buildEngineConfigForOteld(instance *types.OteldInstance, namespace string, mode v1alpha1.Mode, gc *types.OteldConfigGenerater) (*corev1.ConfigMap, error) {
	if instance == nil || instance.Oteld == nil || !instance.Oteld.Spec.UseConfigMap {
		return nil, nil
	}

	name := fmt.Sprintf(OteldEngineConfigMapNamePattern, mode)

	commonLabels := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppNameLabelKey:      OTeldName,
		constant.AppInstanceLabelKey:  name,
	}

	configData, _ := gc.GenerateEngineConfiguration(instance)
	marshal, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	return builder.NewConfigMapBuilder(namespace, name).
		SetData(map[string]string{"kb_engine.yaml": string(marshal)}).
		AddLabelsInMap(commonLabels).
		SetOwnerReferences(instance.Oteld.APIVersion, instance.Oteld.Kind, instance.Oteld).
		GetObject(), nil
}

func buildSecretForOteld(instance *types.OteldInstance, namespace string, exporters *types.Exporters, mode v1alpha1.Mode, gc *types.OteldConfigGenerater) (*corev1.Secret, error) {
	if instance == nil || instance.Oteld == nil {
		return nil, nil
	}

	name := fmt.Sprintf(OteldSecretNamePattern, mode)

	commonLabels := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppNameLabelKey:      OTeldName,
		constant.AppInstanceLabelKey:  name,
	}

	configData, _ := gc.GenerateOteldConfiguration(instance, exporters.MetricsExporter, exporters.LogsExporter)
	marshal, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	return builder.NewSecretBuilder(namespace, name).
		PutData("config.yaml", marshal).
		AddLabelsInMap(commonLabels).
		SetOwnerReferences(instance.Oteld.APIVersion, instance.Oteld.Kind, instance.Oteld).
		GetObject(), nil
}

func buildEngineSecretForOteld(instance *types.OteldInstance, namespace string, mode v1alpha1.Mode, gc *types.OteldConfigGenerater) (*corev1.Secret, error) {
	if instance == nil || instance.Oteld == nil || !instance.Oteld.Spec.UseConfigMap {
		return nil, nil
	}

	name := fmt.Sprintf(OteldEngineSecretNamePattern, mode)

	commonLabels := map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		constant.AppNameLabelKey:      OTeldName,
		constant.AppInstanceLabelKey:  name,
	}

	configData, _ := gc.GenerateEngineConfiguration(instance)
	marshal, err := yaml.Marshal(configData)
	if err != nil {
		return nil, err
	}

	return builder.NewSecretBuilder(namespace, name).
		PutData("kb_engine.yaml", marshal).
		AddLabelsInMap(commonLabels).
		SetOwnerReferences(instance.Oteld.APIVersion, instance.Oteld.Kind, instance.Oteld).
		GetObject(), nil
}
