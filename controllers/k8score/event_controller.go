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

package k8score

import (
	"context"
	"encoding/json"
	"regexp"
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
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensusset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Event object against the actual cluster state, and then
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
	if event.InvolvedObject.FieldPath != component.ProbeRoleChangedCheckPath {
		return nil
	}
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
		return role, consensusset.UpdateConsensusSetRoleLabel(cli, reqCtx, componentDef, pod, role)
	case appsv1alpha1.Replication:
		return role, replicationset.HandleReplicationSetRoleChangeEvent(cli, reqCtx, cluster, compName, pod, role)
	}
	return role, nil
}

// ParseProbeEventMessage parses probe event message.
func ParseProbeEventMessage(reqCtx intctrlutil.RequestCtx, event *corev1.Event) *ProbeMessage {
	message := &ProbeMessage{}
	re := regexp.MustCompile(`Readiness probe failed: ({.*})`)
	matches := re.FindStringSubmatch(event.Message)
	if len(matches) != 2 {
		reqCtx.Log.Info("parser Readiness probe event message failed", "message", event.Message)
		return nil
	}
	msg := matches[1]
	err := json.Unmarshal([]byte(msg), message)
	if err != nil {
		// not role related message, ignore it
		reqCtx.Log.Info("not role message", "message", event.Message, "error", err)
		return nil
	}
	return message
}
