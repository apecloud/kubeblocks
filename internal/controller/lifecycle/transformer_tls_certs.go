/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package lifecycle

import (
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type tlsCertsTransformer struct {
	cr  clusterRefResources
	cli client.ReadonlyClient
	ctx intctrlutil.RequestCtx
}

func (t *tlsCertsTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	origCluster, _ := rootVertex.oriObj.(*appsv1alpha1.Cluster)
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)
	// return fast when cluster is deleting
	if isClusterDeleting(*origCluster) {
		return nil
	}

	var secretList []*corev1.Secret
	for _, comp := range cluster.Spec.ComponentSpecs {
		if !comp.TLS {
			continue
		}
		if comp.Issuer == nil {
			return errors.New("issuer shouldn't be nil when tls enabled")
		}

		switch comp.Issuer.Name {
		case appsv1alpha1.IssuerUserProvided:
			if err := plan.CheckTLSSecretRef(t.ctx, t.cli, cluster.Namespace, comp.Issuer.SecretRef); err != nil {
				return err
			}
		case appsv1alpha1.IssuerKubeBlocks:
			secret, err := plan.ComposeTLSSecret(cluster.Namespace, cluster.Name, comp.Name)
			if err != nil {
				return err
			}
			secretList = append(secretList, secret)
		}
	}

	root := dag.Root()
	if root == nil {
		return fmt.Errorf("root vertex not found: %v", dag)
	}
	for _, secret := range secretList {
		vertex := &lifecycleVertex{obj: secret}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
	}

	return nil
}
