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

package lifecycle

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

const (
	hackedAllCompList = "KB_CLUSTER_COMPONENT_LIST" // declare it here for test
)

type postProvision struct {
	namespace   string
	clusterName string
	compName    string
	action      *appsv1.Action
}

var _ lifecycleAction = &postProvision{}

func (a *postProvision) name() string {
	return "postProvision"
}

func (a *postProvision) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	return hackParameters4Comp(ctx, cli, a.namespace, a.clusterName, a.compName, false)
}

type preTerminate struct {
	namespace   string
	clusterName string
	compName    string
	action      *appsv1.Action
}

var _ lifecycleAction = &preTerminate{}

func (a *preTerminate) name() string {
	return "preTerminate"
}

func (a *preTerminate) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	return hackParameters4Comp(ctx, cli, a.namespace, a.clusterName, a.compName, true)
}

////////// hack for legacy Addons //////////
// The container executing this action has access to following variables:
//
// - KB_CLUSTER_POD_IP_LIST: Comma-separated list of the cluster's pod IP addresses (e.g., "podIp1,podIp2").
// - KB_CLUSTER_POD_NAME_LIST: Comma-separated list of the cluster's pod names (e.g., "pod1,pod2").
// - KB_CLUSTER_POD_HOST_NAME_LIST: Comma-separated list of host names, each corresponding to a pod in
//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostName1,hostName2").
// - KB_CLUSTER_POD_HOST_IP_LIST: Comma-separated list of host IP addresses, each corresponding to a pod in
//   KB_CLUSTER_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
//
// - KB_CLUSTER_COMPONENT_POD_NAME_LIST: Comma-separated list of all pod names within the component
//   (e.g., "pod1,pod2").
// - KB_CLUSTER_COMPONENT_POD_IP_LIST: Comma-separated list of pod IP addresses,
//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "podIp1,podIp2").
// - KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST: Comma-separated list of host names for each pod,
//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostName1,hostName2").
// - KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST: Comma-separated list of host IP addresses for each pod,
//   matching the order of pods in KB_CLUSTER_COMPONENT_POD_NAME_LIST (e.g., "hostIp1,hostIp2").
//
// - KB_CLUSTER_COMPONENT_LIST: Comma-separated list of all cluster components (e.g., "comp1,comp2").
// - KB_CLUSTER_COMPONENT_DELETING_LIST: Comma-separated list of components that are currently being deleted
//   (e.g., "comp1,comp2").
// - KB_CLUSTER_COMPONENT_UNDELETED_LIST: Comma-separated list of components that are not being deleted
//   (e.g., "comp1,comp2").
//
// - KB_CLUSTER_COMPONENT_IS_SCALING_IN: Indicates whether the component is currently scaling in.
//   If this variable is present and set to "true", it denotes that the component is undergoing a scale-in operation.
//   During scale-in, data rebalancing is necessary to maintain cluster integrity.
//   Contrast this with a cluster deletion scenario where data rebalancing is not required as the entire cluster
//   is being cleaned up.

func hackParameters4Comp(ctx context.Context, cli client.Reader, namespace, clusterName, compName string, terminate bool) (map[string]string, error) {
	const (
		clusterPodNameList     = "KB_CLUSTER_POD_NAME_LIST"
		clusterPodIPList       = "KB_CLUSTER_POD_IP_LIST"
		clusterPodHostNameList = "KB_CLUSTER_POD_HOST_NAME_LIST"
		clusterPodHostIPList   = "KB_CLUSTER_POD_HOST_IP_LIST"
		compPodNameList        = "KB_CLUSTER_COMPONENT_POD_NAME_LIST"
		compPodIPList          = "KB_CLUSTER_COMPONENT_POD_IP_LIST"
		compPodHostNameList    = "KB_CLUSTER_COMPONENT_POD_HOST_NAME_LIST"
		compPodHostIPList      = "KB_CLUSTER_COMPONENT_POD_HOST_IP_LIST"
		allCompList            = hackedAllCompList
		deletingCompList       = "KB_CLUSTER_COMPONENT_DELETING_LIST"
		undeletedCompList      = "KB_CLUSTER_COMPONENT_UNDELETED_LIST"
		scalingInComp          = "KB_CLUSTER_COMPONENT_IS_SCALING_IN"
	)

	compList := &appsv1.ComponentList{}
	if err := cli.List(ctx, compList, client.InNamespace(namespace), client.MatchingLabels{constant.AppInstanceLabelKey: clusterName}); err != nil {
		return nil, err
	}

	m := map[string]string{}
	if err := func() error {
		cl := make([][]string, 0)
		ccl := make([][]string, 0)
		for _, comp := range compList.Items {
			name, _ := component.ShortName(clusterName, comp.Name)
			pods, err := component.ListOwnedPods(ctx, cli, namespace, clusterName, name)
			if err != nil {
				return err
			}
			for _, pod := range pods {
				cl = append(cl, []string{pod.Name, pod.Status.PodIP, pod.Spec.NodeName, pod.Status.HostIP})
			}
			if name == compName {
				for _, pod := range pods {
					ccl = append(ccl, []string{pod.Name, pod.Status.PodIP, pod.Spec.NodeName, pod.Status.HostIP})
				}
			}
		}
		slicingNJoin := func(m [][]string, i int32) string {
			r := make([]string, 0)
			for _, l := range m {
				r = append(r, l[i])
			}
			return strings.Join(r, ",")
		}
		m[clusterPodNameList] = slicingNJoin(cl, 0)
		m[clusterPodIPList] = slicingNJoin(cl, 1)
		m[clusterPodHostNameList] = slicingNJoin(cl, 2)
		m[clusterPodHostIPList] = slicingNJoin(cl, 3)
		m[compPodNameList] = slicingNJoin(ccl, 0)
		m[compPodIPList] = slicingNJoin(ccl, 1)
		m[compPodHostNameList] = slicingNJoin(ccl, 2)
		m[compPodHostIPList] = slicingNJoin(ccl, 3)
		return nil
	}(); err != nil {
		return nil, err
	}

	func() {
		all, deleting, undeleted := make([]string, 0), make([]string, 0), make([]string, 0)
		for _, comp := range compList.Items {
			name, _ := component.ShortName(clusterName, comp.Name)
			all = append(all, name)
			if model.IsObjectDeleting(&comp) {
				deleting = append(deleting, name)
			} else {
				undeleted = append(undeleted, name)
			}
		}
		m[allCompList] = strings.Join(all, ",")
		m[deletingCompList] = strings.Join(deleting, ",")
		m[undeletedCompList] = strings.Join(undeleted, ",")
	}()

	if terminate {
		func() {
			for _, comp := range compList.Items {
				name, _ := component.ShortName(clusterName, comp.Name)
				if name == compName {
					if comp.Annotations != nil {
						val, ok := comp.Annotations[constant.ComponentScaleInAnnotationKey]
						if ok {
							m[scalingInComp] = val
						}
					}
					break
				}
			}
		}()
	}
	return m, nil
}
