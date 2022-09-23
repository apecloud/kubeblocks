/*
Copyright 2022.

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

package k8score

import (
	"context"
	"encoding/json"
	"k8s.io/apimachinery/pkg/types"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	consensusSetRoleLabelKey = "cs.dbaas.apecloud.com/role"
)

// EventReconciler reconciles a AppVersion object
type EventReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// NOTES: controller-gen RBAC marker is maintained at rbac.go

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *EventReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("event", req.NamespacedName),
	}

	reqCtx.Log.V(1).Info("event watcher")

	event := &corev1.Event{}
	if err := r.Client.Get(ctx, req.NamespacedName, event); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "getEventError")
	}

	switch event.InvolvedObject.FieldPath {
	case "spec.containers.KBProbeRoleChangedCheck":
		err := r.handleRoleChangedEvent(ctx, event)
		if err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "handleRoleChangedEventError")
		}
	}

	return ctrl.Result{}, nil
}

// TODO probeMessage should be defined by @xuanchi
type probeMessage struct {
	code string
	data probeMessageData
}

type probeMessageData struct {
	role    string
	message string
}

func (r *EventReconciler) handleRoleChangedEvent(ctx context.Context, event *corev1.Event) error {
	// get role
	message := &probeMessage{}
	err := json.Unmarshal([]byte(event.Message), message)
	if err != nil {
		return err
	}
	role := strings.ToLower(message.data.role)

	// get pod
	pod := &corev1.Pod{}
	podName := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	if err := r.Client.Get(ctx, podName, pod); err != nil {
		return err
	}

	// update label
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[consensusSetRoleLabelKey] = role
	err = r.Client.Patch(ctx, pod, patch)
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Event{}).
		Complete(r)
}
