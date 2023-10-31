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

package apps

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// SecretTransformer puts all the secrets at the beginning of the DAG
type SecretTransformer struct{}

var _ graph.Transformer = &SecretTransformer{}

func (c *SecretTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	graphCli, _ := transCtx.Client.(model.GraphClient)

	var secrets, noneClusterObjects []client.Object
	secrets = graphCli.FindAll(dag, &corev1.Secret{})
	noneClusterObjects = graphCli.FindAll(dag, &appsv1alpha1.Cluster{}, model.HaveDifferentTypeWithOption)
	for _, secret := range secrets {
		if graphCli.IsAction(dag, secret, model.ActionUpdatePtr()) {
			graphCli.Noop(dag, secret)
		}
		for _, object := range noneClusterObjects {
			// manipulate all secrets first
			if _, ok := object.(*corev1.Secret); !ok {
				if !graphCli.IsAction(dag, secret, model.ActionDeletePtr()) {
					graphCli.DependOn(dag, object, secret)
				}
			}
		}
	}
	return nil
}
