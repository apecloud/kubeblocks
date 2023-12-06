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

package operations

import (
	"fmt"
	"reflect"
	"time"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type restartOpsHandler struct{}

var _ OpsHandler = restartOpsHandler{}

func init() {
	restartBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may repair it.
		// TODO: we should add "force" flag for these opsRequest.
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		OpsHandler:        restartOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.RestartType, restartBehaviour)
}

// ActionStartedCondition the started condition when handle the restart request.
func (r restartOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewRestartingCondition(opsRes.OpsRequest), nil
}

// Action restarts components by updating StatefulSet.
func (r restartOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if opsRes.OpsRequest.Status.StartTimestamp.IsZero() {
		return fmt.Errorf("status.startTimestamp can not be null")
	}
	componentNameMap := opsRes.OpsRequest.Spec.GetRestartComponentNameSet()
	componentKindList := []client.ObjectList{
		&appv1.DeploymentList{},
		&appv1.StatefulSetList{},
		&workloads.ReplicatedStateMachineList{},
	}
	for _, objectList := range componentKindList {
		if err := restartComponent(reqCtx, cli, opsRes, componentNameMap, objectList); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r restartOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	return reconcileActionWithComponentOps(reqCtx, cli, opsRes, "restart", handleComponentStatusProgress)
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes for Cluster.spec.
// empty implementation here.
func (r restartOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// restartStatefulSet restarts statefulSet workload
func restartComponent(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, componentNameMap map[string]struct{}, objList client.ObjectList) error {
	if err := cli.List(reqCtx.Ctx, objList,
		client.InNamespace(opsRes.Cluster.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: opsRes.Cluster.Name}); err != nil {
		return err
	}

	items := reflect.ValueOf(objList).Elem().FieldByName("Items")
	l := items.Len()
	for i := 0; i < l; i++ {
		// get the underlying object
		object := items.Index(i).Addr().Interface().(client.Object)
		template := items.Index(i).FieldByName("Spec").FieldByName("Template").Addr().Interface().(*corev1.PodTemplateSpec)
		if isRestarted(opsRes, object, componentNameMap, template) {
			continue
		}
		if err := cli.Update(reqCtx.Ctx, object); err != nil {
			return err
		}
	}
	return nil
}

// isRestarted checks whether the component has been restarted
func isRestarted(opsRes *OpsResource, object client.Object, componentNameMap map[string]struct{}, podTemplate *corev1.PodTemplateSpec) bool {
	cName := object.GetLabels()[constant.KBAppComponentLabelKey]
	if _, ok := componentNameMap[cName]; !ok {
		return true
	}
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = map[string]string{}
	}
	hasRestarted := true
	startTimestamp := opsRes.OpsRequest.Status.StartTimestamp
	stsRestartTimeStamp := podTemplate.Annotations[constant.RestartAnnotationKey]
	if res, _ := time.Parse(time.RFC3339, stsRestartTimeStamp); startTimestamp.After(res) {
		podTemplate.Annotations[constant.RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
		hasRestarted = false
	}
	return hasRestarted
}
