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
	"fmt"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	vsv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/utils/pointer"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var (
	clusterCriteria = OwnershipCriteria{
		LabelCriteria: map[string]string{
			constant.AppInstanceLabelKey:  "$(primary.name)",
			constant.AppManagedByLabelKey: constant.AppName,
		},
	}

	componentCriteria = OwnershipCriteria{
		LabelCriteria: map[string]string{
			constant.AppInstanceLabelKey:  "$(primary)",
			constant.AppManagedByLabelKey: constant.AppName,
		},
	}

	itsCriteria = OwnershipCriteria{
		LabelCriteria: map[string]string{
			instanceset.WorkloadsManagedByLabelKey: workloads.Kind,
			instanceset.WorkloadsInstanceLabelKey:  "$(primary.name)",
		},
	}

	configurationCriteria = componentCriteria

	backupCriteria = OwnershipCriteria{
		LabelCriteria: map[string]string{
			constant.AppInstanceLabelKey:  "$(primary)",
			constant.AppManagedByLabelKey: types.AppName,
		},
	}

	restoreCriteria = backupCriteria

	pvcCriteria = OwnershipCriteria{
		SpecifiedNameCriteria: &FieldPath{
			Path: "spec.volumeName",
		},
	}

	FullKBOwnershipRules = []OwnershipRule{
		{
			Primary: objectType(kbappsv1.SchemeGroupVersion.String(), kbappsv1.ClusterKind),
			OwnedResources: []OwnedResource{
				{
					Secondary: objectType(kbappsv1.SchemeGroupVersion.String(), kbappsv1.ComponentKind),
					Criteria:  clusterCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.ServiceKind),
					Criteria:  clusterCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.SecretKind),
					Criteria:  clusterCriteria,
				},
				// TODO(free6om): should own BackupPolicy and BackSchedule ?
			},
		},
		{
			Primary: objectType(kbappsv1.SchemeGroupVersion.String(), kbappsv1.ComponentKind),
			OwnedResources: []OwnedResource{
				{
					Secondary: objectType(workloads.SchemeGroupVersion.String(), workloads.Kind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.ServiceKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.SecretKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.ConfigMapKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.PersistentVolumeClaimKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(rbacv1.SchemeGroupVersion.String(), constant.ClusterRoleBindingKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(rbacv1.SchemeGroupVersion.String(), constant.RoleBindingKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.ServiceAccountKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(batchv1.SchemeGroupVersion.String(), constant.JobKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(dpv1alpha1.SchemeGroupVersion.String(), types.BackupKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(dpv1alpha1.SchemeGroupVersion.String(), types.RestoreKind),
					Criteria:  componentCriteria,
				},
				{
					Secondary: objectType(kbappsv1.SchemeGroupVersion.String(), constant.ConfigurationKind),
					Criteria:  componentCriteria,
				},
			},
		},
		{
			Primary: objectType(workloads.SchemeGroupVersion.String(), workloads.Kind),
			OwnedResources: []OwnedResource{
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.PodKind),
					Criteria:  itsCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.ServiceKind),
					Criteria:  itsCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.PersistentVolumeClaimKind),
					Criteria:  itsCriteria,
				},
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.ConfigMapKind),
					Criteria:  itsCriteria,
				},
			},
		},
		{
			Primary: objectType(kbappsv1.SchemeGroupVersion.String(), constant.ConfigurationKind),
			OwnedResources: []OwnedResource{
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.ConfigMapKind),
					Criteria:  configurationCriteria,
				},
			},
		},
		{
			Primary: objectType(dpv1alpha1.SchemeGroupVersion.String(), types.BackupKind),
			OwnedResources: []OwnedResource{
				{
					Secondary: objectType(batchv1.SchemeGroupVersion.String(), constant.JobKind),
					Criteria:  backupCriteria,
				},
				{
					Secondary: objectType(appsv1.SchemeGroupVersion.String(), constant.StatefulSetKind),
					Criteria:  backupCriteria,
				},
				{
					Secondary: objectType(vsv1.SchemeGroupVersion.String(), constant.VolumeSnapshotKind),
					Criteria:  backupCriteria,
				},
				{
					Secondary: objectType(vsv1beta1.SchemeGroupVersion.String(), constant.VolumeSnapshotKind),
					Criteria:  backupCriteria,
				},
			},
		},
		{
			Primary: objectType(dpv1alpha1.SchemeGroupVersion.String(), types.RestoreKind),
			OwnedResources: []OwnedResource{
				{
					Secondary: objectType(batchv1.SchemeGroupVersion.String(), constant.JobKind),
					Criteria:  restoreCriteria,
				},
			},
		},
		{
			Primary: objectType(corev1.SchemeGroupVersion.String(), constant.PersistentVolumeClaimKind),
			OwnedResources: []OwnedResource{
				{
					Secondary: objectType(corev1.SchemeGroupVersion.String(), constant.PersistentVolumeKind),
					Criteria:  pvcCriteria,
				},
			},
		},
	}

	KBOwnershipRules = filterUnsupportedRules(FullKBOwnershipRules)
)

func filterUnsupportedRules(ownershipRules []OwnershipRule) []OwnershipRule {
	var rules []OwnershipRule
	for _, rule := range ownershipRules {
		if exists, _ := resourceExists(rule.Primary.APIVersion, rule.Primary.Kind); !exists {
			continue
		}
		filteredRule := OwnershipRule{
			Primary: rule.Primary,
		}
		for _, ownedResource := range rule.OwnedResources {
			if exists, _ := resourceExists(ownedResource.Secondary.APIVersion, ownedResource.Secondary.Kind); !exists {
				continue
			}
			filteredRule.OwnedResources = append(filteredRule.OwnedResources, ownedResource)
		}
		if len(filteredRule.OwnedResources) > 0 {
			rules = append(rules, filteredRule)
		}
	}
	return rules
}

// resourceExists checks if a resource with the given apiVersion and kind exists in the cluster.
func resourceExists(apiVersion, kind string) (bool, error) {
	// Load the kubeconfig file to get a config object
	config := intctrlutil.GeKubeRestConfig("kubeblocks-api-tester")

	// Create a discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false, fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Parse the apiVersion into a GroupVersion
	_, err = schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse apiVersion: %w", err)
	}

	// Get the API Resources for the given GroupVersion
	apiResources, err := discoveryClient.ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false, nil // GroupVersion does not exist
		}
		return false, fmt.Errorf("failed to get server resources: %w", err)
	}

	// Check if the kind exists in the API Resources
	for _, resource := range apiResources.APIResources {
		if resource.Kind == kind {
			return true, nil
		}
	}

	return false, nil
}

var rootObjectType = viewv1.ObjectType{
	APIVersion: kbappsv1.APIVersion,
	Kind:       kbappsv1.ClusterKind,
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
	// primary - same value as the primary object label with same key.
	// primary.name - the name of the primary object.
	//
	// +optional
	LabelCriteria map[string]string `json:"labelCriteria,omitempty"`

	// SpecifiedNameCriteria specifies the field from which to retrieve the secondary object name.
	//
	// +optional
	SpecifiedNameCriteria *FieldPath `json:"specifiedNameCriteria,omitempty"`

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
