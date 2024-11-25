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

package k8score

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	eventHandledAnnotationKey = "kubeblocks.io/event-handled"
)

type eventHandler interface {
	Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error
}

// EventReconciler reconciles an Event object
type EventReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// events API only allows ready-only, create, patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *EventReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("event", req.NamespacedName),
	}

	reqCtx.Log.V(1).Info("event watcher")

	event := &corev1.Event{}
	if err := r.Client.Get(ctx, req.NamespacedName, event, multicluster.InDataContextUnspecified()); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "getEventError")
	}

	if r.isEventHandled(event) {
		return intctrlutil.Reconciled()
	}

	handlers := []eventHandler{
		&instanceset.PodRoleEventHandler{},
		&component.AvailableEventHandler{},
		&component.KBAgentTaskEventHandler{},
	}
	for _, handler := range handlers {
		if err := handler.Handle(r.Client, reqCtx, r.Recorder, event); err != nil && !apierrors.IsNotFound(err) {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "handleEventError")
		}
	}

	if err := r.eventHandled(ctx, event); err != nil {
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "eventHandledError")
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *EventReconciler) SetupWithManager(mgr ctrl.Manager, multiClusterMgr multicluster.Manager) error {
	b := intctrlutil.NewControllerManagedBy(mgr).
		For(&corev1.Event{})

	if multiClusterMgr != nil {
		multiClusterMgr.Watch(b, &corev1.Event{}, &handler.EnqueueRequestForObject{})
	}

	return b.Complete(r)
}

func (r *EventReconciler) isEventHandled(event *corev1.Event) bool {
	count := fmt.Sprintf("%d", event.Count)
	annotations := event.GetAnnotations()
	if annotations != nil && annotations[eventHandledAnnotationKey] == count {
		return true
	}
	return false
}

func (r *EventReconciler) eventHandled(ctx context.Context, event *corev1.Event) error {
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string, 0)
	}
	event.Annotations[eventHandledAnnotationKey] = fmt.Sprintf("%d", event.Count)
	return r.Client.Patch(ctx, event, patch, multicluster.InDataContextUnspecified())
}
