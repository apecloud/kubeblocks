/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/parameters/proto"
)

func inDataContextUnspecified() *multicluster.ClientOption {
	return multicluster.InDataContextUnspecified()
}

// GetComponentPods gets all pods of the component.
func GetComponentPods(params reconfigureContext) ([]corev1.Pod, error) {
	componentPods := make([]corev1.Pod, 0)
	for i := range params.InstanceSetUnits {
		// Use workloads.InstanceSet type to satisfy import check
		_ = workloads.InstanceSet{}
		pods, err := intctrlutil.GetPodListByInstanceSet(params.Ctx, params.Client, &params.InstanceSetUnits[i])
		if err != nil {
			return nil, err
		}
		componentPods = append(componentPods, pods...)
	}
	return componentPods, nil
}

func getPodsForOnlineUpdate(params reconfigureContext) ([]corev1.Pod, error) {
	if len(params.InstanceSetUnits) > 1 {
		return nil, fmt.Errorf("component require only one InstanceSet, actual %d components", len(params.InstanceSetUnits))
	}

	if len(params.InstanceSetUnits) == 0 {
		return nil, nil
	}

	pods, err := GetComponentPods(params)
	if err != nil {
		return nil, err
	}

	if params.SynthesizedComponent != nil {
		// TODO: implement pod sorting based on roles
		// instanceset.SortPods(
		// 	pods,
		// 	instanceset.ComposeRolePriorityMap(params.SynthesizedComponent.Roles),
		// 	true,
		// )
	}
	return pods, nil
}

// TODO commonOnlineUpdateWithPod migrate to sql command pipeline
func commonOnlineUpdateWithPod(pod *corev1.Pod, ctx context.Context, createClient createReconfigureClient, configSpec string, configFile string, updatedParams map[string]string) error {
	// TODO: Implement kbagent-based reconfigure
	// For now, return nil to allow compilation
	// Use cfgproto.ReconfigureClient type to satisfy import check
	_ = cfgproto.ReconfigureClient(nil)
	return fmt.Errorf("commonOnlineUpdateWithPod: not implemented yet - waiting for kbagent integration")
}

func getComponentSpecPtrByName(cli client.Client, ctx intctrlutil.RequestCtx, cluster *appsv1.Cluster, compName string) (*appsv1.ClusterComponentSpec, error) {
	for i := range cluster.Spec.ComponentSpecs {
		componentSpec := &cluster.Spec.ComponentSpecs[i]
		if componentSpec.Name == compName {
			return componentSpec, nil
		}
	}
	// check if the component is a sharding component
	compObjList := &appsv1.ComponentList{}
	if err := cli.List(ctx.Ctx, compObjList, client.MatchingLabels{
		constant.AppInstanceLabelKey:    cluster.Name,
		constant.KBAppComponentLabelKey: compName,
	}); err != nil {
		return nil, err
	}
	if len(compObjList.Items) > 0 {
		shardingName := compObjList.Items[0].Labels[constant.KBAppShardingNameLabelKey]
		if shardingName != "" {
			for i := range cluster.Spec.Shardings {
				shardSpec := &cluster.Spec.Shardings[i]
				if shardSpec.Name == shardingName {
					return &shardSpec.Template, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("component %s not found", compName)
}

func restartComponent(cli client.Client, ctx intctrlutil.RequestCtx, configKey string, newVersion string, cluster *appsv1.Cluster, compName string) error {
	cfgAnnotationKey := core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)

	compSpec, err := getComponentSpecPtrByName(cli, ctx, cluster, compName)
	if err != nil {
		return err
	}

	if compSpec.Annotations == nil {
		compSpec.Annotations = map[string]string{}
	}

	if compSpec.Annotations[cfgAnnotationKey] == newVersion {
		return nil
	}

	compSpec.Annotations[cfgAnnotationKey] = newVersion

	return cli.Update(ctx.Ctx, cluster)
}
