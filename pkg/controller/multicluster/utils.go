/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package multicluster

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Enabled4Object(obj client.Object) bool {
	return len(fromObject(obj)) > 0
}

// ObservationContext establishes local provenance for a single-cluster reader without changing routing.
// Multi-cluster readers keep the supplied context, whose missing or ambiguous placement remains unknown.
func ObservationContext(ctx context.Context, reader client.Reader) context.Context {
	switch reader.(type) {
	case *mclient, *clientReader:
		return ctx
	default:
		return IntoContext(ctx, "")
	}
}

// ObjectLocation returns an independently known location. An empty known location is the local cluster.
// Multiple placements and missing provenance cannot identify the cluster of an observed object.
func ObjectLocation(ctx context.Context, obj client.Object) (string, bool) {
	if locations := fromObject(obj); locations != nil {
		if len(locations) == 1 {
			return locations[0], true
		}
		return "", false
	}
	if ctx == nil {
		return "", false
	}
	placement, err := FromContext(ctx)
	if err != nil || strings.Contains(placement, ",") {
		return "", false
	}
	return placement, true
}
