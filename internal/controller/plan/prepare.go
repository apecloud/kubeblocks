/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package plan

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/configfsm"
	"github.com/apecloud/kubeblocks/internal/hsm"
)

// RenderConfigNScriptFiles generates volumes for PodTemplate, volumeMount for container, rendered configTemplate and scriptTemplate,
// and generates configManager sidecar for the reconfigure operation.
// TODO rename this function, this function name is not very reasonable, but there is no suitable name.
func RenderConfigNScriptFiles(clusterVersion *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	component *component.SynthesizedComponent,
	obj client.Object,
	podSpec *corev1.PodSpec,
	localObjs []client.Object,
	ctx context.Context,
	cli client.Client) error {
	// Need to Merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	if len(component.ConfigTemplates) == 0 && len(component.ScriptTemplates) == 0 {
		return nil
	}

	fsmContext := configfsm.NewConfigContext(clusterVersion, cluster, component, obj, podSpec, localObjs, ctx, cli)
	fsm, err := hsm.FromContext(fsmContext, configfsm.ConfigFSMID, configfsm.ConfigFSMSignature)
	if err != nil {
		return err
	}
	return fsm.Fire(configfsm.Creating)

	//clusterName := cluster.Name
	//namespaceName := cluster.Namespace
	//templateBuilder := configuration.newTemplateBuilder(clusterName, namespaceName, cluster, clusterVersion, ctx, cli)
	//// Prepare built-in objects and built-in functions
	//if err := templateBuilder.injectBuiltInObjectsAndFunctions(podSpec, component.ConfigTemplates, component, localObjs); err != nil {
	//	return err
	//}
	//
	//renderWrapper := configuration.newTemplateRenderWrapper(templateBuilder, cluster, ctx, cli)
	//if err := renderWrapper.renderConfigTemplate(cluster, component, localObjs); err != nil {
	//	return err
	//}
	//if err := renderWrapper.renderScriptTemplate(cluster, component, localObjs); err != nil {
	//	return err
	//}
	//
	//if len(renderWrapper.templateAnnotations) > 0 {
	//	configuration.updateResourceAnnotationsWithTemplate(obj, renderWrapper.templateAnnotations)
	//}
	//
	//// Generate Pod Volumes for ConfigMap objects
	//if err := intctrlutil.CreateOrUpdatePodVolumes(podSpec, renderWrapper.volumes); err != nil {
	//	return cfgcore.WrapError(err, "failed to generate pod volume")
	//}
	//
	//if err := configuration.buildConfigManagerWithComponent(podSpec, component.ConfigTemplates, ctx, cli, cluster, component); err != nil {
	//	return cfgcore.WrapError(err, "failed to generate sidecar for configmap's reloader")
	//}
	//// TODO config resource objects are updated by the operator
	//return configuration.createConfigObjects(cli, ctx, renderWrapper.renderedObjs)
}
