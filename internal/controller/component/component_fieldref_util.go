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
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func buildCompoentRef(clusterDef *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterComp *appsv1alpha1.ClusterComponentSpec,
	component *SynthesizedComponent) error {

	compRef := clusterCompDef.ComponentRef
	if clusterComp.ComponentRef != nil {
		compRef = clusterComp.ComponentRef
	}

	if len(compRef) == 0 {
		return nil
	}

	envs := make([]corev1.EnvVar, 0)

	for _, compRef := range compRef {
		if compRef == nil {
			continue
		}

		referredCompName := compRef.ComponentName
		referredCompDefName := compRef.ComponentDefName
		// get referenced component by componentDefName and componentName
		referredComponent, referredComponentDef, err := getReferredComponent(clusterDef, cluster, referredCompDefName, referredCompName)
		if err != nil {
			if compRef.ReferenceStrategy == appsv1alpha1.RequiredStrategy {
				return err
			} else {
				klog.V(4).Infof("ComponentRef %s/%s is not found, but it is not required, so ignore it", referredCompDefName, referredCompName)
				continue
			}
		}

		if fieldEnvs, err := resolveFieldRefs(compRef.FieldRefs, referredComponent); err != nil {
			return err
		} else if len(fieldEnvs) > 0 {
			envs = append(envs, fieldEnvs...)
		}

		if serviceEnvs, err := resolveServiceFieldRefs(compRef.ServiceRefs, cluster, referredComponent, referredComponentDef); err != nil {
			return err
		} else if len(serviceEnvs) > 0 {
			envs = append(envs, serviceEnvs...)
		}
		if resourceEnvs, err := resolveResourceFieldRefs(compRef.ResourceFieldRefs, referredComponent); err != nil {
			return err
		} else if len(resourceEnvs) > 0 {
			envs = append(envs, resourceEnvs...)
		}
	}
	component.ComponentRefEnvs = envs
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
		envNamePrefix := svcref.EnvNamePrefix
		serviceName := svcref.ServiceName
		svcPort, err := getServicePort(referredComponentDef.Service, serviceName)
		if err != nil {
			return nil, err
		}

		// append component svc name
		envs = append(envs, corev1.EnvVar{Name: envNamePrefix + "_NAME", Value: fmt.Sprintf("%s-%s", cluster.Name, referredComponent.Name)})
		// append component svc port
		envs = append(envs, corev1.EnvVar{Name: envNamePrefix + "_PORT", Value: strconv.Itoa(int(svcPort.Port))})
		// append component subdomain
		envs = append(envs, corev1.EnvVar{Name: envNamePrefix + "_DOMAIN_SUFFIX", Value: fmt.Sprintf("%s-%s-headless.%s.svc", cluster.Name,
			referredComponent.Name, cluster.Namespace)})
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

func getReferredComponent(clusterDef *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster,
	compDefName string, compName string) (*appsv1alpha1.ClusterComponentSpec, *appsv1alpha1.ClusterComponentDefinition, error) {
	if len(compName) > 0 {
		// get component by name
		comp := cluster.Spec.GetComponentByName(compName)
		if comp == nil {
			return nil, nil, fmt.Errorf("component %s not found", compName)
		}
		if len(compDefName) > 0 && comp.ComponentDefRef != compDefName {
			return nil, nil, fmt.Errorf("component %s componentDef %s not match", compName, compDefName)
		}
		return comp, clusterDef.GetComponentDefByName(comp.ComponentDefRef), nil
	}

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

// extractFieldPathAsString extract fieldPath value from referredComponent
func extractFieldPathAsString(object *appsv1alpha1.ClusterComponentSpec, fieldPath string) (string, error) {
	switch fieldPath {
	case "primaryIndex":
		if object.PrimaryIndex == nil {
			return "", fmt.Errorf("primaryIndex not set")
		} else {
			return strconv.Itoa(int(*object.PrimaryIndex)), nil
		}
	case "replicas":
		return strconv.Itoa(int(object.Replicas)), nil
	case "name":
		return object.Name, nil
	}
	return "", fmt.Errorf("fieldPath %s not supported", fieldPath)
}
