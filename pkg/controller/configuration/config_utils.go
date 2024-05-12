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

package configuration

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func createConfigObjects(cli client.Client, ctx context.Context, objs []client.Object) error {
	for _, obj := range objs {
		if err := cli.Create(ctx, obj, inDataContext()); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			// for update script cm
			if core.IsSchedulableConfigResource(obj) {
				continue
			}
			if err := cli.Update(ctx, obj, inDataContext()); err != nil {
				return err
			}
		}
	}
	return nil
}

func GetConfigManagerGRPCPort(containers []corev1.Container) (int32, error) {
	for _, container := range containers {
		if found := foundPortByConfigManagerPortName(container); found != nil {
			return found.ContainerPort, nil
		}
	}
	return -1, core.MakeError("failed to find config manager grpc port, please add named config-manager port")
}

func foundPortByConfigManagerPortName(container corev1.Container) *corev1.ContainerPort {
	for _, port := range container.Ports {
		if port.Name == constant.ConfigManagerPortName {
			return &port
		}
	}
	return nil
}

// UpdateConfigPayload updates the configuration payload
func UpdateConfigPayload(config *appsv1alpha1.ConfigurationSpec, component *component.SynthesizedComponent) (bool, error) {
	updated := false
	for i := range config.ConfigItemDetails {
		configSpec := &config.ConfigItemDetails[i]
		// check v-scale operation
		if enableVScaleTrigger(configSpec.ConfigSpec) {
			resourcePayload := intctrlutil.ResourcesPayloadForComponent(component.Resources)
			ret, err := intctrlutil.CheckAndPatchPayload(configSpec, constant.ComponentResourcePayload, resourcePayload)
			if err != nil {
				return false, err
			}
			updated = updated || ret
		}
		// check h-scale operation
		if enableHScaleTrigger(configSpec.ConfigSpec) {
			ret, err := intctrlutil.CheckAndPatchPayload(configSpec, constant.ReplicasPayload, component.Replicas)
			if err != nil {
				return false, err
			}
			updated = updated || ret
		}
	}
	return updated, nil
}

func validRerenderResources(configSpec *appsv1alpha1.ComponentConfigSpec) bool {
	return configSpec != nil && len(configSpec.ReRenderResourceTypes) != 0
}

func enableHScaleTrigger(configSpec *appsv1alpha1.ComponentConfigSpec) bool {
	return validRerenderResources(configSpec) && slices.Contains(configSpec.ReRenderResourceTypes, appsv1alpha1.ComponentHScaleType)
}

func enableVScaleTrigger(configSpec *appsv1alpha1.ComponentConfigSpec) bool {
	return validRerenderResources(configSpec) && slices.Contains(configSpec.ReRenderResourceTypes, appsv1alpha1.ComponentVScaleType)
}

func configSetFromComponent(templates []appsv1alpha1.ComponentConfigSpec) []string {
	configSet := make([]string, 0)
	for _, template := range templates {
		configSet = append(configSet, template.Name)
	}
	return configSet
}
