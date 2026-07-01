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

package instanceset2

import (
	"slices"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func buildDesiredInstancesByName(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) (map[string]*workloads.Instance, []string, error) {
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return nil, nil, err
	}
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		return nil, nil, err
	}
	nameMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return nil, nil, err
	}

	names := make([]string, 0, len(nameMap))
	for name := range nameMap {
		names = append(names, name)
	}
	slices.Sort(names)

	desired := make(map[string]*workloads.Instance, len(names))
	for _, name := range names {
		inst, err := buildInstanceByTemplate(tree, name, nameMap[name], its)
		if err != nil {
			return nil, nil, err
		}
		desired[name] = inst
	}
	return desired, names, nil
}
