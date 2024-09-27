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
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

// default reconcile requeue after duration
var requeueDuration = time.Millisecond * 1000

func boolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func mergeMap(dst, src map[string]string) {
	for key, val := range src {
		dst[key] = val
	}
}

func placement(obj client.Object) string {
	if obj == nil || obj.GetAnnotations() == nil {
		return ""
	}
	return obj.GetAnnotations()[constant.KBAppMultiClusterPlacementKey]
}

func intoContext(ctx context.Context, placement string) context.Context {
	return multicluster.IntoContext(ctx, placement)
}

func inDataContext4C() *multicluster.ClientOption {
	return multicluster.InDataContext()
}

func inUniversalContext4C() *multicluster.ClientOption {
	return multicluster.InUniversalContext()
}

func inDataContext4G() model.GraphOption {
	return model.WithClientOption(multicluster.InDataContext())
}

func inUniversalContext4G() model.GraphOption {
	return model.WithClientOption(multicluster.InUniversalContext())
}

func clientOption(v *model.ObjectVertex) *multicluster.ClientOption {
	if v.ClientOpt != nil {
		opt, ok := v.ClientOpt.(*multicluster.ClientOption)
		if ok {
			return opt
		}
		panic(fmt.Sprintf("unknown client option: %T", v.ClientOpt))
	}
	return multicluster.InControlContext()
}

func resolveServiceDefaultFields(oldSpec, newSpec *corev1.ServiceSpec) {
	var exist *corev1.ServicePort
	for i, port := range newSpec.Ports {
		for _, oldPort := range oldSpec.Ports {
			// assume that port.Name is user specified, if it is not changed, we need to keep the old NodePort and TargetPort if they are not set
			if port.Name != "" && port.Name == oldPort.Name {
				exist = &oldPort
				break
			}
		}
		if exist == nil {
			continue
		}
		// if the service type is NodePort or LoadBalancer, and the nodeport is not set, we should use the nodeport of the exist service
		if shouldAllocateNodePorts(newSpec) && port.NodePort == 0 && exist.NodePort != 0 {
			newSpec.Ports[i].NodePort = exist.NodePort
			port.NodePort = exist.NodePort
		}
		if port.TargetPort.IntVal == 0 && port.TargetPort.StrVal == "" {
			port.TargetPort = exist.TargetPort
		}
		if reflect.DeepEqual(port, *exist) {
			newSpec.Ports[i].TargetPort = exist.TargetPort
		}
	}
	if len(newSpec.ClusterIP) == 0 {
		newSpec.ClusterIP = oldSpec.ClusterIP
	}
	if len(newSpec.ClusterIPs) == 0 {
		newSpec.ClusterIPs = oldSpec.ClusterIPs
	}
	if len(newSpec.Type) == 0 {
		newSpec.Type = oldSpec.Type
	}
	if len(newSpec.SessionAffinity) == 0 {
		newSpec.SessionAffinity = oldSpec.SessionAffinity
	}
	if len(newSpec.IPFamilies) == 0 || (len(newSpec.IPFamilies) == 1 && *newSpec.IPFamilyPolicy != corev1.IPFamilyPolicySingleStack) {
		newSpec.IPFamilies = oldSpec.IPFamilies
	}
	if newSpec.IPFamilyPolicy == nil {
		newSpec.IPFamilyPolicy = oldSpec.IPFamilyPolicy
	}
	if newSpec.InternalTrafficPolicy == nil {
		newSpec.InternalTrafficPolicy = oldSpec.InternalTrafficPolicy
	}
	if newSpec.ExternalTrafficPolicy == "" && oldSpec.ExternalTrafficPolicy != "" {
		newSpec.ExternalTrafficPolicy = oldSpec.ExternalTrafficPolicy
	}
}

func shouldAllocateNodePorts(svc *corev1.ServiceSpec) bool {
	if svc.Type == corev1.ServiceTypeNodePort {
		return true
	}
	if svc.Type == corev1.ServiceTypeLoadBalancer {
		if svc.AllocateLoadBalancerNodePorts != nil {
			return *svc.AllocateLoadBalancerNodePorts
		}
		return true
	}
	return false
}
