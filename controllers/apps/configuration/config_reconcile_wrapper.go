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

package configuration

import (
	"context"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type configSpecList []appsv1alpha1.ComponentConfigSpec

type configReconcileContext struct {
	Err    error
	Ctx    context.Context
	Client client.Client

	Name           string
	Component      string
	MatchingLabels client.MatchingLabels
	ConfigSpec     *appsv1alpha1.ComponentConfigSpec
	ConfigMap      *corev1.ConfigMap

	Cluster    *appsv1alpha1.Cluster
	ClusterDef *appsv1alpha1.ClusterDefinition
	ClusterVer *appsv1alpha1.ClusterVersion

	Containers   []string
	StatefulSets []appv1.StatefulSet
	RSMList      []workloads.ReplicatedStateMachine
	Deployments  []appv1.Deployment

	ConfigConstraint    *appsv1alpha1.ConfigConstraint
	ClusterDefComponent *appsv1alpha1.ClusterComponentDefinition
	ClusterComponent    *appsv1alpha1.ClusterComponentSpec
}

func newConfigReconcileContext(ctx context.Context, cli client.Client, cm *corev1.ConfigMap, cc *appsv1alpha1.ConfigConstraint, componentName string, configSpecName string, matchingLabels client.MatchingLabels) *configReconcileContext {
	return &configReconcileContext{
		Ctx:              ctx,
		Client:           cli,
		ConfigMap:        cm,
		ConfigConstraint: cc,
		Component:        componentName,
		Name:             configSpecName,
		MatchingLabels:   matchingLabels,
	}
}

func (l configSpecList) findByName(name string) *appsv1alpha1.ComponentConfigSpec {
	for i := range l {
		configSpec := &l[i]
		if configSpec.Name == name {
			return configSpec
		}
	}
	return nil
}

func (c *configReconcileContext) GetRelatedObjects() error {
	return c.cluster().
		clusterDef().
		clusterVer().
		clusterComponent().
		clusterDefComponent().
		statefulSet().
		rsm().
		deployment().
		complete()
}

func (c *configReconcileContext) objectWrapper(fn func() error) (ret *configReconcileContext) {
	ret = c
	if ret.Err != nil {
		return
	}
	ret.Err = fn()
	return
}

func (c *configReconcileContext) cluster() *configReconcileContext {
	clusterKey := client.ObjectKey{
		Namespace: c.ConfigMap.GetNamespace(),
		Name:      c.ConfigMap.Labels[constant.AppInstanceLabelKey],
	}
	return c.objectWrapper(func() error {
		c.Cluster = &appsv1alpha1.Cluster{}
		return c.Client.Get(c.Ctx, clusterKey, c.Cluster)
	})
}

func (c *configReconcileContext) clusterDef() *configReconcileContext {
	clusterDefKey := client.ObjectKey{
		Namespace: "",
		Name:      c.Cluster.Spec.ClusterDefRef,
	}
	return c.objectWrapper(func() error {
		c.ClusterDef = &appsv1alpha1.ClusterDefinition{}
		return c.Client.Get(c.Ctx, clusterDefKey, c.ClusterDef)
	})
}

func (c *configReconcileContext) clusterVer() *configReconcileContext {
	clusterVerKey := client.ObjectKey{
		Namespace: "",
		Name:      c.Cluster.Spec.ClusterVersionRef,
	}
	return c.objectWrapper(func() error {
		if clusterVerKey.Name == "" {
			return nil
		}
		c.ClusterVer = &appsv1alpha1.ClusterVersion{}
		return c.Client.Get(c.Ctx, clusterVerKey, c.ClusterVer)
	})
}
func (c *configReconcileContext) clusterDefComponent() *configReconcileContext {
	foundFn := func() (err error) {
		if c.ClusterComponent == nil {
			return
		}
		c.ClusterDefComponent = c.ClusterDef.GetComponentDefByName(c.ClusterComponent.ComponentDefRef)
		return
	}
	return c.objectWrapper(foundFn)
}

func (c *configReconcileContext) clusterComponent() *configReconcileContext {
	return c.objectWrapper(func() (err error) {
		c.ClusterComponent = c.Cluster.Spec.GetComponentByName(c.Component)
		return
	})
}

func (c *configReconcileContext) statefulSet() *configReconcileContext {
	stsFn := func() (err error) {
		dComp := c.ClusterDefComponent
		if dComp == nil || dComp.WorkloadType == appsv1alpha1.Stateless {
			return
		}
		c.StatefulSets, c.Containers, err = retrieveRelatedComponentsByConfigmap(
			c.Client,
			c.Ctx,
			c.Name,
			generics.StatefulSetSignature,
			client.ObjectKeyFromObject(c.ConfigMap),
			client.InNamespace(c.Cluster.Namespace),
			c.MatchingLabels)
		return
	}
	return c.objectWrapper(stsFn)
}

func (c *configReconcileContext) rsm() *configReconcileContext {
	stsFn := func() (err error) {
		dComp := c.ClusterDefComponent
		if dComp == nil {
			return
		}
		c.RSMList, c.Containers, err = retrieveRelatedComponentsByConfigmap(
			c.Client,
			c.Ctx,
			c.Name,
			generics.RSMSignature,
			client.ObjectKeyFromObject(c.ConfigMap),
			client.InNamespace(c.Cluster.Namespace),
			c.MatchingLabels)
		if err != nil {
			return
		}

		// fix uid mismatch bug: convert rsm to sts
		for _, rsm := range c.RSMList {
			var stsObject appv1.StatefulSet
			if err = c.Client.Get(c.Ctx, client.ObjectKeyFromObject(components.ConvertRSMToSTS(&rsm)), &stsObject); err != nil {
				return
			}
			c.StatefulSets = append(c.StatefulSets, stsObject)
		}
		return
	}
	return c.objectWrapper(stsFn)
}

func (c *configReconcileContext) deployment() *configReconcileContext {
	deployFn := func() (err error) {
		dComp := c.ClusterDefComponent
		if dComp == nil || dComp.WorkloadType != appsv1alpha1.Stateless {
			return
		}
		c.Deployments, c.Containers, err = retrieveRelatedComponentsByConfigmap(
			c.Client,
			c.Ctx,
			c.Name,
			generics.DeploymentSignature,
			client.ObjectKeyFromObject(c.ConfigMap),
			client.InNamespace(c.Cluster.Namespace),
			c.MatchingLabels)
		return
	}
	return c.objectWrapper(deployFn)
}

func (c *configReconcileContext) complete() (err error) {
	err = c.Err
	if err != nil {
		return
	}

	var configSpecs configSpecList
	if configSpecs, err = cfgcore.GetConfigTemplatesFromComponent(
		c.clusterComponents(),
		c.clusterDefComponents(),
		c.clusterVerComponents(),
		c.Component); err != nil {
		return
	}
	c.ConfigSpec = configSpecs.findByName(c.Name)
	return
}

func (c *configReconcileContext) clusterComponents() []appsv1alpha1.ClusterComponentSpec {
	return c.Cluster.Spec.ComponentSpecs
}

func (c *configReconcileContext) clusterDefComponents() []appsv1alpha1.ClusterComponentDefinition {
	return c.ClusterDef.Spec.ComponentDefs
}

func (c *configReconcileContext) clusterVerComponents() []appsv1alpha1.ClusterComponentVersion {
	if c.ClusterVer == nil {
		return nil
	}
	return c.ClusterVer.Spec.ComponentVersions
}
