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
	"encoding/json"
	"slices"
	"time"

	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	replicaStatusAnnotationKey = "apps.kubeblocks.io/replicas-status"
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
	Reconfigured      *string    `json:"reconfigured,omitempty"` // TODO: component status
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
