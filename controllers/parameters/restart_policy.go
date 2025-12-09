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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

func init() {
	registerPolicy(parametersv1alpha1.RestartPolicy, restartPolicyInstance)
}

var restartPolicyInstance = &restartPolicy{}

type restartPolicy struct{}

func (s *restartPolicy) Upgrade(rctx reconfigureContext) (returnedStatus, error) {
	rctx.Log.V(1).Info("simple policy begin....")

	var (
		newVersion = rctx.getTargetVersionHash()
		configKey  = rctx.generateConfigIdentifier()
	)

	if err := s.restart(rctx.Client, rctx.RequestCtx, configKey, newVersion, rctx.Cluster, rctx.ClusterComponent.Name); err != nil {
		return makeReturnedStatus(ESFailedAndRetry), err
	}
	return syncLatestConfigStatus(rctx), nil
}

func (s *restartPolicy) restart(cli client.Client, ctx intctrlutil.RequestCtx, configKey string, newVersion string, cluster *appsv1.Cluster, compName string) error {
	cfgAnnotationKey := core.GenerateUniqKeyWithConfig(constant.UpgradeRestartAnnotationKey, configKey)

	compSpec, err := s.getComponentSpecPtrByName(cli, ctx, cluster, compName)
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

func (s *restartPolicy) getComponentSpecPtrByName(cli client.Client, ctx intctrlutil.RequestCtx, cluster *appsv1.Cluster, compName string) (*appsv1.ClusterComponentSpec, error) {
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
