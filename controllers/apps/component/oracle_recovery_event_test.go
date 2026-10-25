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

package component

import (
	"context"
	"testing"

	"github.com/apecloud/kubeblocks/pkg/constant"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOracleRecoveryPodWakeup(t *testing.T) {
	r := &ComponentReconciler{}
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Labels: map[string]string{constant.AppManagedByLabelKey: constant.AppName, constant.AppInstanceLabelKey: "target", constant.KBAppComponentLabelKey: "oracle"}}, Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "oracle", LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, Message: `{"schema":"kubeblocks.io/oracle-restore-failure/v1"}`}}}}}}
	requests := r.filterOracleRecoveryPods(context.Background(), p)
	if len(requests) != 1 || requests[0].Namespace != "ns" || requests[0].Name != "target-oracle" {
		t.Fatalf("wrong wakeup: %#v", requests)
	}
	p.Status.ContainerStatuses[0].Ready = true
	if len(r.filterOracleRecoveryPods(context.Background(), p)) != 0 {
		t.Fatal("ready container woke recovery")
	}
	p.Status.ContainerStatuses[0].Ready = false
	p.Status.ContainerStatuses[0].LastTerminationState.Terminated.Message = "ORA-00308"
	if len(r.filterOracleRecoveryPods(context.Background(), p)) != 0 {
		t.Fatal("ordinary error woke recovery")
	}
}
