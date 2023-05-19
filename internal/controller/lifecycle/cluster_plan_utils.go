/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package lifecycle

import (
	"strings"
)

// mergeAnnotations keeps the original annotations.
// if annotations exist and are replaced, the Deployment/StatefulSet will be updated.
func mergeAnnotations(originalAnnotations map[string]string, targetAnnotations *map[string]string, filters ...func(k, v string) bool) {
	if targetAnnotations == nil {
		return
	}
	if len(originalAnnotations) == 0 {
		return
	}
	if *targetAnnotations == nil {
		*targetAnnotations = map[string]string{}
	}
	for k, v := range originalAnnotations {
		filtered := false
		for _, filter := range filters {
			if filter != nil && filter(k, v) {
				filtered = true
				break
			}
		}
		if filtered {
			continue
		}

		// if the annotation not exist in targetAnnotations, copy it from original.
		if _, ok := (*targetAnnotations)[k]; !ok {
			(*targetAnnotations)[k] = v
			continue
		}
	}
}

// mergeServiceAnnotations keeps the original annotations except prometheus scrape annotations.
// if annotations exist and are replaced, the Service will be updated.
func mergeServiceAnnotations(originalAnnotations map[string]string, targetAnnotations *map[string]string) {
	mergeAnnotations(originalAnnotations, targetAnnotations,
		func(k, v string) bool {
			return strings.HasPrefix(k, "prometheus.io")
		})
}
