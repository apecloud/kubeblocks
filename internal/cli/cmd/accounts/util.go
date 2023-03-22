/*
Copyright ApeCloud, Inc.

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

package accounts

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

func struct2Map(obj interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

type compInfo struct {
	comp       *appsv1alpha1.ClusterComponentSpec
	compStatus *appsv1alpha1.ClusterComponentStatus
	compDef    *appsv1alpha1.ClusterComponentDefinition
}

func (info *compInfo) inferPodName() (string, error) {
	if info.compStatus == nil {
		return "", fmt.Errorf("component status is missing")
	}
	if info.compStatus.Phase != appsv1alpha1.RunningPhase || !*info.compStatus.PodsReady {
		return "", fmt.Errorf("component is not ready, please try later")
	}
	if info.compStatus.ConsensusSetStatus != nil {
		return info.compStatus.ConsensusSetStatus.Leader.Pod, nil
	}
	if info.compStatus.ReplicationSetStatus != nil {
		return info.compStatus.ReplicationSetStatus.Primary.Pod, nil
	}
	return "", fmt.Errorf("cannot infer the pod to connect, please specify the pod name explicitly by `--instance` flag")
}

func fillCompInfoByName(ctx context.Context, dynamic dynamic.Interface, namespace, clusterName, componentName string) (*compInfo, error) {
	// find cluster
	obj, err := dynamic.Resource(types.ClusterGVR()).Namespace(namespace).Get(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cluster := &appsv1alpha1.Cluster{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cluster); err != nil {
		return nil, err
	}
	if cluster.Status.Phase != appsv1alpha1.RunningPhase {
		return nil, fmt.Errorf("cluster %s is not running, please try later", clusterName)
	}

	compInfo := &compInfo{}
	// fill component
	if len(componentName) == 0 {
		compInfo.comp = &cluster.Spec.ComponentSpecs[0]
	} else {
		compInfo.comp = cluster.GetComponentByName(componentName)
	}
	if compInfo.comp == nil {
		return nil, fmt.Errorf("component %s not found in cluster %s", componentName, clusterName)
	}
	// fill component status
	for name, compStatus := range cluster.Status.Components {
		if name == compInfo.comp.Name {
			compInfo.compStatus = &compStatus
			break
		}
	}
	if compInfo.compStatus == nil {
		return nil, fmt.Errorf("componentStatus %s not found in cluster %s", componentName, clusterName)
	}

	// find cluster def
	obj, err = dynamic.Resource(types.ClusterDefGVR()).Namespace(metav1.NamespaceAll).Get(ctx, cluster.Spec.ClusterDefRef, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, clusterDef); err != nil {
		return nil, err
	}
	// find component def by reference
	for _, compDef := range clusterDef.Spec.ComponentDefs {
		if compDef.Name == compInfo.comp.ComponentDefRef {
			compInfo.compDef = &compDef
			break
		}
	}
	if compInfo.compDef == nil {
		return nil, fmt.Errorf("componentDef %s not found in clusterDef %s", compInfo.comp.ComponentDefRef, clusterDef.Name)
	}
	return compInfo, nil
}
