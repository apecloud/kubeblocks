/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
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

	parsed := parseRoleProbeOutput(probeEvent.Output)
	lastAnnotation := ""
	if pod.Annotations != nil {
		lastAnnotation = pod.Annotations[constant.LastRoleEventVersionAnnotationKey]
	}
	decision, newAnnotation := checkRoleProbeStale(parsed, lastAnnotation, event.EventTime.UnixMicro())
	switch decision {
	case roleProbeGateRejectStale:
		logger.Info("stale role probe event rejected by version gate",
			"pod", pod.Name, "role", parsed.role, "mode", parsed.mode,
			"newVersion", parsed.version, "lastAnnotation", lastAnnotation)
		return nil
	case roleProbeGateRejectMalformed:
		logger.Info("malformed kb-role-version line rejected; addon attempted new contract but emitted an unparseable trailer",
			"pod", pod.Name, "role", parsed.role, "rawOutput", string(probeEvent.Output))
		return nil
	}
	logger.Info("handle role change event",
		"pod", pod.Name, "role", parsed.role, "mode", parsed.mode, "version", parsed.version)
	if err := r.updatePodRoleLabel(ctx, pod, parsed.role, newAnnotation); err != nil {
		return err
	}
	return r.cleanupExclusiveRolePeers(ctx, logger, pod, parsed, event.EventTime.UnixMicro())
}

// cleanupExclusiveRolePeers removes the role label from any other Pod in the
// same InstanceSet that still carries an exclusive role label this event just
// claimed. The peer cleanup honours the same engine-version staleness gate as
// the primary update: a peer whose annotation already records a newer engine
// version (or, in the legacy path, a later EventTime) is left alone, so a
// stale primary event cannot strip the label from a freshly-promoted peer
// that has already advanced past it.
//
// `eventTimeMicros` is the source event's EventTime in micros; the legacy
// path uses it as the comparison key against the peer's bare-EventTime
// annotation. Passing it through (instead of a hard-coded 0) keeps the
// version state consistent across the primary update and the peer cleanup.
func (r *InstanceEventReconciler) cleanupExclusiveRolePeers(ctx context.Context, logger logr.Logger, newPod *corev1.Pod, parsed roleProbeOutput, eventTimeMicros int64) error {
	if parsed.role == "" {
		return nil
	}
	itsName, ok := newPod.Labels[instanceset.WorkloadsInstanceLabelKey]
	if !ok || itsName == "" {
		return nil
	}
	its := &workloadsv1.InstanceSet{}
	itsKey := types.NamespacedName{Namespace: newPod.Namespace, Name: itsName}
	if err := r.Client.Get(ctx, itsKey, its); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	exclusive := false
	for _, role := range its.Spec.Roles {
		if strings.EqualFold(role.Name, parsed.role) && role.IsExclusive {
			exclusive = true
			break
		}
	}
	if !exclusive {
		return nil
	}

	labels := instanceset.GetMatchLabels(its.Name)
	labels[constant.RoleLabelKey] = parsed.role
	var pods corev1.PodList
	if err := r.Client.List(ctx, &pods, client.InNamespace(its.Namespace), client.MatchingLabels(labels)); err != nil {
		return err
	}
	var errs []error
	for i := range pods.Items {
		peer := &pods.Items[i]
		if peer.Name == newPod.Name {
			continue
		}
		lastPeerAnnotation := ""
		if peer.Annotations != nil {
			lastPeerAnnotation = peer.Annotations[constant.LastRoleEventVersionAnnotationKey]
		}
		// Reuse the same gate to decide whether the new event is newer than
		// what the peer already recorded; the returned annotation value is
		// intentionally discarded here (see stripExclusiveRoleLabel below).
		// Legacy events rely on eventTimeMicros to do the comparison; engine
		// events use parsed.version internally and ignore EventTime.
		decision, _ := checkRoleProbeStale(parsed, lastPeerAnnotation, eventTimeMicros)
		if decision != roleProbeGateAccept {
			logger.Info("skip exclusive role label cleanup; peer annotation is not older than this event",
				"newPod", newPod.Name, "peer", peer.Name, "lastPeerAnnotation", lastPeerAnnotation)
			continue
		}
		if err := r.stripExclusiveRoleLabel(ctx, peer); err != nil {
			errs = append(errs, err)
			continue
		}
		logger.Info("removed exclusive role label from peer",
			"newPod", newPod.Name, "peer", peer.Name, "role", parsed.role)
	}
	return errors.Join(errs...)
}

// stripExclusiveRoleLabel deletes the exclusive role label from a peer but
// deliberately leaves LastRoleEventVersionAnnotationKey untouched.
//
// That annotation represents the last roleProbe event the peer's *own*
// kbagent emitted. Stamping it with the new primary's engine version (which
// did not originate from this peer's kbagent) would let the strict-newer
// gate later reject a legitimate event from the peer at the same engine
// epoch — for example after failover the demoted pod's next event is
// `secondary <same-epoch>`, and the gate would treat it as stale because
// the peer's annotation was already advanced by the cleanup step.
//
// Strict-newer is intentionally preserved on the gate itself; loosening it
// to "equal version, different role" would re-open the silent same-epoch
// stale-primary write-back the gate is supposed to close.
func (r *InstanceEventReconciler) stripExclusiveRoleLabel(ctx context.Context, peer *corev1.Pod) error {
	if _, has := peer.Labels[constant.RoleLabelKey]; !has {
		return nil
	}
	newPeer := peer.DeepCopy()
	delete(newPeer.Labels, constant.RoleLabelKey)
	return r.Client.Update(ctx, newPeer)
}

func (r *InstanceEventReconciler) updatePodRoleLabel(ctx context.Context, pod *corev1.Pod, roleName, newAnnotation string) error {
	newPod := pod.DeepCopy()
	if len(roleName) == 0 {
		delete(newPod.Labels, constant.RoleLabelKey)
	} else {
		if newPod.Labels == nil {
			newPod.Labels = make(map[string]string)
		}
		newPod.Labels[constant.RoleLabelKey] = roleName
	}
	if newAnnotation != "" {
		if newPod.Annotations == nil {
			newPod.Annotations = make(map[string]string)
		}
		newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = newAnnotation
	}
	if reflect.DeepEqual(newPod.Labels, pod.Labels) && reflect.DeepEqual(newPod.Annotations, pod.Annotations) {
		return nil
	}
	return r.Client.Update(ctx, newPod)
}
