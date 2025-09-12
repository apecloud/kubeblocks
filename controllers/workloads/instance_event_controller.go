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

package workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	eventHandledAnnotationKey = "kubeblocks.io/event-handled"
)

type InstanceEventReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// events API only allows ready-only, create, patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch

func (r *InstanceEventReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("event", req.NamespacedName)

	event := &corev1.Event{}
	if err := r.Client.Get(ctx, req.NamespacedName, event); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, logger, "getEventError")
	}

	if r.isEventHandled(event) || !r.isRoleProbeEvent(event) {
		return intctrlutil.Reconciled()
	}

	if err := r.handleRoleChangedEvent(ctx, logger, event); err != nil {
		return intctrlutil.RequeueWithError(err, logger, "handleRoleChangedEventError")
	}

	if err := r.markEventHandled(ctx, event); err != nil {
		return intctrlutil.RequeueWithError(err, logger, "markEventHandledError")
	}
	return intctrlutil.Reconciled()
}

func (r *InstanceEventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&corev1.Event{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers) / 4,
		}).
		Complete(r)
}

func (r *InstanceEventReconciler) isEventHandled(event *corev1.Event) bool {
	count := fmt.Sprintf("%d", event.Count)
	annotations := event.GetAnnotations()
	if annotations != nil && annotations[eventHandledAnnotationKey] == count {
		return true
	}
	return false
}

func (r *InstanceEventReconciler) markEventHandled(ctx context.Context, event *corev1.Event) error {
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string, 0)
	}
	event.Annotations[eventHandledAnnotationKey] = fmt.Sprintf("%d", event.Count)
	return r.Client.Patch(ctx, event, patch)
}

func (r *InstanceEventReconciler) isRoleProbeEvent(event *corev1.Event) bool {
	return event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath &&
		event.ReportingController == proto.ProbeEventReportingController && event.Reason == "roleProbe"
}

func (r *InstanceEventReconciler) handleRoleChangedEvent(ctx context.Context, logger logr.Logger, event *corev1.Event) error {
	probeEvent := &proto.ProbeEvent{}
	if err := json.Unmarshal([]byte(event.Message), probeEvent); err != nil {
		logger.Error(err, "unmarshal role probe event failed")
		return nil
	}

	if probeEvent.Code != 0 {
		logger.Info("role probe failed", "message", probeEvent.Message)
		return nil
	}

	pod := &corev1.Pod{}
	podKey := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	if err := r.Client.Get(ctx, podKey, pod); err != nil {
		return err
	}
	// event belongs to the old pod with the same name, ignore it
	if strings.Compare(string(pod.UID), string(event.InvolvedObject.UID)) != 0 {
		logger.Info("stale role probe event received, ignore it")
		return nil
	}

	role := strings.ToLower(string(probeEvent.Output))
	logger.Info("handle role change event", "pod", pod.Name, "role", role)
	return r.updatePodRoleLabel(ctx, pod, role)
}

func (r *InstanceEventReconciler) updatePodRoleLabel(ctx context.Context, pod *corev1.Pod, roleName string) error {
	newPod := pod.DeepCopy()
	if len(roleName) == 0 {
		delete(newPod.Labels, constant.RoleLabelKey)
	} else {
		newPod.Labels[constant.RoleLabelKey] = roleName
	}
	if reflect.DeepEqual(newPod.Labels, pod.Labels) {
		return nil
	}
	return r.Client.Update(ctx, newPod)
}
