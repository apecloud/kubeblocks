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
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	replicaStatusMessageKey = "Replica/status"

	// new replicas task & event
	newReplicaTask                           = "newReplica"
	defaultNewReplicaTaskReportPeriodSeconds = 60
)

type replicasStatus struct {
	Replicas int32           `json:"replicas"`
	Status   []replicaStatus `json:"status"`
}

type replicaStatus struct {
	Name              string       `json:"name"`
	Generation        int64        `json:"generation"`
	CreationTimestamp time.Time    `json:"creationTimestamp"`
	DeletionTimestamp *time.Time   `json:"deletionTimestamp,omitempty"`
	Phase             replicaPhase `json:"phase"`
	Message           string       `json:"message,omitempty"`
}

type replicaPhase string

const (
	replicaPhasePending  replicaPhase = "Pending"
	replicaPhaseCreating replicaPhase = "Creating"
	replicaPhaseRunning  replicaPhase = "Running"
	replicaPhaseUpdating replicaPhase = "Updating"
	replicaPhaseDeleting replicaPhase = "Deleting"
)

func NewReplicas(comp *appsv1.Component, replicas []string) error {
	return updateReplicasStatusFunc(comp, func(status *replicasStatus) error {
		status.Replicas = comp.Spec.Replicas
		for _, name := range replicas {
			if slices.ContainsFunc(status.Status, func(s replicaStatus) bool {
				return s.Name == name
			}) {
				continue
			}
			status.Status = append(status.Status, replicaStatus{
				Name:              name,
				Generation:        comp.Generation,
				CreationTimestamp: time.Now(),
				Phase:             replicaPhasePending,
			})
		}
		return nil
	})
}

func DeleteReplicas(comp *appsv1.Component, replicas []string) error {
	return updateReplicasStatusFunc(comp, func(status *replicasStatus) error {
		status.Replicas = comp.Spec.Replicas
		status.Status = slices.DeleteFunc(status.Status, func(s replicaStatus) bool {
			return slices.Contains(replicas, s.Name)
		})
		return nil
	})
}

func StatusReplicas(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, comp *appsv1.Component) error {
	pods, err := ListOwnedPods(ctx, cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
	if err != nil {
		return err
	}
	return updateReplicasStatusFunc(comp, func(status *replicasStatus) error {
		status.Replicas = comp.Spec.Replicas
		for _, pod := range pods {
			if slices.ContainsFunc(status.Status, func(s replicaStatus) bool {
				return s.Name == pod.Name
			}) {
				continue
			}
			status.Status = append(status.Status, replicaStatus{
				Name:              pod.Name,
				Generation:        comp.Generation,
				CreationTimestamp: pod.CreationTimestamp.Time,
				Phase:             replicaPhaseRunning,
			})
		}
		return nil
	})
}

func ReplicasInProvisioning(comp *appsv1.Component) ([]string, error) {
	status, err := getReplicasStatus(comp)
	if err != nil {
		return nil, err
	}
	replicas := make([]string, 0)
	for _, s := range status.Status {
		if s.Phase == replicaPhaseCreating || s.Phase == replicaPhasePending {
			replicas = append(replicas, s.Name)
		}
	}
	return replicas, nil
}

func NewReplicaTask(compName string, uid string, source *corev1.Pod, replicas []string) (map[string]string, error) {
	port, err := intctrlutil.GetPortByName(*source, kbagent.ContainerName, kbagent.DefaultStreamingPortName)
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
			Remote:   source.Status.PodIP,
			Port:     port,
			Replicas: strings.Join(replicas, ","),
		},
	}
	return buildKBAgentTaskEnv(task)
}

func getReplicasStatus(comp *appsv1.Component) (replicasStatus, error) {
	if comp.Status.Message == nil {
		return replicasStatus{}, nil
	}
	message, ok := comp.Status.Message[replicaStatusMessageKey]
	if !ok {
		return replicasStatus{}, nil
	}
	status := &replicasStatus{}
	err := json.Unmarshal([]byte(message), &status)
	if err != nil {
		return replicasStatus{}, err
	}
	return *status, nil
}

func updateReplicasStatusFunc(comp *appsv1.Component, f func(status *replicasStatus) error) error {
	if f == nil {
		return nil
	}

	status, err := getReplicasStatus(comp)
	if err != nil {
		return err
	}

	if err = f(&status); err != nil {
		return err
	}

	out, err := json.Marshal(&status)
	if err != nil {
		return err
	}

	if comp.Status.Message == nil {
		comp.Status.Message = make(map[string]string)
	}
	comp.Status.Message[replicaStatusMessageKey] = string(out)

	return nil
}

func updateReplicaStatus(comp *appsv1.Component, name string, f func(*replicaStatus) error) error {
	return updateReplicasStatusFunc(comp, func(status *replicasStatus) error {
		for i := range status.Status {
			if status.Status[i].Name == name {
				if f != nil {
					return f(&status.Status[i])
				}
				return nil
			}
		}
		return fmt.Errorf("replica %s not found", name)
	})
}

func handleNewReplicaTaskEvent(ctx context.Context, cli client.Client, comp *appsv1.Component, event proto.TaskEvent) error {
	finished := !event.EndTime.IsZero()
	if finished && event.Code == 0 {
		return handleNewReplicaTaskEvent4Finished(ctx, cli, comp, event)
	}
	if finished {
		return handleNewReplicaTaskEvent4Failed(comp, event)
	}
	return handleNewReplicaTaskEvent4Unfinished(comp, event)
}

func handleNewReplicaTaskEvent4Finished(ctx context.Context, cli client.Client, comp *appsv1.Component, event proto.TaskEvent) error {
	if err := func() error {
		envKey := types.NamespacedName{
			Namespace: comp.Namespace,
			Name:      constant.GetCompEnvCMName(comp.Name),
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
	return updateReplicaStatus(comp, event.Replica, func(status *replicaStatus) error {
		status.Generation = comp.Generation // TODO: generation
		status.Phase = replicaPhaseRunning
		status.Message = event.Message
		return nil
	})
}

func handleNewReplicaTaskEvent4Unfinished(comp *appsv1.Component, event proto.TaskEvent) error {
	return updateReplicaStatus(comp, event.Replica, func(status *replicaStatus) error {
		status.Generation = comp.Generation // TODO: generation
		status.Phase = replicaPhaseCreating
		return nil
	})
}

func handleNewReplicaTaskEvent4Failed(comp *appsv1.Component, event proto.TaskEvent) error {
	return updateReplicaStatus(comp, event.Replica, func(status *replicaStatus) error {
		status.Generation = comp.Generation // TODO: generation
		status.Message = event.Message
		return nil
	})
}
