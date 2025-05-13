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
	"errors"
	"reflect"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

type PodNameBuilderOpts struct {
	// when useing combinedPodNameBuilder, status field is necessary to build correct names.
	// use this carefully
	AllowEmptyStatus bool
}

// NewPodNameBuilder returns a PodNameBuilder based on the InstanceSet's PodNamingRule.
// When the PodNamingRule is Combined, it should be a instanceset returned by kubernetes (i.e. with status field included)
func NewPodNameBuilder(itsExt *InstanceSetExt, opts *PodNameBuilderOpts) (PodNameBuilder, error) {
	if opts == nil {
		opts = &PodNameBuilderOpts{}
	}
	switch itsExt.InstanceSet.Spec.PodNamingRule {
	case workloads.PodNamingRuleCombined:
		// validate status is not empty
		if !opts.AllowEmptyStatus && reflect.ValueOf(itsExt.InstanceSet.Status).IsZero() {
			return nil, errors.New("instanceset status is empty")
		}
		return &combinedPodNameBuilder{
			itsExt: itsExt,
		}, nil
	// default to separated naming rule, since it's the old behavior
	default:
		return &separatedPodNameBuilder{
			itsExt: itsExt,
		}, nil
	}
}
