/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
