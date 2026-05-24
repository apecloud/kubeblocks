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

package instanceset

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// PodRoleEventHandler is registered against the k8score EventReconciler
// dispatcher but is intentionally a no-op for the kbagent roleProbe Event path:
// since the multi-cluster Instance API refactor in apecloud/kubeblocks#9697,
// controllers/workloads InstanceEventReconciler is the sole writer of the Pod
// role label for kbagent roleProbe events and owns the engine-authoritative
// role-version staleness gate. The kbagent roleProbe events are also short-
// circuited at the k8score EventReconciler outer guard so this handler never
// observes them in practice; the no-op body is kept as a belt-and-braces
// guarantee against future dispatcher changes.
type PodRoleEventHandler struct{}

func (h *PodRoleEventHandler) Handle(_ client.Client, _ intctrlutil.RequestCtx, _ record.EventRecorder, _ *corev1.Event) error {
	return nil
}
