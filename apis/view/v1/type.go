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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectReference defines a reference to an object.
type ObjectReference struct {
	// Namespace of the referent.
	// Default is same as the ReconciliationPlan object.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the referent.
	// Default is same as the ReconciliationPlan object.
	//
	// +optional
	Name string `json:"name,omitempty"`
}

// ObjectType defines an object type.
type ObjectType struct {
	// APIVersion of the type.
	//
	APIVersion string `json:"apiVersion"`

	// Kind of the type.
	//
	Kind string `json:"kind"`
}

// ObjectSummary defines the total and change of an object.
type ObjectSummary struct {
	// Type of the object.
	//
	Type ObjectType `json:"type"`

	// Total number of the object of type defined by Type.
	//
	Total int32 `json:"total"`

	// ChangeSummary summarizes the change by comparing the final state to the current state of this type.
	// Nil means no change.
	//
	// +optional
	ChangeSummary *ObjectChangeSummary `json:"changeSummary,omitempty"`
}

// ObjectChangeSummary defines changes of an object.
type ObjectChangeSummary struct {
	// Added specifies the number of object will be added.
	//
	// +optional
	Added *int32 `json:"added,omitempty"`

	// Updated specifies the number of object will be updated.
	//
	// +optional
	Updated *int32 `json:"updated,omitempty"`

	// Deleted specifies the number of object will be deleted.
	//
	// +optional
	Deleted *int32 `json:"deleted,omitempty"`
}

// ObjectTreeNode defines an object tree specified by rules in ReconciliationViewDefinition.
type ObjectTreeNode struct {
	// ObjectReference specifies the Object this plan described.
	//
	Root corev1.ObjectReference `json:"objectReference"`

	// Secondaries describes all the secondary objects of this object, if any.
	// The secondary objects are collected by the rules specified in ReconciliationViewDefinition.
	//
	// +optional
	Secondaries []ObjectTreeNode `json:"secondaries,omitempty"`
}

// ObjectChange defines a detailed change of an object.
type ObjectChange struct {
	// ObjectReference specifies the Object this plan described.
	//
	ObjectReference corev1.ObjectReference `json:"objectReference"`

	// Type specifies the change type.
	// Event - specifies that this is a Kubernetes Event.
	// ObjectCreation - specifies that this is an object creation.
	// ObjectUpdate - specifies that this is an object update.
	// ObjectDeletion - specifies that this is an object deletion.
	//
	// +kubebuilder:validation:Enum={Event, ObjectCreation, ObjectUpdate, ObjectDeletion}
	Type string `json:"type"`

	// EventAttributes specifies the attributes of the event when Type is Event.
	//
	// +optional
	EventAttributes *EventAttributes `json:"eventAttributes,omitempty"`

	// State represents the state calculated by StateEvaluationExpression defines in ReconciliationViewDefinition when this change occurs.
	//
	State string `json:"state,omitempty"`

	// Revision specifies the revision of the object after this change.
	// Revision can be compared globally between all ObjectChanges of all Objects, to build a total order object change sequence.
	//
	Revision int64 `json:"revision"`

	// Timestamp is a timestamp representing the ReconciliationPlan Controller time when this change occurred.
	// It is not guaranteed to be set in happens-before order across separate changes.
	// It is represented in RFC3339 form and is in UTC.
	// It is set to empty when used in ObjectReconciliationPlan.
	//
	// +optional
	Timestamp *metav1.Time `json:"timestamp,omitempty"`

	// Description describes the change in a user-friendly way.
	//
	Description string `json:"description"`

	// LocalDescription is the localized version of Description by using the Locale specified in `spec.locale`.
	// Empty if the `spec.locale` is not specified.
	//
	LocalDescription *string `json:"LocalDescription,omitempty"`
}

// EventAttributes defines attributes of the Event.
type EventAttributes struct {
	// Type of the Event.
	//
	Type string `json:"type"`

	// Reason of the Event.
	//
	Reason string `json:"reason"`
}
