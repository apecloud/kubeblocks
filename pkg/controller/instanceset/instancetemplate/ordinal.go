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
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// ConvertOrdinalsToSet assumes oridnals are valid
func ConvertOrdinalsToSet(ordinals kbappsv1.Ordinals) sets.Set[int32] {
	ordinalSet := sets.New(ordinals.Discrete...)
	for _, item := range ordinals.Ranges {
		for ordinal := item.Start; ordinal <= item.End; ordinal++ {
			ordinalSet.Insert(ordinal)
		}
	}
	return ordinalSet
}

// ConvertOrdinalsToSortedList assumes oridnals are valid
func ConvertOrdinalsToSortedList(ordinals kbappsv1.Ordinals) []int32 {
	ordinalSet := ConvertOrdinalsToSet(ordinals)
	sortedOrdinalList := ordinalSet.UnsortedList()
	slices.Sort(sortedOrdinalList)
	return sortedOrdinalList
}

func convertOrdinalSetToSortedList(ordinalSet sets.Set[int32]) []int32 {
	sortedOrdinalList := ordinalSet.UnsortedList()
	slices.Sort(sortedOrdinalList)
	return sortedOrdinalList
}

func GetOrdinal(podName string) (int32, error) {
	index := strings.LastIndex(podName, "-")
	if index < 0 {
		return -1, fmt.Errorf("failed to get ordinal from pod %v", podName)
	}
	ordinalStr := podName[index+1:]
	ordinal, err := strconv.ParseInt(ordinalStr, 10, 32)
	if err != nil {
		return -1, err
	}
	return int32(ordinal), nil
}

func GetInstanceName(its *workloads.InstanceSet, ordinal int32) string {
	return fmt.Sprintf("%v-%v", its.Name, ordinal)
}

// TODO: Merge this to GetOrdinal
// ParseParentNameAndOrdinal parses parent (instance template) Name and ordinal from the give instance name.
// -1 will be returned if no numeric suffix contained.
func ParseParentNameAndOrdinal(s string) (string, int) {
	parent := s
	ordinal := -1

	index := strings.LastIndex(s, "-")
	if index < 0 {
		return parent, ordinal
	}
	ordinalStr := s[index+1:]
	if i, err := strconv.ParseInt(ordinalStr, 10, 32); err == nil {
		ordinal = int(i)
		parent = s[:index]
	}
	return parent, ordinal
}
