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

package builder

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ServiceBuilder struct {
	BaseBuilder[corev1.Service, *corev1.Service, ServiceBuilder]
}

func NewServiceBuilder(namespace, name string) *ServiceBuilder {
	builder := &ServiceBuilder{}
	builder.init(namespace, name, &corev1.Service{}, builder)
	return builder
}

func NewHeadlessServiceBuilder(namespace, name string) *ServiceBuilder {
	builder := &ServiceBuilder{}
	builder.init(namespace, name, &corev1.Service{}, builder)
	builder.SetType(corev1.ServiceTypeClusterIP)
	builder.get().Spec.ClusterIP = corev1.ClusterIPNone
	return builder
}

func (builder *ServiceBuilder) AddSelector(key, value string) *ServiceBuilder {
	keyValues := make(map[string]string, 1)
	keyValues[key] = value
	return builder.AddSelectorsInMap(keyValues)
}

func (builder *ServiceBuilder) AddSelectors(keyValues ...string) *ServiceBuilder {
	return builder.AddSelectorsInMap(WithMap(keyValues...))
}

func (builder *ServiceBuilder) AddSelectorsInMap(keyValues map[string]string) *ServiceBuilder {
	selectors := builder.get().Spec.Selector
	if selectors == nil {
		selectors = make(map[string]string, 0)
	}
	for k, v := range keyValues {
		selectors[k] = v
	}
	builder.get().Spec.Selector = selectors
	return builder
}

func (builder *ServiceBuilder) AddPorts(ports ...corev1.ServicePort) *ServiceBuilder {
	portList := builder.get().Spec.Ports
	if portList == nil {
		portList = make([]corev1.ServicePort, 0)
	}
	portList = append(portList, ports...)
	builder.get().Spec.Ports = portList
	return builder
}

func (builder *ServiceBuilder) AddContainerPorts(ports ...corev1.ContainerPort) *ServiceBuilder {
	servicePorts := make([]corev1.ServicePort, 0)
	for _, containerPort := range ports {
		servicePort := corev1.ServicePort{
			Name:       containerPort.Name,
			Protocol:   containerPort.Protocol,
			Port:       containerPort.ContainerPort,
			TargetPort: intstr.FromString(containerPort.Name),
		}
		servicePorts = append(servicePorts, servicePort)
	}
	return builder.AddPorts(servicePorts...)
}

func (builder *ServiceBuilder) SetType(serviceType corev1.ServiceType) *ServiceBuilder {
	if serviceType == "" {
		return builder
	}
	builder.get().Spec.Type = serviceType
	if serviceType == corev1.ServiceTypeLoadBalancer {
		// Set externalTrafficPolicy to Local has two benefits:
		// 1. preserve client IP
		// 2. improve network performance by reducing one hop
		builder.get().Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
	}
	return builder
}
