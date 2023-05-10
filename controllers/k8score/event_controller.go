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

package k8score

import (
	"context"
	"encoding/json"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	probeutil "github.com/apecloud/kubeblocks/cmd/probe/util"
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensus"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replication"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type EventHandler interface {
	Handle(client.Client, intctrlutil.RequestCtx, record.EventRecorder, *corev1.Event) error
}

// RoleChangeEventHandler is the event handler for the role change event
type RoleChangeEventHandler struct{}

// ProbeEventType defines the type of probe event.
type ProbeEventType string

type ProbeMessage struct {
	Event        ProbeEventType `json:"event,omitempty"`
	Message      string         `json:"message,omitempty"`
	OriginalRole string         `json:"originalRole,omitempty"`
	Role         string         `json:"role,omitempty"`
}

// EventReconciler reconciles an Event object
type EventReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// events API only allows ready-only, create, patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch

var EventHandlerMap = map[string]EventHandler{}

var _ EventHandler = &RoleChangeEventHandler{}

func init() {
	EventHandlerMap["role-change-handler"] = &RoleChangeEventHandler{}
}

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
	if err := r.Client.Get(ctx, req.NamespacedName, event); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "getEventError")
	}

	for _, handler := range EventHandlerMap {
		// ignores the not found error.
		if err := handler.Handle(r.Client, reqCtx, r.Recorder, event); err != nil && !apierrors.IsNotFound(err) {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "handleEventError")
		}
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *EventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Event{}).
		Complete(r)
}

// Handle handles role changed event.
func (r *RoleChangeEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.Reason != string(probeutil.CheckRoleOperation) {
		return nil
	}
	reqCtx.Log.Info("process event: %v", event)
	var (
		err         error
		annotations = event.GetAnnotations()
	)
	// filter role changed event that has been handled
	if annotations != nil && annotations[roleChangedAnnotKey] == trueStr {
		return nil
	}

	if _, err = handleRoleChangedEvent(cli, reqCtx, recorder, event); err != nil {
		return err
	}

	// event order is crucial in role probing, but it's not guaranteed when controller restarted, so we have to mark them to be filtered
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string, 0)
	}
	event.Annotations[roleChangedAnnotKey] = trueStr
	return cli.Patch(reqCtx.Ctx, event, patch)
}

// handleRoleChangedEvent handles role changed event and return role.
func handleRoleChangedEvent(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) (string, error) {
	// parse probe event message
	message := ParseProbeEventMessage(reqCtx, event)
	if message == nil {
		reqCtx.Log.Info("parse probe event message failed", "message", event.Message)
		return "", nil
	}

	// if probe event operation is not impl, check role failed or role invalid, ignore it
	if message.Event == ProbeEventOperationNotImpl || message.Event == ProbeEventCheckRoleFailed || message.Event == ProbeEventRoleInvalid {
		reqCtx.Log.Info("probe event failed", "message", message.Message)
		return "", nil
	}
	role := strings.ToLower(message.Role)

	podName := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	// get pod
	pod := &corev1.Pod{}
	if err := cli.Get(reqCtx.Ctx, podName, pod); err != nil {
		return role, err
	}
	// event belongs to old pod with the same name, ignore it
	if pod.UID != event.InvolvedObject.UID {
		return role, nil
	}

	// get cluster obj of the pod
	cluster := &appsv1alpha1.Cluster{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Labels[constant.AppInstanceLabelKey],
	}, cluster); err != nil {
		return role, err
	}
	reqCtx.Log.V(1).Info("handle role change event", "cluster", cluster.Name, "pod", pod.Name, "role", role, "originalRole", message.OriginalRole)
	compName, componentDef, err := componentutil.GetComponentInfoByPod(reqCtx.Ctx, cli, *cluster, pod)
	if err != nil {
		return role, err
	}
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return role, consensus.UpdateConsensusSetRoleLabel(cli, reqCtx, componentDef, pod, role)
	case appsv1alpha1.Replication:
		return role, replication.HandleReplicationSetRoleChangeEvent(cli, reqCtx, cluster, compName, pod, role)
	}
	return role, nil
}

// ParseProbeEventMessage parses probe event message.
func ParseProbeEventMessage(reqCtx intctrlutil.RequestCtx, event *corev1.Event) *ProbeMessage {
	message := &ProbeMessage{}
	err := json.Unmarshal([]byte(event.Message), message)
	if err != nil {
		// not role related message, ignore it
		reqCtx.Log.Info("not role message", "message", event.Message, "error", err)
		return nil
	}
	return message
}
