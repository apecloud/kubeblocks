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

package instancetemplate

import (
	"k8s.io/client-go/tools/record"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

type PodNameBuilderOpts struct {
	EventLogger record.EventRecorder
}

// BuildInstanceName2TemplateMap returns the effective template for each desired instance.
func BuildInstanceName2TemplateMap(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (map[string]*InstanceTemplateExt, error) {
	itsExt, err := BuildInstanceSetExt(its, tree)
	if err != nil {
		return nil, err
	}
	opts := &PodNameBuilderOpts{}
	if tree != nil {
		opts.EventLogger = tree.EventRecorder
	}
	builder, err := NewPodNameBuilder(itsExt, opts)
	if err != nil {
		return nil, err
	}
	return builder.BuildInstanceName2TemplateMap()
}

func NewPodNameBuilder(itsExt *InstanceSetExt, opts *PodNameBuilderOpts) (PodNameBuilder, error) {
	if opts == nil {
		opts = &PodNameBuilderOpts{}
	}
	if itsExt.InstanceSet.Spec.FlatInstanceOrdinal {
		return &flatNameBuilder{
			itsExt:      itsExt,
			eventLogger: opts.EventLogger,
		}, nil
	}
	return &defaultNameBuilder{
		itsExt: itsExt,
	}, nil
}
