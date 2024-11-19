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

package component

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type KBAgentTaskEventHandler struct{}

func (h *KBAgentTaskEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if !h.isTaskEvent(event) {
		return nil
	}

	taskEvent := &proto.TaskEvent{}
	if err := json.Unmarshal([]byte(event.Message), taskEvent); err != nil {
		return err
	}

	return h.handleEvent(reqCtx, cli, event.InvolvedObject.Namespace, *taskEvent)
}

func (h *KBAgentTaskEventHandler) isTaskEvent(event *corev1.Event) bool {
	return event.ReportingController == proto.ProbeEventReportingController &&
		event.Reason == "task" && event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath
}

func (h *KBAgentTaskEventHandler) handleEvent(reqCtx intctrlutil.RequestCtx, cli client.Client, namespace string, event proto.TaskEvent) error {
	if event.Task == newReplicaTask {
		return handleNewReplicaTaskEvent(reqCtx.Log, reqCtx.Ctx, cli, namespace, event)
	}
	return fmt.Errorf("unsupported kind of task event: %s", event.Task)
}
