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

package instancetemplate

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

// validate a complete instance template list
// TODO: take compressed templates into consideration
func validateOrdinals(its *workloads.InstanceSet) error {
	ordinalSet := sets.New[int32]()
	offlineOrdinals := sets.New[int32]()
	for _, name := range its.Spec.OfflineInstances {
		ordinal, err := GetOrdinal(name)
		if err != nil {
			return err
		}
		if offlineOrdinals.Has(ordinal) {
			return fmt.Errorf("duplicate offlineInstance: %v", name)
		}
		offlineOrdinals.Insert(ordinal)
	}
	tpls := make([]workloads.InstanceTemplate, 0, len(its.Spec.Instances)+1)
	tpls = append(tpls, workloads.InstanceTemplate{Ordinals: its.Spec.DefaultTemplateOrdinals})
	tpls = append(tpls, its.Spec.Instances...)
	for _, tmpl := range tpls {
		ordinals := tmpl.Ordinals
		for _, item := range ordinals.Discrete {
			if item < 0 {
				return fmt.Errorf("ordinal(%v) must >= 0", item)
			}
			if ordinalSet.Has(item) {
				return fmt.Errorf("duplicate ordinal(%v)", item)
			}
			if offlineOrdinals.Has(item) {
				return fmt.Errorf("ordinal(%v) exists in offlineInstances", item)
			}
			ordinalSet.Insert(item)
		}

		for _, item := range ordinals.Ranges {
			start := item.Start
			end := item.End

			if start < 0 {
				return fmt.Errorf("ordinal's start(%v) must >= 0", start)
			}

			if start > end {
				return fmt.Errorf("range's end(%v) must >= start(%v)", end, start)
			}

			for ordinal := start; ordinal <= end; ordinal++ {
				if ordinalSet.Has(ordinal) {
					return fmt.Errorf("duplicate ordinal(%v)", item)
				}
				if offlineOrdinals.Has(ordinal) {
					return fmt.Errorf("ordinal(%v) exists in offlineInstances", item)
				}
				ordinalSet.Insert(ordinal)
			}
		}
	}
	return nil
}

func ValidateInstanceTemplates(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) error {
	if err := validateOrdinals(its); err != nil {
		return fmt.Errorf("failed to validate ordinals: %w", err)
	}

	instancesCompressed, err := findTemplateObject(its, tree)
	if err != nil {
		return fmt.Errorf("failed to find compreesssed template: %w", err)
	}

	instanceTemplates := getInstanceTemplates(its.Spec.Instances, instancesCompressed)
	templateNames := sets.New[string]()
	replicasInTemplates := int32(0)
	for _, template := range instanceTemplates {
		replicas := int32(1)
		if template.Replicas != nil {
			replicas = *template.Replicas
		}
		replicasInTemplates += replicas
		if templateNames.Has(template.Name) {
			err = fmt.Errorf("duplicate instance template name: %s", template.Name)
			return err
		}
		templateNames.Insert(template.Name)
	}
	// sum of spec.templates[*].replicas should not greater than spec.replicas
	if replicasInTemplates > *its.Spec.Replicas {
		err = fmt.Errorf("total replicas in instances(%d) should not greater than replicas in spec(%d)", replicasInTemplates, *its.Spec.Replicas)
		return err
	}

	itsExt, err := BuildInstanceSetExt(its, tree)
	if err != nil {
		return fmt.Errorf("failed to build instance set ext: %w", err)
	}

	// try to generate all pod names
	_, err = GenerateAllInstanceNames(itsExt)
	if err != nil {
		return err
	}
	return nil
}
