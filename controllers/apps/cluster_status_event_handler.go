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

package apps

import (
	"context"
	"regexp"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/k8score"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	probeutil "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

// EventTimeOut timeout of the event
const EventTimeOut = 30 * time.Second

// ClusterStatusEventHandler is the event handler for the cluster status event
type ClusterStatusEventHandler struct{}

var _ k8score.EventHandler = &ClusterStatusEventHandler{}

func init() {
	k8score.EventHandlerMap["cluster-status-handler"] = &ClusterStatusEventHandler{}
}

// Handle is the event handler for the cluster status event.
func (r *ClusterStatusEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.Reason != string(probeutil.CheckRoleOperation) {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}

	// parse probe event message when field path is probe-role-changed-check
	message := k8score.ParseProbeEventMessage(reqCtx, event)
	if message == nil {
		reqCtx.Log.Info("parse probe event message failed", "message", event.Message)
		return nil
	}

	// if probe message event is checkRoleFailed, it means the cluster is abnormal, need to handle the cluster status
	if message.Event == probeutil.OperationFailed {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}
	return nil
}

// TODO: Unified cluster event processing
// handleEventForClusterStatus handles event for cluster Warning and Failed phase
func handleEventForClusterStatus(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {

	type predicateProcessor struct {
		pred      func() bool
		processor func() error
	}

	nilReturnHandler := func() error { return nil }

	pps := []predicateProcessor{
		{
			// handle cronjob complete or fail event
			pred: func() bool {
				return event.InvolvedObject.Kind == constant.CronJobKind &&
					event.Reason == "SawCompletedJob"
			},
			processor: func() error {
				return handleDeletePVCCronJobEvent(ctx, cli, recorder, event)
			},
		},
		{
			pred: func() bool {
				return event.Type != corev1.EventTypeWarning ||
					!isTargetKindForEvent(event)
			},
			processor: nilReturnHandler,
		},
		{
			pred: func() bool {
				// the error repeated several times, so we can sure it's a real error to the cluster.
				return !k8score.IsOvertimeEvent(event, EventTimeOut)
			},
			processor: nilReturnHandler,
		},
		{
			// handle cluster workload error events such as pod/statefulset/deployment errors
			// must be the last one
			pred: func() bool {
				return true
			},
			processor: func() error {
				return handleClusterStatusByEvent(ctx, cli, recorder, event)
			},
		},
	}

	for _, pp := range pps {
		if pp.pred() {
			return pp.processor()
		}
	}
	return nil
}

func handleDeletePVCCronJobEvent(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {
	re := regexp.MustCompile("status: Failed")
	var (
		err    error
		object client.Object
	)
	matches := re.FindStringSubmatch(event.Message)
	if len(matches) == 0 {
		// TODO(impl): introduce a one-shot delayed job to delete the pvc object.
		// delete pvc success, then delete cronjob
		return checkedDeleteDeletePVCCronJob(ctx, cli, event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	}
	// cronjob failed
	if object, err = getEventInvolvedObject(ctx, cli, event); err != nil {
		return err
	}
	return notifyClusterStatusChange(ctx, cli, recorder, object, event)
}

func checkedDeleteDeletePVCCronJob(ctx context.Context, cli client.Client, name string, namespace string) error {
	// label check
	cronJob := batchv1.CronJob{}
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	if cronJob.ObjectMeta.Labels[constant.AppManagedByLabelKey] != constant.AppName {
		return nil
	}
	// check the delete-pvc-cronjob annotation.
	// the reason for this is that the backup policy also creates cronjobs,
	// which need to be distinguished by the annotation.
	if cronJob.ObjectMeta.Annotations[lifecycleAnnotationKey] != lifecycleDeletePVCAnnotation {
		return nil
	}
	// if managed by kubeblocks, then it must be the cronjob used to delete pvc, delete it since it's completed
	if err := cli.Delete(ctx, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

// handleClusterStatusByEvent handles the cluster status when warning event happened
func handleClusterStatusByEvent(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {
	object, err := getEventInvolvedObject(ctx, cli, event)
	if err != nil {
		return err
	}
	return notifyClusterStatusChange(ctx, cli, recorder, object, event)
}

// getEventInvolvedObject gets event involved object for StatefulSet/Deployment/Pod workload
func getEventInvolvedObject(ctx context.Context, cli client.Client, event *corev1.Event) (client.Object, error) {
	objectKey := client.ObjectKey{
		Name:      event.InvolvedObject.Name,
		Namespace: event.InvolvedObject.Namespace,
	}
	var err error
	// If client.object interface object is used as a parameter, it will not return an error when the object is not found.
	// so we should specify the object type to get the object.
	switch event.InvolvedObject.Kind {
	case constant.PodKind:
		pod := &corev1.Pod{}
		err = cli.Get(ctx, objectKey, pod)
		return pod, err
	case constant.StatefulSetKind:
		sts := &appsv1.StatefulSet{}
		err = cli.Get(ctx, objectKey, sts)
		return sts, err
	case constant.DeploymentKind:
		deployment := &appsv1.Deployment{}
		err = cli.Get(ctx, objectKey, deployment)
		return deployment, err
	}
	return nil, err
}

// isTargetKindForEvent checks the event involve object is the target resources
func isTargetKindForEvent(event *corev1.Event) bool {
	return slices.Index([]string{constant.PodKind, constant.DeploymentKind, constant.StatefulSetKind}, event.InvolvedObject.Kind) != -1
}
