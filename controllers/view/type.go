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

package view

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var KBOwnershipRules = []OwnershipRule{
	{
		Primary: objectType(appsv1alpha1.APIVersion, appsv1alpha1.ClusterKind),
		OwnedResources: []OwnedResource{
			{
				Secondary: objectType(appsv1alpha1.APIVersion, appsv1alpha1.ComponentKind),
				Criteria: OwnershipCriteria{
					LabelCriteria: map[string]string{
						constant.AppInstanceLabelKey:  "$(primary.name)",
						constant.AppManagedByLabelKey: constant.AppName,
					},
				},
			},
		},
	},
	{
		Primary: objectType(appsv1alpha1.APIVersion, appsv1alpha1.ComponentKind),
		OwnedResources: []OwnedResource{
			{
				Secondary: objectType(workloads.GroupVersion.String(), workloads.Kind),
				Criteria: OwnershipCriteria{
					LabelCriteria: map[string]string{
						constant.KBAppComponentLabelKey: "$(primary.name)",
						constant.AppManagedByLabelKey:   constant.AppName,
					},
				},
			},
			{
				Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.ServiceKind),
				Criteria: OwnershipCriteria{
					LabelCriteria: map[string]string{
						constant.KBAppComponentLabelKey: "$(primary.name)",
						constant.AppManagedByLabelKey:   constant.AppName,
					},
				},
			},
		},
	},
}

var rootObjectType = viewv1.ObjectType{
	APIVersion: appsv1alpha1.APIVersion,
	Kind:       appsv1alpha1.ClusterKind,
}

var (
	defaultStateEvaluationExpression = viewv1.StateEvaluationExpression{
		CELExpression: &viewv1.CELExpression{
			Expression: "object.status.phase == \"Running\"",
		},
	}

	defaultLocale = pointer.String("en")
)

// OwnershipRule defines an ownership rule between primary resource and its secondary resources.
type OwnershipRule struct {
	// Primary specifies the primary object type.
	//
	Primary viewv1.ObjectType `json:"primary"`

	// OwnedResources specifies all the secondary resources of Primary.
	//
	OwnedResources []OwnedResource `json:"ownedResources"`
}

// OwnedResource defines a secondary resource and the ownership criteria between its primary resource.
type OwnedResource struct {
	// Secondary specifies the secondary object type.
	//
	Secondary viewv1.ObjectType `json:"secondary"`

	// Criteria specifies the ownership criteria with its primary resource.
	//
	Criteria OwnershipCriteria `json:"criteria"`
}

// OwnershipCriteria defines an ownership criteria.
// Only one of SelectorCriteria, LabelCriteria or BuiltinRelationshipCriteria should be configured.
type OwnershipCriteria struct {
	// SelectorCriteria specifies the selector field path in the primary object.
	// For example, if the StatefulSet is the primary resource, selector will be "spec.selector".
	// The selector field should be a map[string]string
	// or LabelSelector (https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/label-selector/#LabelSelector)
	//
	// +optional
	SelectorCriteria *FieldPath `json:"selectorCriteria,omitempty"`

	// LabelCriteria specifies the labels used to select the secondary objects.
	// The value of each k-v pair can contain placeholder that will be replaced by the ReconciliationView Controller.
	// Placeholder is formatted as "$(PLACEHOLDER)".
	// Currently supported PLACEHOLDER:
	// primary.name - the name of the primary object.
	//
	// +optional
	LabelCriteria map[string]string `json:"labelCriteria,omitempty"`

	// BuiltinRelationshipCriteria specifies to use the well-known builtin relationship between some primary resource with its secondary resources.
	// Currently supported resources:
	// PVC-PV pair.
	//
	// +optional
	BuiltinRelationshipCriteria *bool `json:"builtinRelationshipCriteria,omitempty"`

	// Validation specifies the method to validate the OwnerReference of secondary resources.
	//
	// +kubebuilder:validation:Enum={Controller, Owner, None}
	// +kubebuilder:default=Controller
	// +optional
	Validation ValidationType `json:"validation,omitempty"`
}

// FieldPath defines a field path.
type FieldPath struct {
	// Path of the field.
	//
	Path string `json:"path"`
}

// ValidationType specifies the method to validate the OwnerReference of secondary resources.
type ValidationType string

const (
	// ControllerValidation requires the secondary resource to have the primary resource
	// in its OwnerReference with controller set to true.
	ControllerValidation ValidationType = "Controller"

	// OwnerValidation requires the secondary resource to have the primary resource
	// in its OwnerReference.
	OwnerValidation ValidationType = "Owner"

	// NoValidation means no validation is performed on the OwnerReference.
	NoValidation ValidationType = "None"
)
