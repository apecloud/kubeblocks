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
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/apiconversion"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/scheduling"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func FullName(clusterName, compName string) string {
	return constant.GenerateClusterComponentName(clusterName, compName)
}

func ShortName(clusterName, compName string) (string, error) {
	name, found := strings.CutPrefix(compName, fmt.Sprintf("%s-", clusterName))
	if !found {
		return "", fmt.Errorf("the component name has no cluster name as prefix: %s", compName)
	}
	return name, nil
}

func GetClusterName(comp *appsv1alpha1.Component) (string, error) {
	return getCompLabelValue(comp, constant.AppInstanceLabelKey)
}

func GetClusterUID(comp *appsv1alpha1.Component) (string, error) {
	return getCompLabelValue(comp, constant.KBAppClusterUIDLabelKey)
}

// IsGenerated checks if the component is generated from legacy cluster definitions.
func IsGenerated(comp *appsv1alpha1.Component) bool {
	return len(comp.Spec.CompDef) == 0
}

// BuildComponent builds a new Component object from cluster component spec and definition.
func BuildComponent(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec,
	labels, annotations map[string]string) (*appsv1alpha1.Component, error) {
	compName := FullName(cluster.Name, compSpec.Name)
	compDefName := func() string {
		if strings.HasPrefix(compSpec.ComponentDef, constant.KBGeneratedVirtualCompDefPrefix) {
			return ""
		}
		return compSpec.ComponentDef
	}()
	schedulingPolicy, err := scheduling.BuildSchedulingPolicy(cluster, compSpec)
	if err != nil {
		return nil, err
	}
	compBuilder := builder.NewComponentBuilder(cluster.Namespace, compName, compDefName).
		AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
		AddAnnotations(constant.KBAppMultiClusterPlacementKey, cluster.Annotations[constant.KBAppMultiClusterPlacementKey]).
		AddLabelsInMap(constant.GetComponentWellKnownLabels(cluster.Name, compSpec.Name)).
		AddLabels(constant.KBAppClusterUIDLabelKey, string(cluster.UID)).
		SetServiceVersion(compSpec.ServiceVersion).
		SetSchedulingPolicy(schedulingPolicy).
		DisableExporter(compSpec.GetDisableExporter()).
		SetReplicas(compSpec.Replicas).
		SetResources(compSpec.Resources).
		SetServiceAccountName(compSpec.ServiceAccountName).
		SetVolumeClaimTemplates(compSpec.VolumeClaimTemplates).
		SetConfigs(compSpec.Configs).
		SetEnabledLogs(compSpec.EnabledLogs).
		SetServiceRefs(compSpec.ServiceRefs).
		SetTLSConfig(compSpec.TLS, compSpec.Issuer).
		SetInstances(compSpec.Instances).
		SetOfflineInstances(compSpec.OfflineInstances)
	if labels != nil {
		compBuilder.AddLabelsInMap(labels)
	}
	if annotations != nil {
		compBuilder.AddAnnotationsInMap(annotations)
	}
	if !IsGenerated(compBuilder.GetObject()) {
		compBuilder.SetServices(compSpec.Services)
	}
	if cluster.Spec.RuntimeClassName != nil {
		compBuilder.SetRuntimeClassName(*cluster.Spec.RuntimeClassName)
	}
	return compBuilder.GetObject(), nil
}

func BuildComponentDefinition(clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	clusterCompDef, clusterCompVer, err := getClusterCompDefAndVersion(clusterDef, clusterVer, clusterCompSpec)
	if err != nil {
		return nil, err
	}
	compDef, err := buildComponentDefinitionByConversion(clusterCompDef, clusterCompVer)
	if err != nil {
		return nil, err
	}
	return compDef, nil
}

func getOrBuildComponentDefinition(ctx context.Context, cli client.Reader,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ComponentDefinition, error) {
	if len(cluster.Spec.ClusterDefRef) > 0 && len(clusterCompSpec.ComponentDefRef) > 0 {
		return BuildComponentDefinition(clusterDef, clusterVer, clusterCompSpec)
	}
	if len(clusterCompSpec.ComponentDef) > 0 {
		compDef := &appsv1alpha1.ComponentDefinition{}
		if err := cli.Get(ctx, types.NamespacedName{Name: clusterCompSpec.ComponentDef}, compDef); err != nil {
			return nil, err
		}
		return compDef, nil
	}
	return nil, fmt.Errorf("the component definition is not provided")
}

func getClusterReferencedResources(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster) (*appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, error) {
	var (
		clusterDef *appsv1alpha1.ClusterDefinition
		clusterVer *appsv1alpha1.ClusterVersion
	)
	if len(cluster.Spec.ClusterDefRef) > 0 {
		clusterDef = &appsv1alpha1.ClusterDefinition{}
		if err := cli.Get(ctx, types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
			return nil, nil, err
		}
	}
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		clusterVer = &appsv1alpha1.ClusterVersion{}
		if err := cli.Get(ctx, types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, clusterVer); err != nil {
			return nil, nil, err
		}
	}
	if clusterDef == nil {
		if len(cluster.Spec.ClusterDefRef) == 0 {
			return nil, nil, fmt.Errorf("cluster definition is needed for generated component")
		} else {
			return nil, nil, fmt.Errorf("referenced cluster definition is not found: %s", cluster.Spec.ClusterDefRef)
		}
	}
	return clusterDef, clusterVer, nil
}

func getClusterCompDefAndVersion(clusterDef *appsv1alpha1.ClusterDefinition,
	clusterVer *appsv1alpha1.ClusterVersion,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.ClusterComponentDefinition, *appsv1alpha1.ClusterComponentVersion, error) {
	if len(clusterCompSpec.ComponentDefRef) == 0 {
		return nil, nil, fmt.Errorf("cluster component definition ref is empty: %s", clusterCompSpec.Name)
	}
	clusterCompDef := clusterDef.GetComponentDefByName(clusterCompSpec.ComponentDefRef)
	if clusterCompDef == nil {
		return nil, nil, fmt.Errorf("referenced cluster component definition is not defined: %s", clusterCompSpec.ComponentDefRef)
	}
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	if clusterVer != nil {
		clusterCompVer = clusterVer.Spec.GetDefNameMappingComponents()[clusterCompSpec.ComponentDefRef]
	}
	return clusterCompDef, clusterCompVer, nil
}

func getClusterCompSpec4Component(ctx context.Context, cli client.Reader,
	clusterDef *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component) (*appsv1alpha1.ClusterComponentSpec, error) {
	compName, err := ShortName(cluster.Name, comp.Name)
	if err != nil {
		return nil, err
	}
	compSpec, err := intctrlutil.GetOriginalOrGeneratedComponentSpecByName(ctx, cli, cluster, compName)
	if err != nil {
		return nil, err
	}
	if compSpec != nil {
		return compSpec, nil
	}
	return apiconversion.HandleSimplifiedClusterAPI(clusterDef, cluster), nil
}

func getCompLabelValue(comp *appsv1alpha1.Component, label string) (string, error) {
	if comp.Labels == nil {
		return "", fmt.Errorf("required label %s is not provided, component: %s", label, comp.GetName())
	}
	val, ok := comp.Labels[label]
	if !ok {
		return "", fmt.Errorf("required label %s is not provided, component: %s", label, comp.GetName())
	}
	return val, nil
}

// GetCompDefByName gets the component definition by component definition name.
func GetCompDefByName(ctx context.Context, cli client.Reader, compDefName string) (*appsv1alpha1.ComponentDefinition, error) {
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: compDefName}, compDef); err != nil {
		return nil, err
	}
	return compDef, nil
}

func GetCompNCompDefByName(ctx context.Context, cli client.Reader, namespace, fullCompName string) (*appsv1alpha1.Component, *appsv1alpha1.ComponentDefinition, error) {
	comp := &appsv1alpha1.Component{}
	if err := cli.Get(ctx, client.ObjectKey{Name: fullCompName, Namespace: namespace}, comp); err != nil {
		return nil, nil, err
	}
	compDef, err := GetCompDefByName(ctx, cli, comp.Spec.CompDef)
	if err != nil {
		return nil, nil, err
	}
	return comp, compDef, nil
}

// ListClusterComponents lists the components of the cluster.
func ListClusterComponents(ctx context.Context, cli client.Reader, cluster *appsv1alpha1.Cluster) ([]appsv1alpha1.Component, error) {
	compList := &appsv1alpha1.ComponentList{}
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}); err != nil {
		return nil, err
	}
	return compList.Items, nil
}

// GetClusterComponentShortNameSet gets the component short name set of the cluster.
func GetClusterComponentShortNameSet(ctx context.Context, cli client.Reader, cluster *appsv1alpha1.Cluster) (sets.Set[string], error) {
	compList, err := ListClusterComponents(ctx, cli, cluster)
	if err != nil {
		return nil, err
	}
	compSet := sets.Set[string]{}
	for _, comp := range compList {
		compShortName, err := ShortName(cluster.Name, comp.Name)
		if err != nil {
			return nil, err
		}
		compSet.Insert(compShortName)
	}
	return compSet, nil
}

func GetExporter(componentDef appsv1alpha1.ComponentDefinitionSpec) *common.Exporter {
	if componentDef.Exporter != nil {
		return &common.Exporter{Exporter: *componentDef.Exporter}
	}

	// Compatible with previous versions of kb
	if componentDef.Monitor == nil || componentDef.Monitor.Exporter == nil {
		return nil
	}

	return &common.Exporter{
		TargetPort: &componentDef.Monitor.Exporter.ScrapePort,
		Exporter: appsv1alpha1.Exporter{
			ScrapePath: componentDef.Monitor.Exporter.ScrapePath,
		},
	}
}
