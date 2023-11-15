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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	rsmcore "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type configSpecList []appsv1alpha1.ComponentConfigSpec

type configReconcileContext struct {
	intctrlutil.ResourceFetcher[configReconcileContext]

	Name           string
	MatchingLabels client.MatchingLabels
	ConfigSpec     *appsv1alpha1.ComponentConfigSpec
	ConfigMap      *corev1.ConfigMap

	Containers   []string
	StatefulSets []appv1.StatefulSet
	RSMList      []workloads.ReplicatedStateMachine
	Deployments  []appv1.Deployment

	ConfigConstraint *appsv1alpha1.ConfigConstraint
}

func newConfigReconcileContext(resourceCtx *intctrlutil.ResourceCtx,
	cm *corev1.ConfigMap,
	cc *appsv1alpha1.ConfigConstraint,
	configSpecName string,
	matchingLabels client.MatchingLabels) *configReconcileContext {
	configContext := configReconcileContext{
		ConfigMap:        cm,
		ConfigConstraint: cc,
		Name:             configSpecName,
		MatchingLabels:   matchingLabels,
	}
	return configContext.Init(resourceCtx, &configContext)
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
	return c.Cluster().
		ClusterDef().
		ClusterVer().
		ClusterComponent().
		RSM().
		Complete()
}

func (c *configReconcileContext) RSM() *configReconcileContext {
	stsFn := func() (err error) {
		c.RSMList, c.Containers, err = retrieveRelatedComponentsByConfigmap(
			c.Client,
			c.Context,
			c.Name,
			generics.RSMSignature,
			client.ObjectKeyFromObject(c.ConfigMap),
			client.InNamespace(c.Namespace),
			c.MatchingLabels)
		if err != nil {
			return
		}

		// fix uid mismatch bug: convert rsm to sts
		// NODE: all components use the StatefulSet
		for _, rsm := range c.RSMList {
			var stsObject appv1.StatefulSet
			if err = c.Client.Get(c.Context, client.ObjectKeyFromObject(rsmcore.ConvertRSMToSTS(&rsm)), &stsObject); err != nil {
				return
			}
			c.StatefulSets = append(c.StatefulSets, stsObject)
		}
		return
	}
	return c.Wrap(stsFn)
}

func (c *configReconcileContext) Complete() (err error) {
	err = c.Err
	if err != nil {
		return
	}

	var configSpecs configSpecList
	if configSpecs, err = cfgcore.GetConfigTemplatesFromComponent(
		c.clusterComponents(),
		c.clusterDefComponents(),
		c.clusterVerComponents(),
		c.ComponentName); err != nil {
		return
	}
	c.ConfigSpec = configSpecs.findByName(c.Name)
	return
}

func (c *configReconcileContext) clusterComponents() []appsv1alpha1.ClusterComponentSpec {
	return c.ClusterObj.Spec.ComponentSpecs
}

func (c *configReconcileContext) clusterDefComponents() []appsv1alpha1.ClusterComponentDefinition {
	return c.ClusterDefObj.Spec.ComponentDefs
}

func (c *configReconcileContext) clusterVerComponents() []appsv1alpha1.ClusterComponentVersion {
	if c.ClusterVerObj == nil {
		return nil
	}
	return c.ClusterVerObj.Spec.ComponentVersions
}
