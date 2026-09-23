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

package operations

import (
	"fmt"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// volumeExpansionTargetKey identifies a volume expansion target. An empty
// InstanceTemplateName identifies the component's default volume template.
type volumeExpansionTargetKey struct {
	InstanceTemplateName string
	VCTName              string
}

// volumeExpansionTarget is a normalized volume expansion target.
type volumeExpansionTarget struct {
	Key              volumeExpansionTargetKey
	RequestedStorage resource.Quantity
}

// volumeExpansionPlan is the normalized intent of a VolumeExpansion request.
// Instance-template targets take precedence over component targets with the
// same volume template name.
type volumeExpansionPlan struct {
	Targets map[volumeExpansionTargetKey]volumeExpansionTarget
}

// normalizeVolumeExpansion validates and normalizes volume expansion intent
// against the current component desired state. It does not perform client
// backed checks such as StorageClass validation.
func normalizeVolumeExpansion(request opsv1alpha1.VolumeExpansion, spec *appsv1.ClusterComponentSpec) (volumeExpansionPlan, error) {
	plan := volumeExpansionPlan{Targets: map[volumeExpansionTargetKey]volumeExpansionTarget{}}
	if spec == nil {
		return plan, fmt.Errorf("component spec is nil")
	}
	defaults := make(map[string]appsv1.PersistentVolumeClaimTemplate, len(spec.VolumeClaimTemplates))
	for _, vct := range spec.VolumeClaimTemplates {
		defaults[vct.Name] = vct
	}
	add := func(scope string, key volumeExpansionTargetKey, v opsv1alpha1.OpsRequestVolumeClaimTemplate, current *appsv1.PersistentVolumeClaimTemplate) error {
		if _, ok := plan.Targets[key]; ok {
			return fmt.Errorf("duplicate volume expansion target %s/%s in %s", key.InstanceTemplateName, key.VCTName, scope)
		}
		if current == nil {
			return fmt.Errorf("volumeClaimTemplate %q not found in %s", v.Name, scope)
		}
		declared := current.Spec.Resources.Requests[corev1.ResourceStorage]
		if declared.IsZero() {
			return fmt.Errorf("volumeClaimTemplate %q in %s has no declared storage", v.Name, scope)
		}
		if v.Storage.Cmp(declared) < 0 {
			return fmt.Errorf("requested storage for %s/%s cannot be less than declared size %s", scope, v.Name, declared.String())
		}
		plan.Targets[key] = volumeExpansionTarget{Key: key, RequestedStorage: v.Storage}
		return nil
	}
	for _, v := range request.VolumeClaimTemplates {
		current, ok := defaults[v.Name]
		if !ok {
			return plan, fmt.Errorf("volumeClaimTemplate %q not found in %s", v.Name, spec.Name)
		}
		if err := add(spec.Name, volumeExpansionTargetKey{VCTName: v.Name}, v, &current); err != nil {
			return plan, err
		}
	}
	instances := make(map[string]appsv1.InstanceTemplate, len(spec.Instances))
	for _, instance := range spec.Instances {
		instances[instance.Name] = instance
	}
	for _, instanceRequest := range request.Instances {
		instance, ok := instances[instanceRequest.Name]
		if !ok {
			return plan, fmt.Errorf("instance %q not found in %s", instanceRequest.Name, spec.Name)
		}
		effective := make(map[string]appsv1.PersistentVolumeClaimTemplate, len(defaults)+len(instance.VolumeClaimTemplates))
		for name, vct := range defaults {
			effective[name] = vct
		}
		for _, vct := range instance.VolumeClaimTemplates {
			effective[vct.Name] = vct
		}
		seen := map[string]struct{}{}
		for _, v := range instanceRequest.VolumeClaimTemplates {
			if _, exists := seen[v.Name]; exists {
				return plan, fmt.Errorf("duplicate volume expansion target %s/%s", instanceRequest.Name, v.Name)
			}
			seen[v.Name] = struct{}{}
			key := volumeExpansionTargetKey{InstanceTemplateName: instanceRequest.Name, VCTName: v.Name}
			if _, exists := plan.Targets[key]; exists {
				return plan, fmt.Errorf("duplicate volume expansion target %s/%s", instanceRequest.Name, v.Name)
			}
			current, exists := effective[v.Name]
			if !exists {
				return plan, fmt.Errorf("volumeClaimTemplate %q not found in instance %s", v.Name, instanceRequest.Name)
			}
			if err := add("instance "+instanceRequest.Name, key, v, &current); err != nil {
				return plan, err
			}
		}
	}
	return plan, nil
}

// effectiveVolumeClaimTemplate returns the instance override when present and
// otherwise the component default.
func effectiveVolumeClaimTemplate(spec *appsv1.ClusterComponentSpec, instanceName, vctName string) (*appsv1.PersistentVolumeClaimTemplate, bool) {
	for _, instance := range spec.Instances {
		if instance.Name != instanceName {
			continue
		}
		for _, vct := range instance.VolumeClaimTemplates {
			if vct.Name == vctName {
				return vct.DeepCopy(), true
			}
		}
	}
	for _, vct := range spec.VolumeClaimTemplates {
		if vct.Name == vctName {
			return vct.DeepCopy(), true
		}
	}
	return nil, false
}
