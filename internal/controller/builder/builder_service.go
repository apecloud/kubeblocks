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
