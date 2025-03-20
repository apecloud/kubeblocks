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
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

const (
	// used to specify the configmap which stores the compressed instance template
	TemplateRefAnnotationKey = "kubeblocks.io/template-ref"
	TemplateRefDataKey       = "instances"

	TemplateNameLabelKey = "workloads.kubeblocks.io/template-name"
)

type InstanceSetExt struct {
	InstanceSet       *workloads.InstanceSet
	InstanceTemplates map[string]*workloads.InstanceTemplate // key is template name
}

// InstanceTemplateExt merges the default podSpec with overrides in the template
type InstanceTemplateExt struct {
	Name     string
	Replicas int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}
