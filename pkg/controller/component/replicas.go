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
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
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

func NewReplicaTask(compName string, uid string, source *corev1.Pod, replicas []string) (proto.Task, error) {
	port, err := intctrlutil.GetPortByName(*source, kbagent.ContainerName, kbagent.DefaultStreamingPortName)
	if err != nil {
		return proto.Task{}, err
	}
	return proto.Task{
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
	}, nil
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

// func updateReplicasStatus(comp *appsv1.Component, status replicasStatus) error {
//	return updateReplicasStatusFunc(comp, func(s *replicasStatus) error {
//		*s = status
//		return nil
//	})
// }

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

func handleNewReplicaTaskEvent(comp *appsv1.Component, event proto.TaskEvent) error {
	if !event.EndTime.IsZero() && event.Code == 0 {
		return handleNewReplicaTaskEvent4Finished(comp, event)
	}
	if !event.EndTime.IsZero() {
		return handleNewReplicaTaskEvent4Unfinished(comp, event)
	}
	return handleNewReplicaTaskEvent4Failed(comp, event)
}

func handleNewReplicaTaskEvent4Finished(comp *appsv1.Component, event proto.TaskEvent) error {
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
