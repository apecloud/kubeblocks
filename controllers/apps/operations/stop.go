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

package operations

import (
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type StopOpsHandler struct{}

var _ OpsHandler = StopOpsHandler{}

func init() {
	stopBehaviour := OpsBehaviour{
		FromClusterPhases: append(appsv1alpha1.GetClusterUpRunningPhases(), appsv1alpha1.UpdatingClusterPhase),
		ToClusterPhase:    appsv1alpha1.StoppingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        StopOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.StopType, stopBehaviour)
}

// ActionStartedCondition the started condition when handling the stop request.
func (stop StopOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewStopCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (stop StopOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		componentReplicasMap = map[string]int32{}
		cluster              = opsRes.Cluster
	)
	if _, ok := cluster.Annotations[constant.SnapShotForStartAnnotationKey]; ok {
		return nil
	}
	setReplicas := func(compSpec *appsv1alpha1.ClusterComponentSpec, componentName string) {
		compKey := getComponentKeyForStartSnapshot(componentName, "")
		componentReplicasMap[compKey] = compSpec.Replicas
		expectReplicas := int32(0)
		compSpec.Replicas = expectReplicas
		for i := range compSpec.Instances {
			compKey = getComponentKeyForStartSnapshot(componentName, compSpec.Instances[i].Name)
			componentReplicasMap[compKey] = intctrlutil.TemplateReplicas(compSpec.Instances[i])
			compSpec.Instances[i].Replicas = &expectReplicas
		}
	}
	for i := range cluster.Spec.ComponentSpecs {
		compSpec := &cluster.Spec.ComponentSpecs[i]
		setReplicas(compSpec, compSpec.Name)
	}
	for i, v := range cluster.Spec.ShardingSpecs {
		setReplicas(&cluster.Spec.ShardingSpecs[i].Template, v.Name)
	}
	componentReplicasSnapshot, err := json.Marshal(componentReplicasMap)
	if err != nil {
		return err
	}
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	// record the replicas snapshot of components to the annotations of cluster before stopping the cluster.
	cluster.Annotations[constant.SnapShotForStartAnnotationKey] = string(componentReplicasSnapshot)
	return cli.Update(reqCtx.Ctx, cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for stop opsRequest.
func (stop StopOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	getExpectReplicas := func(opsRequest *appsv1alpha1.OpsRequest, compOps ComponentOpsInteface) *int32 {
		return pointer.Int32(0)
	}
	handleComponentProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		expectProgressCount, completedCount, err := handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus, getExpectReplicas)
		if err != nil {
			return expectProgressCount, completedCount, err
		}
		return expectProgressCount, completedCount, nil
	}
	compOpsHelper := newComponentOpsHelper([]appsv1alpha1.ComponentOps{})
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "stop", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (stop StopOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	saveLastConfigurationForStopAndStart(opsRes)
	return nil
}

func getComponentKeyForStartSnapshot(compName, templateName string) string {
	if templateName != "" {
		return fmt.Sprintf("%s.%s", compName, templateName)
	}
	return compName
}

func saveLastConfigurationForStopAndStart(opsRes *OpsResource) {
	getLastComponentConfiguration := func(compSpec appsv1alpha1.ClusterComponentSpec) appsv1alpha1.LastComponentConfiguration {
		var instances []appsv1alpha1.InstanceTemplate
		for _, v := range compSpec.Instances {
			instances = append(instances, appsv1alpha1.InstanceTemplate{
				Name:     v.Name,
				Replicas: v.Replicas,
			})
		}
		return appsv1alpha1.LastComponentConfiguration{
			Replicas:  pointer.Int32(compSpec.Replicas),
			Instances: instances,
		}
	}
	lastConfiguration := &opsRes.OpsRequest.Status.LastConfiguration
	lastConfiguration.Components = map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		lastConfiguration.Components[v.Name] = getLastComponentConfiguration(v)
	}
	for _, v := range opsRes.Cluster.Spec.ShardingSpecs {
		lastConfiguration.Components[v.Name] = getLastComponentConfiguration(v.Template)
	}
}
