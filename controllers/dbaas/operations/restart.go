/*
Copyright ApeCloud Inc.

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

package operations

import (
	"fmt"
	"time"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

func init() {
	restartBehaviour := &OpsBehaviour{
		FromClusterPhases:      []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase},
		ToClusterPhase:         dbaasv1alpha1.UpdatingPhase,
		Action:                 RestartAction,
		ActionStartedCondition: dbaasv1alpha1.NewRestartingCondition,
		ReconcileAction:        ReconcileActionWithComponentOps,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.RestartType, restartBehaviour)
}

// RestartAction restart components by updating StatefulSet.
func RestartAction(opsRes *OpsResource) error {
	var (
		componentNameMap = getAllComponentsNameMap(opsRes.OpsRequest)
		startTimestamp   = opsRes.OpsRequest.Status.StartTimestamp
	)
	if startTimestamp == nil {
		return fmt.Errorf("status.startTimestamp can not be null")
	}
	if err := restartDeployment(opsRes, componentNameMap); err != nil {
		return err
	}
	return restartStatefulSet(opsRes, componentNameMap)
}

// restartStatefulSet restart statefulSet workload
func restartStatefulSet(opsRes *OpsResource, componentNameMap map[string]*dbaasv1alpha1.ComponentOps) error {
	var (
		statefulSetList = &appv1.StatefulSetList{}
		err             error
	)
	if err = opsRes.Client.List(opsRes.Ctx, statefulSetList,
		client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels{AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
		return err
	}

	for _, v := range statefulSetList.Items {
		if isRestarted(opsRes, &v, componentNameMap, &v.Spec.Template) {
			continue
		}
		if err = opsRes.Client.Update(opsRes.Ctx, &v); err != nil {
			return err
		}
	}
	return nil
}

// restartDeployment restart deployment workload
func restartDeployment(opsRes *OpsResource, componentNameMap map[string]*dbaasv1alpha1.ComponentOps) error {
	var (
		deploymentList = &appv1.DeploymentList{}
		err            error
	)
	if err = opsRes.Client.List(opsRes.Ctx, deploymentList,
		client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels{AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
		return err
	}

	for _, v := range deploymentList.Items {
		if isRestarted(opsRes, &v, componentNameMap, &v.Spec.Template) {
			continue
		}
		if err = opsRes.Client.Update(opsRes.Ctx, &v); err != nil {
			return err
		}
	}
	return nil
}

// isRestarted check whether the component has been restarted
func isRestarted(opsRes *OpsResource, object client.Object, componentNameMap map[string]*dbaasv1alpha1.ComponentOps, podTemplate *corev1.PodTemplateSpec) bool {
	cName := object.GetLabels()[AppComponentNameLabelKey]
	if _, ok := componentNameMap[cName]; !ok {
		return true
	}
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = map[string]string{}
	}
	hasRestarted := true
	startTimestamp := opsRes.OpsRequest.Status.StartTimestamp
	stsRestartTimeStamp := podTemplate.Annotations[RestartAnnotationKey]
	if res, _ := time.Parse(time.RFC3339, stsRestartTimeStamp); startTimestamp.After(res) {
		podTemplate.Annotations[RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
		hasRestarted = false
	}
	return hasRestarted
}
