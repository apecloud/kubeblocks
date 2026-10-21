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
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	corev1 "k8s.io/api/core/v1"
)

// This opt-in policy limits component-level vars injected into the exporter.
// Unannotated definitions retain their existing behavior. Explicit runtime env
// and volumes are still controlled by the ComponentDefinition author.
const exporterEnvVarsAnnotation = "apps.kubeblocks.io/exporter-env-vars"

func exporterEnvSelection(def *appsv1.ComponentDefinition, vars []corev1.EnvVar) (string, []corev1.EnvVar, error) {
	raw, enabled := def.Annotations[exporterEnvVarsAnnotation]
	if !enabled {
		return "", nil, nil
	}
	exporter := component.GetExporter(def.Spec)
	if exporter == nil || exporter.ContainerName == "" {
		return "", nil, fmt.Errorf("%s requires an exporter container", exporterEnvVarsAnnotation)
	}
	var names []string
	if len(raw) > 8192 || !strings.HasPrefix(strings.TrimSpace(raw), "[") || json.Unmarshal([]byte(raw), &names) != nil || len(names) > 128 {
		return "", nil, fmt.Errorf("%s must be a JSON array of at most 128 variable names", exporterEnvVarsAnnotation)
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" || allowed[name] {
			return "", nil, fmt.Errorf("%s contains an empty or duplicate name", exporterEnvVarsAnnotation)
		}
		allowed[name] = true
	}
	selected := make([]corev1.EnvVar, 0, len(names))
	found := make(map[string]bool, len(names))
	for _, v := range vars {
		if allowed[v.Name] {
			if found[v.Name] {
				return "", nil, fmt.Errorf("duplicate selected exporter variable %s", v.Name)
			}
			selected = append(selected, v)
			found[v.Name] = true
		}
	}
	for name := range allowed {
		if !found[name] {
			return "", nil, fmt.Errorf("exporter variable %s did not resolve", name)
		}
	}
	return exporter.ContainerName, selected, nil
}

func injectSelectedExporterEnv(comp *component.SynthesizedComponent, vars []corev1.EnvVar, sources []corev1.EnvFromSource, exporter string, selected []corev1.EnvVar) {
	if exporter == "" {
		component.InjectEnvVars(comp, vars, sources)
		return
	}
	component.InjectEnvVars4Containers(comp, vars, sources, func(c *corev1.Container) bool { return c.Name != exporter })
	// Do not inherit the component-wide env ConfigMap. Selected non-secret vars
	// are passed directly; selected credentials retain SecretKeyRef references.
	component.InjectEnvVars4Containers(comp, selected, nil, func(c *corev1.Container) bool { return c.Name == exporter })
}
