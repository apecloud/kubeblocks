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
	"k8s.io/client-go/tools/record"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type PodNameBuilderOpts struct {
	EventLogger record.EventRecorder
}

// NewPodNameBuilder returns a PodNameBuilder based on the InstanceSet's PodNamingRule.
// When the PodNamingRule is Combined, it should be a instanceset returned by kubernetes (i.e. with status field included)
func NewPodNameBuilder(itsExt *InstanceSetExt, opts *PodNameBuilderOpts) (PodNameBuilder, error) {
	if opts == nil {
		opts = &PodNameBuilderOpts{}
	}
	switch itsExt.InstanceSet.Spec.PodNamingRule {
	case kbappsv1.PodNamingRuleCombined:
		return &combinedPodNameBuilder{
			itsExt:      itsExt,
			eventLogger: opts.EventLogger,
		}, nil
	// default to separated naming rule, since it's the old behavior
	default:
		return &separatedPodNameBuilder{
			itsExt: itsExt,
		}, nil
	}
}
