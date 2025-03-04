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
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	// new replicas task & event
	newReplicaTask                           = "newReplica"
	defaultNewReplicaTaskReportPeriodSeconds = 60

	// render task
	renderTask = "render"
)

func NewReplicaTask(compName, uid string, source *corev1.Pod, replicas []string) (*proto.Task, error) {
	port, err := intctrlutil.GetPortByName(*source, kbagent.ContainerName, kbagent.DefaultStreamingPortName)
	if err != nil {
		return nil, err
	}
	return &proto.Task{
		Instance:            compName,
		Task:                newReplicaTask,
		UID:                 uid,
		Replicas:            strings.Join(replicas, ","),
		NotifyAtFinish:      true,
		ReportPeriodSeconds: defaultNewReplicaTaskReportPeriodSeconds,
		NewReplica: &proto.NewReplicaTask{
			Remote:   intctrlutil.PodFQDN(source.Namespace, compName, source.Name),
			Port:     port,
			Replicas: strings.Join(replicas, ","),
		},
	}, nil
}

func NewRenderTask(compName, uid string, replicas []string, synthesizedComp *SynthesizedComponent, files map[string][]string) *proto.Task {
	task := proto.Task{
		Instance:            compName,
		Task:                renderTask,
		UID:                 uid,
		Replicas:            strings.Join(replicas, ","),
		NotifyAtFinish:      false,
		ReportPeriodSeconds: 0,
		Render: &proto.RenderTask{
			Templates: []proto.RenderTaskFileTemplate{},
		},
	}
	for _, tpl := range synthesizedComp.FileTemplates {
		if tpl.RequiresPodRender {
			task.Render.Templates = append(task.Render.Templates, proto.RenderTaskFileTemplate{
				Name:      tpl.Name,
				Files:     templateFiles(synthesizedComp, tpl.Name, files[tpl.Name]),
				Variables: tpl.Variables,
			})
		}
	}
	if len(task.Render.Templates) > 0 {
		return &task
	}
	return nil
}

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

func handleNewReplicaTaskEvent(logger logr.Logger, ctx context.Context, cli client.Client, namespace string, event proto.TaskEvent) error {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      event.Instance,
	}
	its := &workloads.InstanceSet{}
	if err := cli.Get(ctx, key, its); err != nil {
		logger.Error(err, "get ITS failed when handle new replica task event",
			"code", event.Code, "finished", !event.EndTime.IsZero(), "message", event.Message)
		return err
	}

	var err error
	finished := !event.EndTime.IsZero()
	switch {
	case finished && event.Code == 0:
		err = handleNewReplicaTaskEvent4Finished(ctx, cli, its, event)
	case finished:
		err = handleNewReplicaTaskEvent4Failed(ctx, cli, its, event)
	default:
		err = handleNewReplicaTaskEvent4Unfinished(ctx, cli, its, event)
	}
	if err != nil {
		logger.Error(err, "handle new replica task event failed",
			"code", event.Code, "finished", finished, "message", event.Message)
	} else {
		logger.Info("handle new replica task event success",
			"code", event.Code, "finished", finished, "message", event.Message)
	}
	return err
}

func handleNewReplicaTaskEvent4Finished(ctx context.Context, cli client.Client, its *workloads.InstanceSet, event proto.TaskEvent) error {
	if err := func() error {
		envKey := types.NamespacedName{
			Namespace: its.Namespace,
			Name:      constant.GetCompEnvCMName(its.Name),
		}
		obj := &corev1.ConfigMap{}
		err := cli.Get(ctx, envKey, obj, inDataContext())
		if err != nil {
			return err
		}

		parameters, err := updateKBAgentTaskEnv(obj.Data, func(task proto.Task) *proto.Task {
			if task.Task == newReplicaTask {
				replicas := strings.Split(task.Replicas, ",")
				replicas = slices.DeleteFunc(replicas, func(r string) bool {
					return r == event.Replica
				})
				if len(replicas) == 0 {
					return nil
				}
				task.Replicas = strings.Join(replicas, ",")
				if task.NewReplica != nil {
					task.NewReplica.Replicas = task.Replicas
				}
			}
			return &task
		})
		if err != nil {
			return err
		}
		if parameters == nil {
			return nil // do nothing
		}

		if obj.Data == nil {
			obj.Data = make(map[string]string)
		}
		for k, v := range parameters {
			obj.Data[k] = v
		}
		return cli.Update(ctx, obj, inDataContext())
	}(); err != nil {
		return err
	}
	return updateReplicaStatusFunc(ctx, cli, its, event.Replica, func(status *ReplicaStatus) error {
		status.Message = ""
		status.Provisioned = true
		status.DataLoaded = ptr.To(true)
		return nil
	})
}

func handleNewReplicaTaskEvent4Unfinished(ctx context.Context, cli client.Client, its *workloads.InstanceSet, event proto.TaskEvent) error {
	return updateReplicaStatusFunc(ctx, cli, its, event.Replica, func(status *ReplicaStatus) error {
		status.Message = event.Message
		status.Provisioned = true
		status.DataLoaded = ptr.To(false)
		return nil
	})
}

func handleNewReplicaTaskEvent4Failed(ctx context.Context, cli client.Client, its *workloads.InstanceSet, event proto.TaskEvent) error {
	return updateReplicaStatusFunc(ctx, cli, its, event.Replica, func(status *ReplicaStatus) error {
		status.Message = event.Message
		status.Provisioned = true
		return nil
	})
}

func updateReplicaStatusFunc(ctx context.Context, cli client.Client,
	its *workloads.InstanceSet, replicaName string, f func(*ReplicaStatus) error) error {
	if err := UpdateReplicasStatusFunc(its, func(status *ReplicasStatus) error {
		for i := range status.Status {
			if status.Status[i].Name == replicaName {
				if f != nil {
					return f(&status.Status[i])
				}
				return nil
			}
		}
		return fmt.Errorf("replica %s not found", replicaName)
	}); err != nil {
		return err
	}
	return cli.Update(ctx, its)
}
