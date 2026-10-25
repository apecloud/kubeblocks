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

package restore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dp "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// EngineRecoveryCheckAnnotation marks a failure-only postReady record. It must
// never execute ordinary postReady actions before the component is ready.
const EngineRecoveryCheckAnnotation = "dataprotection.kubeblocks.io/engine-recovery-check"

type oracleFailureMessage struct {
	Schema     string `json:"schema"`
	Reason     string `json:"reason"`
	Code       string `json:"code"`
	TargetTime string `json:"targetTime"`
}

// OracleEngineFailure consumes the addon's versioned termination contract, not
// free-form RMAN output. A failure belongs to this recovery only when workload
// ownership, backup, target time and completed data preparation all agree.
func OracleEngineFailure(ctx context.Context, reader client.Reader, restore *dp.Restore) (string, error) {
	if restore.Spec.ReadyConfig == nil || !restore.DeletionTimestamp.IsZero() {
		return "", nil
	}
	target, err := time.Parse(time.RFC3339, restore.Spec.RestoreTime)
	if err != nil {
		return "", nil
	}
	owner := metav1.GetControllerOf(restore)
	if !ownerKind(owner, appsv1.GroupVersion.Group, "Component") {
		return "", nil
	}
	comp := &appsv1.Component{}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: restore.Namespace, Name: owner.Name}, comp); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if comp.UID != owner.UID || !comp.DeletionTimestamp.IsZero() || comp.Annotations[constant.RestoreDoneAnnotationKey] == "true" {
		return "", nil
	}
	// Bind to the active Cluster restore request as well as the Component UID.
	co := metav1.GetControllerOf(comp)
	if !ownerKind(co, appsv1.GroupVersion.Group, "Cluster") {
		return "", nil
	}
	cluster := &appsv1.Cluster{}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: comp.Namespace, Name: co.Name}, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if cluster.UID != co.UID || !cluster.DeletionTimestamp.IsZero() {
		return "", nil
	}
	var requests map[string]map[string]string
	if json.Unmarshal([]byte(cluster.Annotations[constant.RestoreFromBackupAnnotationKey]), &requests) != nil {
		return "", nil
	}
	compName := comp.Labels[constant.KBAppShardingNameLabelKey]
	if compName == "" {
		compName = strings.TrimPrefix(comp.Name, cluster.Name+"-")
	}
	req := requests[compName]
	rt, err := time.Parse(time.RFC3339, req[constant.RestoreTimeKeyForRestore])
	ns := req[constant.BackupNamespaceKeyForRestore]
	if ns == "" {
		ns = cluster.Namespace
	}
	rn := restore.Spec.Backup.Namespace
	if rn == "" {
		rn = restore.Namespace
	}
	if err != nil || !rt.Equal(target) || req[constant.BackupNameKeyForRestore] != restore.Spec.Backup.Name || ns != rn {
		return "", nil
	}
	prepares := &dp.RestoreList{}
	if err := reader.List(ctx, prepares, client.InNamespace(restore.Namespace)); err != nil {
		return "", err
	}
	var prepared time.Time
	for i := range prepares.Items {
		p := &prepares.Items[i]
		if !ownedBy(p, appsv1.GroupVersion.Group, "Component", comp.UID) || p.Spec.PrepareDataConfig == nil || !sameBackup(p, restore) {
			continue
		}
		pt, e := time.Parse(time.RFC3339, p.Spec.RestoreTime)
		if e != nil || !pt.Equal(target) {
			continue
		}
		if !p.DeletionTimestamp.IsZero() || p.Status.Phase != dp.RestorePhaseCompleted || p.Status.CompletionTimestamp == nil {
			return "", nil
		}
		if p.Status.CompletionTimestamp.Time.After(prepared) {
			prepared = p.Status.CompletionTimestamp.Time
		}
	}
	if prepared.IsZero() {
		return "", nil
	}
	pods := &corev1.PodList{}
	if err := reader.List(ctx, pods, client.InNamespace(restore.Namespace)); err != nil {
		return "", err
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		po := metav1.GetControllerOf(pod)
		if !pod.DeletionTimestamp.IsZero() || !ownerKind(po, workloads.GroupVersion.Group, "InstanceSet") {
			continue
		}
		its := &workloads.InstanceSet{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: po.Name}, its); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return "", err
		}
		if its.UID != po.UID || !its.DeletionTimestamp.IsZero() || !ownedBy(its, appsv1.GroupVersion.Group, "Component", comp.UID) {
			continue
		}
		for _, c := range pod.Status.ContainerStatuses {
			if c.Name != "oracle" || c.Ready {
				continue
			}
			for _, terminated := range []*corev1.ContainerStateTerminated{c.State.Terminated, c.LastTerminationState.Terminated} {
				if terminated == nil || terminated.ExitCode == 0 || terminated.FinishedAt.IsZero() || terminated.FinishedAt.Time.Before(prepared) || len(terminated.Message) > 4096 {
					continue
				}
				var m oracleFailureMessage
				if json.Unmarshal([]byte(terminated.Message), &m) != nil || m.Schema != "kubeblocks.io/oracle-restore-failure/v1" || m.Reason != "MissingArchive" {
					continue
				}
				if m.Code != "RMAN-06025" && m.Code != "RMAN-06054" {
					continue
				}
				mt, e := time.ParseInLocation("2006-01-02 15:04:05", m.TargetTime, time.UTC)
				if e != nil || !mt.Equal(target) {
					continue
				}
				return fmt.Sprintf("Oracle recovery failed: %s (MissingArchive), target %s, pod %s/%s", m.Code, restore.Spec.RestoreTime, pod.Namespace, pod.Name), nil
			}
		}
	}
	return "", nil
}

func ownerKind(owner *metav1.OwnerReference, group, kind string) bool {
	if owner == nil || owner.UID == "" || owner.Kind != kind {
		return false
	}
	gv, err := schema.ParseGroupVersion(owner.APIVersion)
	return err == nil && gv.Group == group
}
func ownedBy(obj client.Object, group, kind string, uid types.UID) bool {
	o := metav1.GetControllerOf(obj)
	return ownerKind(o, group, kind) && o.UID == uid
}
func sameBackup(a, b *dp.Restore) bool {
	ns := func(r *dp.Restore) string {
		if r.Spec.Backup.Namespace != "" {
			return r.Spec.Backup.Namespace
		}
		return r.Namespace
	}
	return a.Spec.Backup.Name == b.Spec.Backup.Name && ns(a) == ns(b) && a.Spec.Backup.SourceTargetName == b.Spec.Backup.SourceTargetName
}
