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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type restartOpsHandler struct{}

var _ OpsHandler = restartOpsHandler{}

func init() {
	restartBehaviour := OpsBehaviour{
		FromClusterPhases: []dbaasv1alpha1.Phase{dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase, dbaasv1alpha1.AbnormalPhase},
		ToClusterPhase:    dbaasv1alpha1.UpdatingPhase,
		OpsHandler:        restartOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.RestartType, restartBehaviour)
}

// ActionStartedCondition the started condition when handle the restart request.
func (r restartOpsHandler) ActionStartedCondition(opsRequest *dbaasv1alpha1.OpsRequest) *metav1.Condition {
	return dbaasv1alpha1.NewRestartingCondition(opsRequest)
}

// Action restarts components by updating StatefulSet.
func (r restartOpsHandler) Action(opsRes *OpsResource) error {
	if opsRes.OpsRequest.Status.StartTimestamp.IsZero() {
		return fmt.Errorf("status.startTimestamp can not be null")
	}
	componentNameMap := opsRes.OpsRequest.GetRestartComponentNameMap()
	if err := restartDeployment(opsRes, componentNameMap); err != nil {
		return err
	}
	return restartStatefulSet(opsRes, componentNameMap)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for volume expansion opsRequest.
func (r restartOpsHandler) ReconcileAction(opsRes *OpsResource) (dbaasv1alpha1.Phase, time.Duration, error) {
	return ReconcileActionWithComponentOps(opsRes, "restart", handleComponentStatusProgress)
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (r restartOpsHandler) GetRealAffectedComponentMap(opsRequest *dbaasv1alpha1.OpsRequest) realAffectedComponentMap {
	return opsRequest.GetRestartComponentNameMap()
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes in Cluster.spec.
// empty implementation here.
func (r restartOpsHandler) SaveLastConfiguration(opsRes *OpsResource) error {
	return nil
}

// restartStatefulSet restarts statefulSet workload
func restartStatefulSet(opsRes *OpsResource, componentNameMap map[string]struct{}) error {
	var (
		statefulSetList = &appv1.StatefulSetList{}
		err             error
	)
	if err = opsRes.Client.List(opsRes.Ctx, statefulSetList,
		client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels{intctrlutil.AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
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

// restartDeployment restarts deployment workload
func restartDeployment(opsRes *OpsResource, componentNameMap map[string]struct{}) error {
	var (
		deploymentList = &appv1.DeploymentList{}
		err            error
	)
	if err = opsRes.Client.List(opsRes.Ctx, deploymentList,
		client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels{intctrlutil.AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
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

// isRestarted checks whether the component has been restarted
func isRestarted(opsRes *OpsResource, object client.Object, componentNameMap map[string]struct{}, podTemplate *corev1.PodTemplateSpec) bool {
	cName := object.GetLabels()[intctrlutil.AppComponentLabelKey]
	if _, ok := componentNameMap[cName]; !ok {
		return true
	}
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = map[string]string{}
	}
	hasRestarted := true
	startTimestamp := opsRes.OpsRequest.Status.StartTimestamp
	stsRestartTimeStamp := podTemplate.Annotations[intctrlutil.RestartAnnotationKey]
	if res, _ := time.Parse(time.RFC3339, stsRestartTimeStamp); startTimestamp.After(res) {
		podTemplate.Annotations[intctrlutil.RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
		hasRestarted = false
	}
	return hasRestarted
}
