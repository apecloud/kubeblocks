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

package component

import (
	"context"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func inDataContext() *multicluster.ClientOption {
	return multicluster.InDataContext()
}

func IsHostNetworkEnabled(synthesizedComp *SynthesizedComponent) bool {
	if !hasHostNetworkCapability(synthesizedComp, nil) {
		return false
	}
	// legacy definition, ignore the cluster annotations
	if synthesizedComp.PodSpec.HostNetwork {
		return true
	}
	return hasHostNetworkEnabled(synthesizedComp.Annotations, synthesizedComp.Name)
}

func isHostNetworkEnabled(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, compName string) (bool, error) {
	// fast path: refer to self
	if compName == synthesizedComp.Name {
		return IsHostNetworkEnabled(synthesizedComp), nil
	}

	// check the component object that whether the host-network is enabled
	compKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, compName),
	}
	comp := &appsv1alpha1.Component{}
	if err := cli.Get(ctx, compKey, comp, inDataContext()); err != nil {
		return false, err
	}
	if !hasHostNetworkEnabled(comp.Annotations, compName) {
		return false, nil
	}

	// check the component definition that whether it has the host-network capability
	if len(comp.Spec.CompDef) > 0 {
		compDef := &appsv1alpha1.ComponentDefinition{}
		if err := cli.Get(ctx, types.NamespacedName{Name: comp.Spec.CompDef}, compDef); err != nil {
			return false, err
		}
		if hasHostNetworkCapability(nil, compDef) {
			return true, nil
		}
	}
	return false, nil
}

func hasHostNetworkCapability(synthesizedComp *SynthesizedComponent, compDef *appsv1alpha1.ComponentDefinition) bool {
	switch {
	case synthesizedComp != nil:
		return synthesizedComp.HostNetwork != nil
	case compDef != nil:
		return compDef.Spec.HostNetwork != nil
	}
	return false
}

func hasHostNetworkEnabled(annotations map[string]string, compName string) bool {
	if annotations == nil {
		return false
	}
	comps, ok := annotations[constant.HostNetworkAnnotationKey]
	if !ok {
		return false
	}
	return slices.Index(strings.Split(comps, ","), compName) >= 0
}

func getHostNetworkPort(ctx context.Context, _ client.Reader, clusterName, compName, cName, pName string) (int32, error) {
	key := intctrlutil.BuildHostPortName(clusterName, compName, cName, pName)
	if v, ok := ctx.Value(mockHostNetworkPortManagerKey{}).(map[string]int32); ok {
		if p, okk := v[key]; okk {
			return p, nil
		}
		return 0, nil
	}
	pm := intctrlutil.GetPortManager()
	if pm == nil {
		return 0, nil
	}
	return pm.GetPort(key)
}

func mockHostNetworkPort(ctx context.Context, _ client.Reader, clusterName, compName, cName, pName string, port int32) context.Context {
	key := intctrlutil.BuildHostPortName(clusterName, compName, cName, pName)
	mockHostNetworkPortManager[key] = port
	return context.WithValue(ctx, mockHostNetworkPortManagerKey{}, mockHostNetworkPortManager)
}

var (
	mockHostNetworkPortManager = map[string]int32{}
)

type mockHostNetworkPortManagerKey struct{}
