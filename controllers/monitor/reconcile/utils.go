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

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/apecloud/kubeblocks/apis/monitor/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/monitor/types"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

const (
	OteldConfigMapNamePattern = "oteld-configmap-%s"
	OteldServiceNamePattern   = "oteld-service-%s"
	OteldSecretNamePattern    = "oteld-secret-%s"
)

// var (
//	defaultMetricsPort = 8888
// )

func buildPodSpecForOteld(template *v1alpha1.OTeld) *corev1.PodSpec {
	container := corev1.Container{
		Name:            OTeldName,
		Image:           template.Spec.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"--config=/etc/oteld/config/config.yaml",
		},
		Env: template.Spec.Env,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: 1234,
				//HostPort:      1234,
			},
		},
		Resources:       template.Spec.Resources,
		VolumeMounts:    template.Spec.VolumeMounts,
		SecurityContext: &template.Spec.SecurityContext,
	}

	return &builder.NewPodBuilder("", "").
		AddSerciveAccount("oteld-controller").
		AddContainer(container).
		AddVolumes(template.Spec.Volumes...).
		SetSecurityContext(template.Spec.PodSecurityContext).
		GetObject().Spec
}

func buildSvcForOtel(oteld *v1alpha1.OTeld, namespace string) (*corev1.Service, error) {
	if oteld == nil {
		return nil, nil
	}

	name := fmt.Sprintf(OteldServiceNamePattern, oteld.Spec.Mode)
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

func buildConfigMapForOteld(instance *types.OteldInstance, namespace string, exporters *types.Exporters, gc *types.OteldConfigGenerater) (*corev1.ConfigMap, error) {
	if instance == nil || instance.Oteld == nil || !instance.Oteld.Spec.UseConfigMap {
		return nil, nil
	}

	name := fmt.Sprintf(OteldConfigMapNamePattern, instance.Oteld.Spec.Mode)

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

func buildSecretForOteld(instance *types.OteldInstance, namespace string, exporters *types.Exporters, gc *types.OteldConfigGenerater) (*corev1.Secret, error) {
	if instance == nil || instance.Oteld == nil {
		return nil, nil
	}

	name := fmt.Sprintf(OteldSecretNamePattern, instance.Oteld.Spec.Mode)

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
