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

package apps

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

// componentSshTransformer handles the ssh configuration for the component.
type componentSshTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentSshTransformer{}

func (t *componentSshTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	var (
		transCtx        = ctx.(*componentTransformContext)
		compDef         = transCtx.CompDef
		synthesizedComp = transCtx.SynthesizeComponent
	)

	enabled, err := t.enabled(compDef, synthesizedComp)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	if synthesizedComp.TLSConfig.Issuer == nil {
		return fmt.Errorf("issuer shouldn't be nil when tls enabled")
	}

	return nil
}

func (t *componentSshTransformer) enabled(compDef *appsv1.ComponentDefinition, synthesizedComp *component.SynthesizedComponent) (bool, error) {
	if synthesizedComp.TLSConfig == nil || !synthesizedComp.TLSConfig.Enable {
		return false, nil
	}
	if compDef.Spec.TLS == nil {
		return false, fmt.Errorf("the TLS is not supported by the component definition %s", compDef.Name)
	}
	return true, nil
}
