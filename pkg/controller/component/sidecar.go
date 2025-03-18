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

package component

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func buildSidecars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, comp *appsv1.Component) error {
	for _, sidecar := range comp.Spec.Sidecars {
		if err := buildSidecar(ctx, cli, synthesizedComp, comp, sidecar); err != nil {
			return err
		}
	}
	return nil
}

func buildSidecar(ctx context.Context, cli client.Reader,
	synthesizedComp *SynthesizedComponent, comp *appsv1.Component, sidecar appsv1.Sidecar) error {
	sidecarDef, err := getNCheckSidecarDefinition(ctx, cli, sidecar.SidecarDef)
	if err != nil {
		return err
	}
	for _, builder := range []func(*SynthesizedComponent, *appsv1.SidecarDefinition, *appsv1.Component) error{
		buildSidecarContainers,
		buildSidecarVars,
		buildSidecarConfigs,
		buildSidecarScripts,
	} {
		if err := builder(synthesizedComp, sidecarDef, comp); err != nil {
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

func buildSidecarContainers(synthesizedComp *SynthesizedComponent, sidecarDef *appsv1.SidecarDefinition, _ *appsv1.Component) error {
	synthesizedComp.PodSpec.Containers = append(synthesizedComp.PodSpec.Containers, sidecarDef.Spec.Containers...)
	return nil
}

func buildSidecarVars(synthesizedComp *SynthesizedComponent, sidecarDef *appsv1.SidecarDefinition, _ *appsv1.Component) error {
	if sidecarDef.Spec.Vars != nil {
		if synthesizedComp.SidecarVars == nil {
			synthesizedComp.SidecarVars = make([]appsv1.EnvVar, 0)
		}
		// already checked for duplication in the definition controller
		synthesizedComp.SidecarVars = append(synthesizedComp.SidecarVars, sidecarDef.Spec.Vars...)
	}
	return nil
}

func buildSidecarConfigs(synthesizedComp *SynthesizedComponent, sidecarDef *appsv1.SidecarDefinition, comp *appsv1.Component) error {
	for _, tpl := range sidecarDef.Spec.Configs {
		synthesizedComp.FileTemplates = append(synthesizedComp.FileTemplates, synthesizeFileTemplate(comp, tpl, true))
	}
	return nil
}

func buildSidecarScripts(synthesizedComp *SynthesizedComponent, sidecarDef *appsv1.SidecarDefinition, comp *appsv1.Component) error {
	for _, tpl := range sidecarDef.Spec.Scripts {
		synthesizedComp.FileTemplates = append(synthesizedComp.FileTemplates, synthesizeFileTemplate(comp, tpl, false))
	}
	return nil
}
