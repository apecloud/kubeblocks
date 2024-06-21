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
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
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
	hook.RegisterCRDConversion(cdGVR, hook.NewNoVersion(1, 0), &cdConvertor{},
		hook.NewNoVersion(0, 7),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

type cdConvertor struct {
	namespaces []string // TODO: namespaces

	cds           map[string]*appsv1alpha1.ClusterDefinition
	errors        map[string]error
	unused        sets.Set[string]
	native        sets.Set[string]
	beenConverted sets.Set[string]
	toBeConverted sets.Set[string]
}

func (c *cdConvertor) Convert(ctx context.Context, cli hook.CRClient) ([]client.Object, error) {
	cdList, err := cli.KBClient.AppsV1alpha1().ClusterDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i, cd := range cdList.Items {
		c.cds[cd.GetName()] = &cdList.Items[i]

		used, err1 := c.used(ctx, cli, cd.GetName(), c.namespaces)
		if err1 != nil {
			c.errors[cd.GetName()] = err1
			continue
		}
		if !used {
			c.unused.Insert(cd.GetName())
			continue
		}

		cdv1, err2 := c.existed(ctx, cli, cd.GetName())
		switch {
		case err2 != nil:
			c.errors[cd.GetName()] = err2
		case cdv1 == nil:
			c.toBeConverted.Insert(cd.GetName())
		case c.converted(cdv1):
			c.beenConverted.Insert(cd.GetName())
		default:
			c.native.Insert(cd.GetName())
		}
	}
	c.dump()

	objects := make([]client.Object, 0)
	for name := range c.toBeConverted {
		objects = append(objects, c.convert(c.cds[name]))
	}
	return objects, nil
}

func (c *cdConvertor) used(ctx context.Context, cli hook.CRClient, cdName string, namespaces []string) (bool, error) {
	selectors := []string{
		fmt.Sprintf("%s=%s", constant.AppManagedByLabelKey, constant.AppName),
		fmt.Sprintf("%s=%s", constant.AppNameLabelKey, cdName),
	}
	opts := metav1.ListOptions{
		LabelSelector: strings.Join(selectors, ","),
	}

	used := false
	for _, namespace := range namespaces {
		compList, err := cli.KBClient.AppsV1alpha1().Clusters(namespace).List(ctx, opts)
		if err != nil {
			return false, err
		}
		used = used || (len(compList.Items) > 0)
	}
	return used, nil
}

func (c *cdConvertor) existed(ctx context.Context, cli hook.CRClient, name string) (*appsv1.ClusterDefinition, error) {
	obj, err := cli.KBClient.AppsV1().ClusterDefinitions().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return obj, nil
}

func (c *cdConvertor) converted(cd *appsv1.ClusterDefinition) bool {
	if cd != nil && cd.GetAnnotations() != nil {
		_, ok := cd.GetAnnotations()[convertedFromAnnotationKey]
		return ok
	}
	return false
}

func (c *cdConvertor) dump() {
	hook.Log("ClusterDefinition conversion to v1 status")
	hook.Log("\tunused ClusterDefinitions")
	hook.Log(c.doubleTableFormat(sets.List(c.unused)))
	hook.Log("\thas native ClusterDefinitions defined")
	hook.Log(c.doubleTableFormat(sets.List(c.native)))
	hook.Log("\thas been converted ClusterDefinitions")
	hook.Log(c.doubleTableFormat(sets.List(c.beenConverted)))
	hook.Log("\tto be converted ClusterDefinitions")
	hook.Log(c.doubleTableFormat(sets.List(c.toBeConverted)))
	hook.Log("\terror occurred when perform pre-check")
	hook.Log(c.doubleTableFormat(maps.Keys(c.errors), c.errors))
}

func (c *cdConvertor) doubleTableFormat(items []string, errors ...map[string]error) string {
	formattedErr := func(key string) string {
		if len(errors) == 0 {
			return ""
		}
		if err, ok := errors[0][key]; ok {
			return fmt.Sprintf(": %s", err.Error())
		}
		return ""
	}
	var sb strings.Builder
	for _, item := range items {
		sb.WriteString("\t\t" + item + formattedErr(item) + "\n")
	}
	return sb.String()
}

func (c *cdConvertor) convert(cd *appsv1alpha1.ClusterDefinition) client.Object {
	// TODO: filter labels & annotations
	labels := func() map[string]string {
		return cd.GetLabels()
	}
	annotations := func() map[string]string {
		m := map[string]string{}
		maps.Copy(m, cd.GetAnnotations())
		b, _ := json.Marshal(cd)
		m[convertedFromAnnotationKey] = string(b)
		return m
	}
	return &appsv1.ClusterDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cd.GetName(),
			Labels:      labels(),
			Annotations: annotations(),
		},
		Spec: appsv1.ClusterDefinitionSpec{
			Topologies: c.topologies(cd.Spec.Topologies),
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

func generatedCmpdName(clusterDef, compDef string) string {
	return fmt.Sprintf("%s-%s", clusterDef, compDef)
}
