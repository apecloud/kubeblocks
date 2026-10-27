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

package instanceset

import (
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/workloads/instancereadiness"
)

func reconcileReadiness(tree *kubebuilderx.ObjectTree) (bool, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	assignments, _, err := instancetemplate.BuildAssignments(tree, its)
	health := make(map[string]instancereadiness.Health)
	for _, object := range tree.List(&corev1.Pod{}) {
		pod := object.(*corev1.Pod)
		health[pod.Name] = instancereadiness.Pod(pod, its.Spec.MinReadySeconds)
	}
	return instancereadiness.Publish(its, assignments, health, err)
}
