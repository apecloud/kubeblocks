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

package instanceset

import (
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

// TODO: remove these after extract the Schema of the API types from Kubeblocks into a separate Go package.

// InstanceSetExt serves as a Public Struct,
// used as the type for the input parameters of BuildInstanceTemplateExts.
type InstanceSetExt struct {
	Its               *workloads.InstanceSet
	InstanceTemplates []*workloads.InstanceTemplate
}

// InstanceTemplateExt serves as a Public Struct,
// used as the type for the construction results returned by BuildInstanceTemplateExts.
type InstanceTemplateExt struct {
	Name     string
	Replicas int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

// BuildInstanceName2TemplateMap serves as a Public API, through which users can obtain InstanceName2TemplateMap objects
// processed by the buildInstanceName2TemplateMap function.
func BuildInstanceName2TemplateMap(itsExt *InstanceSetExt) (map[string]*instanceTemplateExt, error) {
	instanceTemplateList := buildInstanceTemplateExts(&instanceSetExt{
		its:               itsExt.Its,
		instanceTemplates: itsExt.InstanceTemplates,
	})
	allNameTemplateMap := make(map[string]*instanceTemplateExt)
	var instanceNameList []string
	for _, template := range instanceTemplateList {
		ordinalList, err := getOrdinalListByTemplateName(itsExt.Its, template.Name)
		if err != nil {
			return nil, err
		}
		instanceNames, err := GenerateInstanceNamesFromTemplate(itsExt.Its.Name, template.Name, template.Replicas, itsExt.Its.Spec.OfflineInstances, ordinalList)
		if err != nil {
			return nil, err
		}
		instanceNameList = append(instanceNameList, instanceNames...)
		for _, name := range instanceNames {
			allNameTemplateMap[name] = template
		}
	}
	// validate duplicate pod names
	getNameFunc := func(n string) string {
		return n
	}
	if err := ValidateDupInstanceNames(instanceNameList, getNameFunc); err != nil {
		return nil, err
	}

	return allNameTemplateMap, nil
}

// BuildInstanceTemplateExts serves as a Public API, through which users can obtain InstanceTemplateExt objects
// processed by the buildInstanceTemplateExts function.
// Its main purpose is to acquire the PodTemplate rendered by InstanceTemplate.
func BuildInstanceTemplateExts(itsExt *InstanceSetExt) []*InstanceTemplateExt {
	itsExts := buildInstanceTemplateExts(&instanceSetExt{
		its:               itsExt.Its,
		instanceTemplates: itsExt.InstanceTemplates,
	})
	var instanceTemplateExts []*InstanceTemplateExt
	for _, itsExt := range itsExts {
		instanceTemplateExts = append(instanceTemplateExts, (*InstanceTemplateExt)(itsExt))
	}
	return instanceTemplateExts
}

// BuildInstanceTemplates serves as a Public API, allowing users to construct InstanceTemplates.
// The constructed InstanceTemplates can be used as part of the input for BuildInstanceTemplateExts.
func BuildInstanceTemplates(totalReplicas int32, instances []workloads.InstanceTemplate, instancesCompressed *corev1.ConfigMap) []*workloads.InstanceTemplate {
	return buildInstanceTemplates(totalReplicas, instances, instancesCompressed)
}
