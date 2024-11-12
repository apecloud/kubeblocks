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

package v1

// ReplicaRole represents a role that can be assigned to a component instance, defining its behavior and responsibilities.
//
// +kubebuilder:validation:XValidation:rule="self.filter(x, x.participatesInQuorum == true).map(x, x.updatePriority).min() > self.filter(x, x.participatesInQuorum == false).map(x, x.updatePriority).max()",message="Roles participate in quorum should have higher update priority than roles do not participate in quorum."
type ReplicaRole struct {
	// Name defines the role's unique identifier. This value is used to set the "apps.kubeblocks.io/role" label
	// on the corresponding object to identify its role.
	//
	// For example, common role names include:
	// - "leader": The primary/master instance that handles write operations
	// - "follower": Secondary/replica instances that replicate data from the leader
	// - "learner": Read-only instances that don't participate in elections
	//
	// This field is immutable once set.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`^.*[^\s]+.*$`
	Name string `json:"name"`

	// UpdatePriority determines the order in which pods with different roles are updated.
	// Pods are sorted by this priority (higher numbers = higher priority) and updated accordingly.
	// Roles with the highest priority will be updated last.
	// The default priority is 0.
	//
	// For example:
	// - Leader role may have priority 2 (updated last)
	// - Follower role may have priority 1 (updated before leader)
	// - Learner role may have priority 0 (updated first)
	//
	// This field is immutable once set.
	//
	// +kubebuilder:default=0
	// +optional
	UpdatePriority int `json:"updatePriority"`

	// ParticipatesInQuorum indicates if pods with this role are counted when determining quorum.
	// This affects update strategies that need to maintain quorum for availability. Roles participate
	// in quorum should have higher update priority than roles do not participate in quorum.
	// The default value is false.
	//
	// For example, in a 5-pod component where:
	// - 2 learner pods (participatesInQuorum=false)
	// - 2 follower pods (participatesInQuorum=true)
	// - 1 leader pod (participatesInQuorum=true)
	// The quorum size would be 3 (based on the 3 participating pods), allowing parallel updates
	// of 2 learners and 1 follower while maintaining quorum.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	ParticipatesInQuorum bool `json:"participatesInQuorum bool"`

	// SwitchoverBeforeUpdate indicates if a role switchover operation should be performed before
	// updating or scaling in pods with this role. This is typically used for leader roles to
	// ensure minimal disruption during updates.
	// The default value is false.
	//
	// This field is immutable once set.
	//
	// +kubebuilder:default=false
	// +optional
	SwitchoverBeforeUpdate bool `json:"switchoverBeforeUpdate"`
}
