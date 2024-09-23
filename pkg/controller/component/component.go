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

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/scheduling"
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

func GetClusterName(comp *appsv1.Component) (string, error) {
	return getCompLabelValue(comp, constant.AppInstanceLabelKey)
}

func GetClusterUID(comp *appsv1.Component) (string, error) {
	return getCompLabelValue(comp, constant.KBAppClusterUIDLabelKey)
}

// BuildComponent builds a new Component object from cluster component spec and definition.
func BuildComponent(cluster *appsv1.Cluster, compSpec *appsv1.ClusterComponentSpec,
	labels, annotations map[string]string) (*appsv1.Component, error) {
	schedulingPolicy, err := scheduling.BuildSchedulingPolicy(cluster, compSpec)
	if err != nil {
		return nil, err
	}
	compBuilder := builder.NewComponentBuilder(cluster.Namespace, FullName(cluster.Name, compSpec.Name), compSpec.ComponentDef).
		AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
		AddLabelsInMap(constant.GetComponentWellKnownLabels(cluster.Name, compSpec.Name)).
		AddLabels(constant.KBAppClusterUIDLabelKey, string(cluster.UID)).
		SetServiceVersion(compSpec.ServiceVersion).
		SetLabels(compSpec.Labels).
		SetAnnotations(compSpec.Annotations).
		SetEnv(compSpec.Env).
		SetSchedulingPolicy(schedulingPolicy).
		SetDisableExporter(compSpec.DisableExporter).
		SetReplicas(compSpec.Replicas).
		SetResources(compSpec.Resources).
		SetServiceAccountName(compSpec.ServiceAccountName).
		SetParallelPodManagementConcurrency(compSpec.ParallelPodManagementConcurrency).
		SetPodUpdatePolicy(compSpec.PodUpdatePolicy).
		SetVolumeClaimTemplates(compSpec.VolumeClaimTemplates).
		SetVolumes(compSpec.Volumes).
		SetServices(compSpec.Services).
		SetConfigs(compSpec.Configs).
		SetServiceRefs(compSpec.ServiceRefs).
		SetTLSConfig(compSpec.TLS, compSpec.Issuer).
		SetInstances(compSpec.Instances).
		SetOfflineInstances(compSpec.OfflineInstances).
		SetRuntimeClassName(cluster.Spec.RuntimeClassName).
		SetSystemAccounts(compSpec.SystemAccounts).
		SetStop(compSpec.Stop)
	if labels != nil {
		compBuilder.AddLabelsInMap(labels)
	}
	if annotations != nil {
		compBuilder.AddAnnotationsInMap(annotations)
	}
	if cluster.Annotations != nil {
		p, ok := cluster.Annotations[constant.KBAppMultiClusterPlacementKey]
		if ok {
			compBuilder.AddAnnotations(constant.KBAppMultiClusterPlacementKey, p)
		}
	}
	return compBuilder.GetObject(), nil
}

func getCompLabelValue(comp *appsv1.Component, label string) (string, error) {
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
func GetCompDefByName(ctx context.Context, cli client.Reader, compDefName string) (*appsv1.ComponentDefinition, error) {
	compDef := &appsv1.ComponentDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: compDefName}, compDef); err != nil {
		return nil, err
	}
	return compDef, nil
}

func GetComponentByName(ctx context.Context, cli client.Reader, namespace, fullCompName string) (*appsv1.Component, error) {
	comp := &appsv1.Component{}
	if err := cli.Get(ctx, client.ObjectKey{Name: fullCompName, Namespace: namespace}, comp); err != nil {
		return nil, err
	}
	return comp, nil
}

func GetCompNCompDefByName(ctx context.Context, cli client.Reader, namespace, fullCompName string) (*appsv1.Component, *appsv1.ComponentDefinition, error) {
	comp, err := GetComponentByName(ctx, cli, namespace, fullCompName)
	if err != nil {
		return nil, nil, err
	}
	compDef, err := GetCompDefByName(ctx, cli, comp.Spec.CompDef)
	if err != nil {
		return nil, nil, err
	}
	return comp, compDef, nil
}

// ListClusterComponents lists the components of the cluster.
func ListClusterComponents(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster) ([]appsv1.Component, error) {
	compList := &appsv1.ComponentList{}
	if err := cli.List(ctx, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels{constant.AppInstanceLabelKey: cluster.Name}); err != nil {
		return nil, err
	}
	return compList.Items, nil
}

// GetClusterComponentShortNameSet gets the component short name set of the cluster.
func GetClusterComponentShortNameSet(ctx context.Context, cli client.Reader, cluster *appsv1.Cluster) (sets.Set[string], error) {
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

func GetExporter(componentDef appsv1.ComponentDefinitionSpec) *common.Exporter {
	if componentDef.Exporter != nil {
		return &common.Exporter{Exporter: *componentDef.Exporter}
	}
	return nil
}
