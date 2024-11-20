/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func buildSidecars(ctx context.Context, cli client.Reader, comp *appsv1.Component, synthesizedComp *SynthesizedComponent) error {
	for _, sidecar := range comp.Spec.Sidecars {
		if err := buildSidecar(ctx, cli, sidecar, synthesizedComp); err != nil {
			return err
		}
	}
	return nil
}

func buildSidecar(ctx context.Context, cli client.Reader, sidecar appsv1.Sidecar, synthesizedComp *SynthesizedComponent) error {
	sidecarDef, err := getNCheckSidecarDefinition(ctx, cli, sidecar.SidecarDef)
	if err != nil {
		return err
	}
	for _, builder := range []func(*appsv1.SidecarDefinition, *SynthesizedComponent) error{
		buildSidecarContainers,
		buildSidecarVars,
		buildSidecarConfigs,
		buildSidecarScripts,
	} {
		if err := builder(sidecarDef, synthesizedComp); err != nil {
			return err
		}
	}
	return nil
}

func getNCheckSidecarDefinition(ctx context.Context, cli client.Reader, name string) (*appsv1.SidecarDefinition, error) {
	sidecarDefKey := types.NamespacedName{
		Name: name,
	}
	sidecarDef := &appsv1.SidecarDefinition{}
	if err := cli.Get(ctx, sidecarDefKey, sidecarDef); err != nil {
		return nil, err
	}
	if sidecarDef.Generation != sidecarDef.Status.ObservedGeneration {
		return nil, fmt.Errorf("the referenced SidecarDefinition is not up to date: %s", sidecarDef.Name)
	}
	if sidecarDef.Status.Phase != appsv1.AvailablePhase {
		return nil, fmt.Errorf("the referenced SidecarDefinition is unavailable: %s", sidecarDef.Name)
	}
	return sidecarDef, nil
}

func buildSidecarContainers(sidecarDef *appsv1.SidecarDefinition, synthesizedComp *SynthesizedComponent) error {
	synthesizedComp.PodSpec.Containers = append(synthesizedComp.PodSpec.Containers, sidecarDef.Spec.Containers...)
	return nil
}

func buildSidecarVars(sidecarDef *appsv1.SidecarDefinition, synthesizedComp *SynthesizedComponent) error {
	if sidecarDef.Spec.Vars != nil {
		if synthesizedComp.SidecarVars == nil {
			synthesizedComp.SidecarVars = make([]appsv1.EnvVar, 0)
		}
		// already checked for duplication in the definition controller
		synthesizedComp.SidecarVars = append(synthesizedComp.SidecarVars, sidecarDef.Spec.Vars...)
	}
	return nil
}

func buildSidecarConfigs(sidecarDef *appsv1.SidecarDefinition, synthesizedComp *SynthesizedComponent) error {
	if sidecarDef.Spec.Configs != nil {
		templateToConfig := func() []appsv1.ComponentConfigSpec {
			if len(sidecarDef.Spec.Configs) == 0 {
				return nil
			}
			l := make([]appsv1.ComponentConfigSpec, 0)
			for i := range sidecarDef.Spec.Configs {
				l = append(l, appsv1.ComponentConfigSpec{ComponentTemplateSpec: sidecarDef.Spec.Configs[i]})
			}
			return l
		}
		synthesizedComp.ConfigTemplates = append(synthesizedComp.ConfigTemplates, templateToConfig()...)
	}
	return nil
}

func buildSidecarScripts(sidecarDef *appsv1.SidecarDefinition, synthesizedComp *SynthesizedComponent) error {
	if sidecarDef.Spec.Scripts != nil {
		synthesizedComp.ScriptTemplates = append(synthesizedComp.ScriptTemplates, sidecarDef.Spec.Scripts...)
	}
	return nil
}
