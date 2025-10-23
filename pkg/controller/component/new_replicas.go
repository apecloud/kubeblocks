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
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	// new replicas task & event
	newReplicaTask                           = "newReplica"
	defaultNewReplicaTaskReportPeriodSeconds = 60
)

func NewReplicaTask(compName, uid string, source lifecycle.Replica, replicas []string) (map[string]string, error) {
	host, port, err := source.StreamingEndpoint()
	if err != nil {
		return nil, err
	}
	task := proto.Task{
		Instance:            compName,
		Task:                newReplicaTask,
		UID:                 uid,
		Replicas:            strings.Join(replicas, ","),
		NotifyAtFinish:      true,
		ReportPeriodSeconds: defaultNewReplicaTaskReportPeriodSeconds,
		NewReplica: &proto.NewReplicaTask{
			Remote:   host,
			Port:     port,
			Replicas: strings.Join(replicas, ","),
		},
	}
	return buildKBAgentTaskEnv(task)
}

func handleNewReplicaTaskEvent(logger logr.Logger, ctx context.Context, cli client.Client, namespace string, event proto.TaskEvent) error {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      event.Replica,
	}
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, key, pod); err != nil {
		logger.Error(err, "get pod failed when handle new replica event",
			"code", event.Code, "finished", !event.EndTime.IsZero(), "message", event.Message)
		return err
	}

	var err error
	finished := !event.EndTime.IsZero()
	if finished && event.Code == 0 {
		err = handleNewReplicaTaskEvent4Finished(ctx, cli, pod, event)
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

func handleNewReplicaTaskEvent4Finished(ctx context.Context, cli client.Client, pod *corev1.Pod, event proto.TaskEvent) error {
	// if err := func() error {
	//	envKey := types.NamespacedName{
	//		Namespace: its.Namespace,
	//		Name:      constant.GetCompEnvCMName(its.Name),
	//	}
	//	obj := &corev1.ConfigMap{}
	//	err := cli.Get(ctx, envKey, obj)
	//	if err != nil {
	//		return err
	//	}
	//
	//	parameters, err := updateKBAgentTaskEnv(obj.Data, func(task proto.Task) *proto.Task {
	//		if task.Task == newReplicaTask {
	//			replicas := strings.Split(task.Replicas, ",")
	//			replicas = slices.DeleteFunc(replicas, func(r string) bool {
	//				return r == event.Replica
	//			})
	//			if len(replicas) == 0 {
	//				return nil
	//			}
	//			task.Replicas = strings.Join(replicas, ",")
	//			if task.NewReplica != nil {
	//				task.NewReplica.Replicas = task.Replicas
	//			}
	//		}
	//		return &task
	//	})
	//	if err != nil {
	//		return err
	//	}
	//	if parameters == nil {
	//		return nil // do nothing
	//	}
	//
	//	if obj.Data == nil {
	//		obj.Data = make(map[string]string)
	//	}
	//	for k, v := range parameters {
	//		obj.Data[k] = v
	//	}
	//	return cli.Update(ctx, obj)
	// }(); err != nil {
	//	return err
	// }

	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[constant.LifeCycleDataLoadedAnnotationKey] = "true"
	return cli.Status().Update(ctx, pod)
}
