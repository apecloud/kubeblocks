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
)

// validate a complete instance template list
// TODO: validate default template's ordinal
// TODO: take offlice instances into consideration
func validateOrdinals(instanceTemplateList []workloads.InstanceTemplate) error {
	ordinalSet := sets.New[int32]()
	for _, tmpl := range instanceTemplateList {
		ordinals := tmpl.Ordinals
		for _, item := range ordinals.Discrete {
			if item < 0 {
				return fmt.Errorf("ordinal(%v) must >= 0", item)
			}
			if ordinalSet.Has(item) {
				return fmt.Errorf("duplicate ordinal(%v)", item)
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
				ordinalSet.Insert(ordinal)
			}
		}
	}
	return nil
}
