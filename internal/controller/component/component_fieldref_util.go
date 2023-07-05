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

package component

import (
	"bytes"
	"fmt"
	"html/template"
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func buildComponentRef(clusterDef *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterComp *appsv1alpha1.ClusterComponentSpec,
	component *SynthesizedComponent) error {

	compRef := clusterCompDef.ComponentRef
	if len(compRef) == 0 {
		return nil
	}

	component.ComponentRefEnvs = make([]corev1.EnvVar, 0)
	for _, compRef := range compRef {
		if compRef == nil {
			continue
		}
		selector := compRef.ComponentDefName
		// get referenced component by componentDefName and componentName
		referredComponent, referredComponentDef, err := getReferredComponent(clusterDef, cluster, selector)
		if err != nil {
			if compRef.FailurePolicy == appsv1alpha1.FailurePolicyFail {
				return err
			} else {
				klog.Errorf("ComponentSelector: %s failes to match,", selector)
				continue
			}
		}

		if envs, err := resolveFieldRefs(compRef.FieldRefs, referredComponent); err != nil {
			return err
		} else {
			component.ComponentRefEnvs = append(component.ComponentRefEnvs, envs...)
		}

		if envs, err := resolveServiceFieldRefs(compRef.ServiceRefs, cluster, referredComponent, referredComponentDef); err != nil {
			return err
		} else {
			component.ComponentRefEnvs = append(component.ComponentRefEnvs, envs...)
		}

		if envs, err := resolveHeadlessServiceFieldRefs(compRef.HeadlessServiceRefs, cluster, referredComponent, referredComponentDef); err != nil {
			return err
		} else {
			component.ComponentRefEnvs = append(component.ComponentRefEnvs, envs...)
		}

		if envs, err := resolveResourceFieldRefs(compRef.ResourceFieldRefs, referredComponent); err != nil {
			return err
		} else {
			component.ComponentRefEnvs = append(component.ComponentRefEnvs, envs...)
		}
	}
	return nil
}

func resolveFieldRefs(fieldRefs []*appsv1alpha1.ComponentFieldRef,
	referredComponent *appsv1alpha1.ClusterComponentSpec) ([]corev1.EnvVar, error) {
	if len(fieldRefs) == 0 {
		return nil, nil
	}

	envs := make([]corev1.EnvVar, 0)
	for _, fieldRef := range fieldRefs {
		fieldPath := fieldRef.FieldPath
		envName := fieldRef.EnvName
		value, err := extractFieldPathAsString(referredComponent, fieldPath)
		if err != nil {
			return nil, err
		}
		envs = append(envs, corev1.EnvVar{Name: envName, Value: value})
	}
	return envs, nil
}

func resolveServiceFieldRefs(serviceRefs []*appsv1alpha1.ComponentServiceRef,
	cluster *appsv1alpha1.Cluster,
	referredComponent *appsv1alpha1.ClusterComponentSpec,
	referredComponentDef *appsv1alpha1.ClusterComponentDefinition) ([]corev1.EnvVar, error) {
	if len(serviceRefs) == 0 {
		return nil, nil
	}

	envs := make([]corev1.EnvVar, 0)
	for _, svcref := range serviceRefs {
		envNamePrefix := svcref.EnvName
		serviceName := svcref.ServiceName
		svcPort, err := getServicePort(referredComponentDef.Service, serviceName)
		if err != nil {
			return nil, err
		}
		// append component svc name
		envs = append(envs, corev1.EnvVar{Name: envNamePrefix + "_NAME", Value: fmt.Sprintf("%s-%s", cluster.Name, referredComponent.Name)})
		// append component svc port
		envs = append(envs, corev1.EnvVar{Name: envNamePrefix + "_PORT", Value: strconv.Itoa(int(svcPort.Port))})
	}
	return envs, nil
}

type headlessSvc struct {
	Hostname string `json:"hostname"`
	FQDN     string `json:"fqdn"`
	Port     int32  `json:"port"`
	Ordinal  int32  `json:"ordinal"`
}

func resolveHeadlessServiceFieldRefs(serviceRefs []*appsv1alpha1.ComponentHeadlessServiceRef,
	cluster *appsv1alpha1.Cluster,
	referredComponent *appsv1alpha1.ClusterComponentSpec,
	referredComponentDef *appsv1alpha1.ClusterComponentDefinition) ([]corev1.EnvVar, error) {
	if len(serviceRefs) == 0 {
		return nil, nil
	}

	envs := make([]corev1.EnvVar, 0)
	hosts := make([]string, 0)
	for _, svcref := range serviceRefs {
		envNamePrefix := svcref.EnvName
		portName := svcref.PortName
		containerName := svcref.ContainerName

		port, err := getContainerPort(referredComponentDef, containerName, portName)
		if err != nil {
			return nil, err
		}

		for i := int32(0); i < referredComponent.Replicas; i++ {
			if svcref.Top != nil && *svcref.Top != 0 && i >= *svcref.Top {
				break
			}

			headlessSvc := headlessSvc{
				Hostname: fmt.Sprintf("%s-%s-%d", cluster.Name, referredComponent.Name, i),
				FQDN:     fmt.Sprintf("%s-%s-%d.%s-%s-headless.%s.svc", cluster.Name, referredComponent.Name, i, cluster.Name, referredComponent.Name, cluster.Namespace),
				Port:     port.ContainerPort,
				Ordinal:  i,
			}

			tmpl, err := template.New("headlessSvc").Parse(svcref.Format)
			if err != nil {
				return nil, err
			}

			var buf bytes.Buffer
			if err = tmpl.Execute(&buf, headlessSvc); err != nil {
				return nil, err
			} else {
				hosts = append(hosts, buf.String())
			}
		}
		envs = append(envs, corev1.EnvVar{Name: envNamePrefix, Value: strings.Join(hosts, svcref.JoinWith)})
	}
	return envs, nil
}

func resolveResourceFieldRefs(resourceFieldRefs []*appsv1alpha1.ComponentResourceFieldRef,
	referredComponent *appsv1alpha1.ClusterComponentSpec) ([]corev1.EnvVar, error) {
	if len(resourceFieldRefs) == 0 {
		return nil, nil
	}
	envs := make([]corev1.EnvVar, 0)
	for _, resourceFieldRef := range resourceFieldRefs {
		envName := resourceFieldRef.EnvName
		envValue, err := extractComponentResourceValue(resourceFieldRef, referredComponent)
		if err != nil {
			return nil, err
		}
		envs = append(envs, corev1.EnvVar{Name: envName, Value: envValue})
	}
	return envs, nil
}

func getReferredComponent(clusterDef *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster, compDefName string) (*appsv1alpha1.ClusterComponentSpec, *appsv1alpha1.ClusterComponentDefinition, error) {
	if len(compDefName) > 0 {
		mapping := cluster.Spec.GetDefNameMappingComponents()
		if comps, ok := mapping[compDefName]; ok {
			if len(comps) > 1 {
				return nil, nil, fmt.Errorf("componentDef %s found multiple components", compDefName)
			}
			return &comps[0], clusterDef.GetComponentDefByName(compDefName), nil
		} else {
			return nil, nil, fmt.Errorf("componentDef %s not found", compDefName)
		}
	}
	return nil, nil, fmt.Errorf("must specify either componentName or componentDefName")
}

func getServicePort(serviceSpec *appsv1alpha1.ServiceSpec, serviceName string) (*appsv1alpha1.ServicePort, error) {
	if serviceSpec == nil {
		return nil, fmt.Errorf("serviceSpec is nil")
	}
	for _, svc := range serviceSpec.Ports {
		if svc.Name == serviceName {
			return &svc, nil
		}
	}
	return nil, fmt.Errorf("service %s not found", serviceName)
}

func getContainerPort(referredComponentDef *appsv1alpha1.ClusterComponentDefinition,
	containerName, portName string) (*corev1.ContainerPort, error) {
	if referredComponentDef == nil {
		return nil, fmt.Errorf("referredComponentDef is nil")
	}
	idx, container := intctrlutil.GetContainerByName(referredComponentDef.PodSpec.Containers, containerName)
	if idx < 0 {
		return nil, fmt.Errorf("container %s not found", containerName)
	}
	for _, port := range container.Ports {
		if port.Name == portName {
			return &port, nil
		}
	}
	return nil, fmt.Errorf("port %s not found", portName)
}

// extractFieldPathAsString extracts fieldPath value from referredComponent
func extractFieldPathAsString(object *appsv1alpha1.ClusterComponentSpec, fieldPath string) (string, error) {
	// get field value by fieldPath
	value := reflect.ValueOf(*object).FieldByName(fieldPath)
	if !value.IsValid() {
		return "", fmt.Errorf("fieldPath %s not found", fieldPath)
	}
	switch value.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(value.Bool()), nil
	case reflect.String:
		return value.String(), nil
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.Itoa(int(value.Int())), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("fieldPath %s is not string, int32 or bool", fieldPath)
	}
}
