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

package component

import (
	"fmt"
	"maps"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// InstanceFileTemplates derives managed configuration templates for an instance template.
func InstanceFileTemplates(comp *SynthesizedComponent, instance *appsv1.InstanceTemplate) ([]SynthesizedFileTemplate, error) {
	templates := make([]SynthesizedFileTemplate, 0)
	byName := make(map[string]int)
	for _, tpl := range comp.FileTemplates {
		if !tpl.Config {
			continue
		}
		byName[tpl.Name] = len(templates)
		tpl.Variables = maps.Clone(tpl.Variables)
		templates = append(templates, tpl)
	}
	if instance != nil {
		seen := make(map[string]bool)
		for _, config := range instance.Configs {
			name := ptr.Deref(config.Name, "")
			if name == "" {
				return nil, fmt.Errorf("instance template %s: config name is required", instance.Name)
			}
			if seen[name] {
				return nil, fmt.Errorf("instance template %s: duplicated config template %s", instance.Name, name)
			}
			seen[name] = true
			i, ok := byName[name]
			if !ok {
				return nil, fmt.Errorf("instance template %s: config template %s is not defined", instance.Name, name)
			}
			tpl := &templates[i]
			if ptr.Deref(tpl.ExternalManaged, false) {
				return nil, fmt.Errorf("instance template %s: config template %s is externally managed", instance.Name, name)
			}
			if config.ExternalManaged != nil || config.ConfigHash != nil || config.Restart != nil ||
				config.Reconfigure != nil || config.ReconfigureAction != nil || config.ReconfigureArgs != nil {
				return nil, fmt.Errorf("instance template %s: config template %s accepts only name, variables and configMap.name", instance.Name, name)
			}
			if config.ConfigMap != nil {
				source := config.ConfigMap
				if source.Items != nil || source.DefaultMode != nil || source.Optional != nil {
					return nil, fmt.Errorf("instance template %s: config template %s accepts only configMap.name", instance.Name, name)
				}
				if errs := validation.IsDNS1123Subdomain(source.Name); len(errs) > 0 {
					return nil, fmt.Errorf("instance template %s: config template %s has invalid configMap.name %q: %s", instance.Name, name, source.Name, strings.Join(errs, "; "))
				}
				tpl.Template = source.Name
				tpl.Namespace = comp.Namespace
			}
			if len(config.Variables) > 0 {
				if tpl.Variables == nil {
					tpl.Variables = make(map[string]string)
				}
				maps.Copy(tpl.Variables, config.Variables)
			}
		}
	}
	managed := templates[:0]
	for _, tpl := range templates {
		if !ptr.Deref(tpl.ExternalManaged, false) {
			managed = append(managed, tpl)
		}
	}
	return managed, nil
}
