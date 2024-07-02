/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package multiversion

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// covert appsv1alpha1.clusterdefinition resources to:
// - appsv1.clusterdefinition
// - appsv1.componentdefinition  // TODO

var (
	cdResource = "clusterdefinitions"
	cdGVR      = appsv1.GroupVersion.WithResource(cdResource)
)

func init() {
	hook.RegisterCRDConversion(cdGVR, hook.NewNoVersion(1, 0), cdHandler(),
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

func cdHandler() hook.ConversionHandler {
	return &convertor{
		kind: "ClusterDefinition",
		source: &cdConvertor{
			namespaces: []string{"default"}, // TODO: namespaces
		},
		target: &cdConvertor{},
	}
}

type cdConvertor struct {
	namespaces []string
}

func (c *cdConvertor) list(ctx context.Context, cli *versioned.Clientset, _ string) ([]client.Object, error) {
	list, err := cli.AppsV1alpha1().ClusterDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range list.Items {
		addons = append(addons, &list.Items[i])
	}
	return addons, nil
}

func (c *cdConvertor) used(ctx context.Context, cli *versioned.Clientset, _, name string) (bool, error) {
	return checkUsedByCluster(ctx, cli, constant.AppNameLabelKey, name, c.namespaces)
}

func (c *cdConvertor) get(ctx context.Context, cli *versioned.Clientset, _, name string) (client.Object, error) {
	return cli.AppsV1().ClusterDefinitions().Get(ctx, name, metav1.GetOptions{})
}

func (c *cdConvertor) convert(source client.Object) []client.Object {
	cd := source.(*appsv1alpha1.ClusterDefinition)
	return []client.Object{
		&appsv1.ClusterDefinition{
			Spec: appsv1.ClusterDefinitionSpec{
				Topologies: c.topologies(cd.Spec.Topologies),
			},
		},
	}
}

func (c *cdConvertor) topologies(topologies []appsv1alpha1.ClusterTopology) []appsv1.ClusterTopology {
	if len(topologies) == 0 {
		return nil
	}
	newTopologies := make([]appsv1.ClusterTopology, 0)
	for i := range topologies {
		topology := appsv1.ClusterTopology{
			Name:       topologies[i].Name,
			Components: make([]appsv1.ClusterTopologyComponent, 0),
			Default:    topologies[i].Default,
		}
		for _, comp := range topologies[i].Components {
			topology.Components = append(topology.Components, appsv1.ClusterTopologyComponent{
				Name:    comp.Name,
				CompDef: comp.CompDef,
			})
		}
		if topologies[i].Orders != nil {
			topology.Orders = &appsv1.ClusterTopologyOrders{
				Provision: topologies[i].Orders.Provision,
				Terminate: topologies[i].Orders.Terminate,
				Update:    topologies[i].Orders.Update,
			}
		}
		newTopologies = append(newTopologies, topology)
	}
	return newTopologies
}

// checkUsedByCluster checks if a resource is used by any cluster in the given namespaces.
func checkUsedByCluster(ctx context.Context, cli *versioned.Clientset, labelKey, resourceName string, namespaces []string) (bool, error) {
	selectors := []string{
		fmt.Sprintf("%s=%s", constant.AppManagedByLabelKey, constant.AppName),
		fmt.Sprintf("%s=%s", labelKey, resourceName),
	}
	opts := metav1.ListOptions{
		LabelSelector: strings.Join(selectors, ","),
	}

	used := false
	for _, namespace := range namespaces {
		clusterList, err := cli.AppsV1alpha1().Clusters(namespace).List(ctx, opts)
		if err != nil {
			return false, err
		}
		used = used || (len(clusterList.Items) > 0)
	}
	return used, nil
}

func generatedCmpdName(clusterDef, compDef string) string {
	return fmt.Sprintf("%s-%s", clusterDef, compDef)
}
