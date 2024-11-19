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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	replicaStatusAnnotationKey = "apps.kubeblocks.io/replicas-status"

	// new replicas task & event
	newReplicaTask                           = "newReplica"
	defaultNewReplicaTaskReportPeriodSeconds = 60
)

type ReplicasStatus struct {
	Replicas int32           `json:"replicas"`
	Status   []ReplicaStatus `json:"status"`
}

type ReplicaStatus struct {
	Name              string     `json:"name"`
	Generation        string     `json:"generation"`
	CreationTimestamp time.Time  `json:"creationTimestamp"`
	DeletionTimestamp *time.Time `json:"deletionTimestamp,omitempty"`
	Message           string     `json:"message,omitempty"`
	Provisioned       bool       `json:"provisioned,omitempty"`
	DataLoaded        *bool      `json:"dataLoaded,omitempty"`
	MemberJoined      *bool      `json:"memberJoined,omitempty"`
}

func BuildReplicasStatus(running, proto *workloads.InstanceSet) {
	if running == nil || proto == nil {
		return
	}
	annotations := running.Annotations
	if annotations == nil {
		return
	}
	message, ok := annotations[replicaStatusAnnotationKey]
	if !ok {
		return
	}
	if proto.Annotations == nil {
		proto.Annotations = make(map[string]string)
	}
	proto.Annotations[replicaStatusAnnotationKey] = message
}

func NewReplicasStatus(its *workloads.InstanceSet, replicas []string, hasMemberJoin, hasDataAction bool) error {
	loaded := func() *bool {
		if hasDataAction {
			return ptr.To(false)
		}
		return nil
	}()
	joined := func() *bool {
		if hasMemberJoin {
			return ptr.To(false)
		}
		return nil
	}()
	return UpdateReplicasStatusFunc(its, func(status *ReplicasStatus) error {
		status.Replicas = *its.Spec.Replicas
		if status.Status == nil {
			status.Status = make([]ReplicaStatus, 0)
		}
		for _, name := range replicas {
			if slices.ContainsFunc(status.Status, func(s ReplicaStatus) bool {
				return s.Name == name
			}) {
				continue
			}
			status.Status = append(status.Status, ReplicaStatus{
				Name:              name,
				Generation:        compGenerationFromITS(its),
				CreationTimestamp: time.Now(),
				Provisioned:       false,
				DataLoaded:        loaded,
				MemberJoined:      joined,
			})
		}
		return nil
	})
}

func DeleteReplicasStatus(its *workloads.InstanceSet, replicas []string, f func(status ReplicaStatus)) error {
	return UpdateReplicasStatusFunc(its, func(status *ReplicasStatus) error {
		status.Replicas = *its.Spec.Replicas
		status.Status = slices.DeleteFunc(status.Status, func(s ReplicaStatus) bool {
			if slices.Contains(replicas, s.Name) {
				if f != nil {
					f(s)
				}
				return true
			}
			return false
		})
		return nil
	})
}

func StatusReplicasStatus(its *workloads.InstanceSet, replicas []string, hasMemberJoin, hasDataAction bool) error {
	loaded := func() *bool {
		if hasDataAction {
			return ptr.To(true)
		}
		return nil
	}()
	joined := func() *bool {
		if hasMemberJoin {
			return ptr.To(true)
		}
		return nil
	}()
	return UpdateReplicasStatusFunc(its, func(status *ReplicasStatus) error {
		status.Replicas = *its.Spec.Replicas
		if status.Status == nil {
			status.Status = make([]ReplicaStatus, 0)
		}
		for _, replica := range replicas {
			i := slices.IndexFunc(status.Status, func(s ReplicaStatus) bool {
				return s.Name == replica
			})
			if i >= 0 {
				status.Status[i].Provisioned = true
			} else {
				status.Status = append(status.Status, ReplicaStatus{
					Name:              replica,
					Generation:        compGenerationFromITS(its),
					CreationTimestamp: its.CreationTimestamp.Time,
					Provisioned:       true,
					DataLoaded:        loaded,
					MemberJoined:      joined,
				})
			}
		}
		return nil
	})
}

func UpdateReplicasStatusFunc(its *workloads.InstanceSet, f func(status *ReplicasStatus) error) error {
	if f == nil {
		return nil
	}

	status, err := getReplicasStatus(its)
	if err != nil {
		return err
	}

	if err = f(&status); err != nil {
		return err
	}

	return setReplicasStatus(its, status)
}

func GetReplicasStatusFunc(its *workloads.InstanceSet, f func(ReplicaStatus) bool) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	status, err := getReplicasStatus(its)
	if err != nil {
		return nil, err
	}
	replicas := make([]string, 0)
	for _, s := range status.Status {
		if f(s) {
			replicas = append(replicas, s.Name)
		}
	}
	return replicas, nil
}

func NewReplicaTask(compName, uid string, source *corev1.Pod, replicas []string) (map[string]string, error) {
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

func compGenerationFromITS(its *workloads.InstanceSet) string {
	if its == nil {
		return ""
	}
	annotations := its.Annotations
	if annotations == nil {
		return ""
	}
	return annotations[constant.KubeBlocksGenerationKey]
}

func getReplicasStatus(its *workloads.InstanceSet) (ReplicasStatus, error) {
	if its == nil {
		return ReplicasStatus{}, nil
	}
	annotations := its.GetAnnotations()
	if annotations == nil {
		return ReplicasStatus{}, nil
	}
	message, ok := annotations[replicaStatusAnnotationKey]
	if !ok {
		return ReplicasStatus{}, nil
	}
	status := &ReplicasStatus{}
	err := json.Unmarshal([]byte(message), &status)
	if err != nil {
		return ReplicasStatus{}, err
	}
	return *status, nil
}

func setReplicasStatus(its *workloads.InstanceSet, status ReplicasStatus) error {
	if its == nil {
		return nil
	}
	out, err := json.Marshal(&status)
	if err != nil {
		return err
	}
	annotations := its.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[replicaStatusAnnotationKey] = string(out)
	its.SetAnnotations(annotations)
	return nil
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
