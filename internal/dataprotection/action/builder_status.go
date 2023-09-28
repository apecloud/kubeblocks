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

package action

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type statusBuilder struct {
	status *dpv1alpha1.ActionStatus
}

func newStatusBuilder(a Action) *statusBuilder {
	sb := &statusBuilder{
		status: &dpv1alpha1.ActionStatus{
			Name:       a.GetName(),
			ActionType: a.Type(),
			Phase:      dpv1alpha1.ActionPhaseRunning,
		},
	}
	return sb.startTimestamp(nil)
}

func (b *statusBuilder) phase(phase dpv1alpha1.ActionPhase) *statusBuilder {
	b.status.Phase = phase
	return b
}

func (b *statusBuilder) reason(reason string) *statusBuilder {
	b.status.FailureReason = reason
	return b
}

func (b *statusBuilder) startTimestamp(timestamp *metav1.Time) *statusBuilder {
	t := timestamp
	if t == nil {
		t = &metav1.Time{
			Time: metav1.Now().UTC(),
		}
	}
	b.status.StartTimestamp = t
	return b
}

func (b *statusBuilder) completionTimestamp(timestamp *metav1.Time) *statusBuilder {
	t := timestamp
	if t == nil {
		t = &metav1.Time{
			Time: metav1.Now().UTC(),
		}
	}
	b.status.CompletionTimestamp = t
	return b
}

func (b *statusBuilder) objectRef(objectRef *corev1.ObjectReference) *statusBuilder {
	b.status.ObjectRef = objectRef
	return b
}

func (b *statusBuilder) withErr(err error) *statusBuilder {
	if err == nil {
		return b
	}
	b.status.FailureReason = err.Error()
	b.status.Phase = dpv1alpha1.ActionPhaseFailed
	return b
}

func (b *statusBuilder) totalSize(size string) *statusBuilder {
	b.status.TotalSize = size
	return b
}

func (b *statusBuilder) timeRange(start, end *metav1.Time) *statusBuilder {
	b.status.TimeRange = &dpv1alpha1.BackupTimeRange{
		Start: start,
		End:   end,
	}
	return b
}

func (b *statusBuilder) build() *dpv1alpha1.ActionStatus {
	return b.status
}
