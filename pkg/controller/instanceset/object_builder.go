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

package instanceset

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

func buildHeadlessSvc(its workloads.InstanceSet, labels, selectors map[string]string) *corev1.Service {
	annotations := ParseAnnotationsOfScope(HeadlessServiceScope, its.Annotations)
	hdlBuilder := builder.NewHeadlessServiceBuilder(its.Namespace, getHeadlessSvcName(its.Name)).
		AddLabelsInMap(labels).
		AddSelectorsInMap(selectors).
		AddAnnotationsInMap(annotations).
		SetPublishNotReadyAddresses(true)

	portNames := sets.New[string]()
	for _, container := range its.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			servicePort := corev1.ServicePort{
				Protocol: port.Protocol,
				Port:     port.ContainerPort,
			}
			switch {
			case len(port.Name) > 0 && !portNames.Has(port.Name):
				portNames.Insert(port.Name)
				servicePort.Name = port.Name
			default:
				servicePort.Name = fmt.Sprintf("%s-%d", strings.ToLower(string(port.Protocol)), port.ContainerPort)
			}
			hdlBuilder.AddPorts(servicePort)
		}
	}
	return hdlBuilder.GetObject()
}

func getHeadlessSvcName(itsName string) string {
	return strings.Join([]string{itsName, "headless"}, "-")
}
