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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
)

type configTemplateBuilder struct {
	namespace    string
	clusterName  string
	templateName string

	// Global Var
	builtInFunctions *gotemplate.BuiltInObjectsFunc
	builtInObjects   *builtInObjects

	podSpec *corev1.PodSpec
	ctx     context.Context
	cli     client.Reader
}

const defaultTemplateName = "KbTemplate"

func newTemplateBuilder(
	clusterName, namespace string,
	ctx context.Context,
	cli client.Reader) *configTemplateBuilder {
	return &configTemplateBuilder{
		namespace:    namespace,
		clusterName:  clusterName,
		templateName: defaultTemplateName,
		ctx:          ctx,
		cli:          cli,
	}
}

func (c *configTemplateBuilder) setTemplateName(templateName string) {
	c.templateName = templateName
}

func (c *configTemplateBuilder) formatError(file string, err error) error {
	return fmt.Errorf("failed to render configuration template[cm:%s][key:%s], error: [%v]", c.templateName, file, err)
}

func (c *configTemplateBuilder) render(configs map[string]string) (map[string]string, error) {
	values, err := builtinObjectsAsValues(c.builtInObjects)
	if err != nil {
		return nil, err
	}

	rendered := make(map[string]string, len(configs))
	engine := gotemplate.NewTplEngine(values, c.builtInFunctions, c.templateName, c.cli, c.ctx)
	for file, configContext := range configs {
		newContext, err := engine.Render(configContext)
		if err != nil {
			return nil, c.formatError(file, err)
		}
		rendered[file] = newContext
	}
	return rendered, nil
}

func (c *configTemplateBuilder) injectBuiltInObjectsAndFunctions(
	podSpec *corev1.PodSpec,
	component *component.SynthesizedComponent,
	localObjs []client.Object,
	cluster *appsv1.Cluster) {
	c.podSpec = podSpec
	c.builtInFunctions = BuiltInCustomFunctions(c, component, localObjs)
	c.builtInObjects = buildInComponentObjects(podSpec, component, cluster)
}
