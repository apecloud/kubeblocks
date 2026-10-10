/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instancestatus

import (
	"reflect"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// LifecycleObservation contains actual engine observations. Nil fields mean no observation, not absence or completion.
type LifecycleObservation struct {
	InstanceName string
	Membership   *workloads.InstanceMembershipStatus
	Data         *workloads.InstanceDataStatus
	Execution    *workloads.InstanceExecutionObservation
}

// MergeLifecycle preserves identity-bearing facts while validating data against current actual storage.
// A new observer must explicitly confirm data after unknown storage; rediscovering storage alone cannot restore success.
func MergeLifecycle(previous, observed LifecycleObservation, storage *workloads.InstanceStorageIdentity) LifecycleObservation {
	result := LifecycleObservation{Membership: previous.Membership.DeepCopy(), Data: previous.Data.DeepCopy(), Execution: previous.Execution.DeepCopy()}
	if observed.Membership != nil {
		fresh := observed.Membership.DeepCopy()
		if fresh.Identity == nil && fresh.State != workloads.InstanceMembershipUnknown {
			fresh.State = workloads.InstanceMembershipUnknown
			fresh.Reason = "IdentityUnknown"
		}
		if result.Membership != nil && result.Membership.State != workloads.InstanceMembershipAbsent && result.Membership.Identity != nil &&
			(fresh.Identity == nil && fresh.State != workloads.InstanceMembershipUnknown || fresh.Identity != nil && !reflect.DeepEqual(result.Membership.Identity, fresh.Identity)) {
			result.Membership.State = workloads.InstanceMembershipUnknown
			result.Membership.Reason = "IdentityConflict"
			result.Membership.Message = "The previous member binding has not been confirmed absent"
		} else {
			if fresh.State == workloads.InstanceMembershipUnknown && result.Membership != nil && result.Membership.Identity != nil {
				fresh.Identity = result.Membership.Identity.DeepCopy()
			}
			result.Membership = fresh
		}
	}
	if observed.Data != nil {
		fresh := observed.Data.DeepCopy()
		if result.Data != nil && result.Data.Identity != nil && (fresh.Identity == nil || result.Data.Identity.Dataset != "" && fresh.Identity.Dataset == "") {
			fresh.State = workloads.InstanceDataUnknown
			fresh.Identity = result.Data.Identity.DeepCopy()
			fresh.Reason = "IdentityUnknown"
		}
		result.Data = fresh
	}
	if result.Data != nil {
		identity := result.Data.Identity
		switch {
		case identity == nil:
			result.Data.State = workloads.InstanceDataUnknown
		case storageReplaced(&identity.Storage, storage):
			result.Data = &workloads.InstanceDataStatus{State: workloads.InstanceDataUnknown, Reason: "StorageReplaced"}
			if storage != nil {
				result.Data.Identity = &workloads.InstanceDataIdentity{Storage: *storage.DeepCopy()}
			}
		case storage == nil || !storage.Complete || !sameStorageIdentity(&identity.Storage, storage):
			result.Data.State = workloads.InstanceDataUnknown
			result.Data.Reason = "StorageUnknown"
		}
	}
	if observed.Execution != nil {
		fresh := observed.Execution.DeepCopy()
		if !referenceKnown(&fresh.Executor) {
			fresh.State = workloads.InstanceExecutionUnknown
			fresh.Reason = "ExecutorUnknown"
		}
		if executionUnresolved(result.Execution) && !sameExecution(result.Execution, fresh) {
			result.Execution.State = workloads.InstanceExecutionUnknown
			result.Execution.Reason = "IdentityConflict"
			result.Execution.Message = "The previous execution target has not been resolved"
		} else {
			result.Execution = fresh
		}
	}
	return result
}

func sameExecution(a, b *workloads.InstanceExecutionObservation) bool {
	return a.Action == b.Action && reflect.DeepEqual(a.Executor, b.Executor) && reflect.DeepEqual(a.Member, b.Member) && sameDataIdentity(a.Data, b.Data)
}

func executionUnresolved(execution *workloads.InstanceExecutionObservation) bool {
	return execution != nil && (execution.State == workloads.InstanceExecutionRunning || execution.State == workloads.InstanceExecutionUnknown)
}

func lifecycleUnresolved(observation LifecycleObservation) bool {
	return observation.Membership != nil && (observation.Membership.State == workloads.InstanceMembershipUnknown || observation.Membership.State == workloads.InstanceMembershipPresent) || executionUnresolved(observation.Execution)
}

func storageReplaced(old, current *workloads.InstanceStorageIdentity) bool {
	if current == nil {
		return false
	}
	if referencesConflict(old.EphemeralPod, current.EphemeralPod) {
		return true
	}
	oldVolumes, currentVolumes := volumesByName(old), volumesByName(current)
	for name, before := range oldVolumes {
		if after, exists := currentVolumes[name]; exists {
			if referencesConflict(before.Claim, after.Claim) {
				return true
			}
			if before.VolumeName != "" && after.VolumeName != "" && before.VolumeName != after.VolumeName {
				return true
			}
		}
	}
	return old.Complete && current.Complete && !sameStorageIdentity(old, current)
}

func referencesConflict(old, current *workloads.InstanceObjectReference) bool {
	if old == nil || current == nil {
		return false
	}
	return old.UID != "" && current.UID != "" && old.UID != current.UID ||
		old.Name != "" && current.Name != "" && old.Name != current.Name ||
		old.Namespace != "" && current.Namespace != "" && old.Namespace != current.Namespace ||
		old.Cluster != nil && current.Cluster != nil && *old.Cluster != *current.Cluster
}

func volumesByName(storage *workloads.InstanceStorageIdentity) map[string]workloads.InstanceVolumeIdentity {
	result := make(map[string]workloads.InstanceVolumeIdentity, len(storage.Volumes))
	for _, volume := range storage.Volumes {
		volume.OwnerUID = ""
		result[volume.Name] = volume
	}
	return result
}

func sameStorageIdentity(a, b *workloads.InstanceStorageIdentity) bool {
	return a != nil && b != nil && reflect.DeepEqual(a.EphemeralPod, b.EphemeralPod) && reflect.DeepEqual(volumesByName(a), volumesByName(b))
}

func sameDataIdentity(a, b *workloads.InstanceDataIdentity) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Dataset == b.Dataset && sameStorageIdentity(&a.Storage, &b.Storage)
}
