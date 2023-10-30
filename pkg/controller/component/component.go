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
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func FullName(clusterName, compName string) string {
	return constant.GenerateClusterComponentPattern(clusterName, compName)
}

func ShortName(clusterName, compName string) (string, error) {
	name, found := strings.CutPrefix(compName, fmt.Sprintf("%s-", clusterName))
	if !found {
		return "", fmt.Errorf("the component name has no cluster name as prefix: %s", compName)
	}
	return name, nil
}

// BuildProtoComponent builds a new Component object from cluster component spec and definition.
func BuildProtoComponent(cluster *appsv1alpha1.Cluster, clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {
	compName := FullName(cluster.Name, clusterCompSpec.Name)
	affinities := BuildAffinity(cluster, clusterCompSpec)
	tolerations, err := BuildTolerations(cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	builder := builder.NewComponentBuilder(cluster.Namespace, compName, cluster.Name, clusterCompSpec.ComponentDef).
		AddLabelsInMap(constant.GetComponentWellKnownLabels(cluster.Name, clusterCompSpec.Name)).
		SetAffinity(affinities).
		SetTolerations(tolerations).
		SetReplicas(clusterCompSpec.Replicas).
		SetResources(clusterCompSpec.Resources).
		SetMonitor(clusterCompSpec.Monitor).
		SetServiceAccountName(clusterCompSpec.ServiceAccountName).
		SetVolumeClaimTemplates(clusterCompSpec.VolumeClaimTemplates).
		SetUpdateStrategy(clusterCompSpec.UpdateStrategy).
		SetEnabledLogs(clusterCompSpec.EnabledLogs).
		SetServiceRefs(clusterCompSpec.ServiceRefs).
		SetClassRef(clusterCompSpec.ClassDefRef).
		SetTLSConfig(clusterCompSpec.TLS, clusterCompSpec.Issuer)
	return builder.GetObject(), nil
}

// BuildComponentDefinition constructs a ComponentDefinition object based on the following rules:
// 1. If the clusterCompSpec.EnableComponentDefinition feature gate is enabled, return the ComponentDefinition object corresponding to clusterCompSpec.ComponentDef directly.
// 2. Otherwise, generate the corresponding ComponentDefinition object from converting clusterComponentDefinition.
func BuildComponentDefinition(reqCtx ictrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	if clusterCompSpec.ComponentDef == "" {
		if clusterCompSpec.ComponentDefRef == "" {
			return nil, errors.New("invalid component spec")
		}
		if cluster.Spec.ClusterDefRef == "" {
			return nil, errors.New("clusterDefRef is required  when component def is not provided")
		}
	}

	if clusterCompSpec.ComponentDef != "" {
		compDef := &appsv1alpha1.ComponentDefinition{}
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: clusterCompSpec.ComponentDef}, compDef); err != nil {
			return nil, err
		}
		return compDef, nil
	} else {
		clusterDef, clusterVer, err := getClusterDefAndVersion(reqCtx.Ctx, cli, cluster)
		if err != nil {
			return nil, err
		}
		return BuildComponentDefinitionLow(clusterDef, clusterVer, cluster, clusterCompSpec)
	}
}

func BuildComponentDefinitionLow(clusterDef *appsv1alpha1.ClusterDefinition, clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	if clusterCompSpec.ComponentDefRef == "" {
		return nil, fmt.Errorf("cluster component definition ref is empty: %s-%s", cluster.Name, clusterCompSpec.Name)
	}
	clusterCompDef, clusterCompVer, err := getClusterCompDefAndVersion(clusterDef, clusterVer, cluster, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	return buildComponentDefinitionFrom(clusterCompDef, clusterCompVer, cluster.Name)
}

// getClusterDefAndVersion gets ClusterDefinition and ClusterVersion object from cluster.
func getClusterDefAndVersion(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, error) {
	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err := ictrlutil.ValidateExistence(ctx, cli, types.NamespacedName{Name: cluster.Spec.ClusterDefRef},
		clusterDef, false); err != nil {
		return nil, nil, err
	}

	var clusterVer *appsv1alpha1.ClusterVersion
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		clusterVerObj := &appsv1alpha1.ClusterVersion{}
		if err := ictrlutil.ValidateExistence(ctx, cli, types.NamespacedName{Name: cluster.Spec.ClusterVersionRef},
			clusterVerObj, false); err != nil {
			return nil, nil, err
		}
		clusterVer = clusterVerObj
	}

	return clusterDef, clusterVer, nil
}

// getClusterCompDefAndVersion gets ClusterComponentDefinition and ClusterComponentVersion object from cluster.
func getClusterCompDefAndVersion(clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ClusterComponentDefinition, *appsv1alpha1.ClusterComponentVersion, error) {
	clusterCompDef := clusterDef.GetComponentDefByName(clusterCompSpec.ComponentDefRef)
	if clusterCompDef == nil {
		return nil, nil, fmt.Errorf("referenced cluster component definiton is not defined: %s-%s", cluster.Name, clusterCompSpec.Name)
	}
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	if clusterVer != nil {
		clusterCompVer = clusterVer.Spec.GetDefNameMappingComponents()[clusterCompSpec.ComponentDefRef]
	}
	return clusterCompDef, clusterCompVer, nil
}
