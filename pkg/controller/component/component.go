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

package component

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// BuildProtoComponent builds a new Component object from cluster componentSpec.
func BuildProtoComponent(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {
	// check if clusterCompSpec enable the ComponentDefinition API feature gate.
	if clusterCompSpec.EnableComponentDefinition && clusterCompSpec.ComponentDef != "" {
		return buildProtoCompFromCompDef(reqCtx, cli, cluster, clusterCompSpec)
	}
	if !clusterCompSpec.EnableComponentDefinition && clusterCompSpec.ComponentDefRef != "" {
		if cluster.Spec.ClusterDefRef == "" {
			return nil, errors.New("clusterDefRef is required when enableComponentDefinition is false")
		}
		return buildProtoCompFromConvertor(reqCtx, cli, cluster, clusterCompSpec)
	}
	return nil, errors.New("invalid component spec")
}

// BuildComponentDefinition constructs a ComponentDefinition object based on the following rules:
// 1. If the clusterCompSpec.EnableComponentDefinition feature gate is enabled, return the ComponentDefinition object corresponding to clusterCompSpec.ComponentDef directly.
// 2. Otherwise, generate the corresponding ComponentDefinition object from converting clusterComponentDefinition.
func BuildComponentDefinition(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	// check if clusterCompSpec enable the ComponentDefinition API feature gate.
	if clusterCompSpec.EnableComponentDefinition && clusterCompSpec.ComponentDef != "" {
		cmpd := &appsv1alpha1.ComponentDefinition{}
		if err := ictrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: clusterCompSpec.ComponentDef}, cmpd, false); err != nil {
			return nil, err
		}
		return cmpd, nil
	}
	if !clusterCompSpec.EnableComponentDefinition && clusterCompSpec.ComponentDefRef != "" {
		if cluster.Spec.ClusterDefRef == "" {
			return nil, errors.New("clusterDefRef is required when enableComponentDefinition is false")
		}
		return buildCompDefFromConvertor(reqCtx, cli, cluster, clusterCompSpec)
	}
	return nil, errors.New("invalid component spec")
}

// buildCompDefFromConvertor builds a new ComponentDefinition object based on converting clusterComponentDefinition to ComponentDefinition.
func buildCompDefFromConvertor(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	clusterCompDef, clusterCompVer, err := getClusterCompDefAndVersion(reqCtx, cli, cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	return BuildComponentDefinitionFrom(clusterCompDef, clusterCompVer, cluster.Name)
}

// buildProtoCompFromConvertor builds a new Component object based on converting clusterComponentDefinition to ComponentDefinition.
func buildProtoCompFromConvertor(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {
	clusterCompDef, clusterCompVer, err := getClusterCompDefAndVersion(reqCtx, cli, cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	return BuildComponentFrom(clusterCompDef, clusterCompVer, clusterCompSpec)
}

// buildProtoCompFromCompDef builds a new Component object based on ComponentDefinition API.
func buildProtoCompFromCompDef(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {

	if clusterCompSpec == nil || clusterCompSpec.ComponentDef == "" {
		return nil, errors.New("invalid component spec")
	}

	cmpd := &appsv1alpha1.ComponentDefinition{}
	if err := ictrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: clusterCompSpec.ComponentDef}, cmpd, false); err != nil {
		return nil, err
	}

	comp := builder.NewComponentBuilder(cluster.Namespace, clusterCompSpec.Name, cluster.Name, clusterCompSpec.ComponentDef).
		SetAffinity(clusterCompSpec.Affinity).
		SetTolerations(clusterCompSpec.Tolerations).
		SetReplicas(clusterCompSpec.Replicas).
		SetResources(clusterCompSpec.Resources).
		SetMonitor(clusterCompSpec.Monitor).
		SetServiceAccountName(clusterCompSpec.ServiceAccountName).
		SetVolumeClaimTemplates(clusterCompSpec.VolumeClaimTemplates).
		SetUpdateStrategy(clusterCompSpec.UpdateStrategy).
		SetEnabledLogs(clusterCompSpec.EnabledLogs).
		SetServiceRefs(clusterCompSpec.ServiceRefs).
		SetClassRef(clusterCompSpec.ClassDefRef).
		SetIssuer(clusterCompSpec.Issuer).
		SetTLS(clusterCompSpec.TLS).
		GetObject()

	return comp, nil
}

// getClusterCompDefAndVersion gets ClusterComponentDefinition and ClusterComponentVersion object from cluster.
func getClusterCompDefAndVersion(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ClusterComponentDefinition, *appsv1alpha1.ClusterComponentVersion, error) {
	if cluster.Spec.ClusterDefRef == "" || cluster.Spec.ClusterVersionRef == "" {
		return nil, nil, errors.New("clusterDefRef and ClusterVersionRef is required when enableComponentDefinition is false")
	}
	cd, cv, err := getClusterDefAndVersion(reqCtx, cli, cluster)
	if err != nil {
		return nil, nil, err
	}
	var clusterCompDef *appsv1alpha1.ClusterComponentDefinition
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	clusterCompDef = cd.GetComponentDefByName(clusterCompSpec.ComponentDefRef)
	if clusterCompDef == nil {
		return nil, nil, fmt.Errorf("referenced component definition does not exist, cluster: %s, component: %s, component definition ref:%s", cluster.Name, clusterCompSpec.Name, clusterCompSpec.ComponentDefRef)
	}
	if cv != nil {
		clusterCompVer = cv.Spec.GetDefNameMappingComponents()[clusterCompSpec.ComponentDefRef]
	}
	return clusterCompDef, clusterCompVer, nil
}

// getClusterDefAndVersion gets ClusterDefinition and ClusterVersion object from cluster.
func getClusterDefAndVersion(reqCtx ictrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, error) {
	cd := &appsv1alpha1.ClusterDefinition{}
	if err := ictrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, cd, false); err != nil {
		return nil, nil, err
	}

	cv := &appsv1alpha1.ClusterVersion{}
	if err := ictrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, cv, false); err != nil {
		return nil, nil, err
	}

	return cd, cv, nil
}
