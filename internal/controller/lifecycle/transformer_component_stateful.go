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
	"github.com/apecloud/kubeblocks/internal/controller/graph"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func newStatefulComponent(definition *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compVer *appsv1alpha1.ClusterComponentVersion,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) *statefulComponent {
	return &statefulComponent{
		statefulsetComponentBase: statefulsetComponentBase{
			componentBase: componentBase{
				Definition:      definition,
				Cluster:         cluster,
				CompDef:         compDef,
				CompVer:         compVer,
				CompSpec:        compSpec,
				Component:       nil,
				workloadVertexs: make([]*lifecycleVertex, 0),
				dag:             dag,
			},
		},
	}
}

type statefulComponent struct {
	statefulsetComponentBase
}

type statefulComponentWorkloadBuilder struct {
	componentWorkloadBuilderBase
	workload *appsv1.StatefulSet
}

func (b *statefulComponentWorkloadBuilder) mutableWorkload(_ int32) client.Object {
	return b.workload
}

func (b *statefulComponentWorkloadBuilder) mutableRuntime(_ int32) *corev1.PodSpec {
	return &b.workload.Spec.Template.Spec
}

func (b *statefulComponentWorkloadBuilder) buildWorkload(_ int32) componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.envConfig == nil {
			return nil, fmt.Errorf("build consensus workload but env config is nil, cluster: %s, component: %s",
				b.comp.GetClusterName(), b.comp.GetName())
		}

		sts, err := builder.BuildStsLow(b.reqCtx, b.comp.GetCluster(), b.comp.GetSynthesizedComponent(), b.envConfig.Name)
		if err != nil {
			return nil, err
		}

		b.workload = sts

		return nil, nil // don't return deploy here
	}
	return b.buildWrapper(buildfn)
}

func (c *statefulComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, action *Action) error {
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, cli, *c.Cluster, *c.Definition, *c.CompDef, *c.CompSpec, c.CompVer)
	if err != nil {
		return err
	}
	c.Component = synthesizedComp

	builder := &statefulComponentWorkloadBuilder{
		componentWorkloadBuilderBase: componentWorkloadBuilderBase{
			reqCtx:        reqCtx,
			client:        cli,
			comp:          c,
			defaultAction: action,
			error:         nil,
			envConfig:     nil,
		},
		workload: nil,
	}
	builder.concreteBuilder = builder

	return builder.buildEnv().
		buildWorkload(0).
		buildHeadlessService().
		buildConfig(0).
		buildTLSVolume(0).
		buildVolumeMount(0).
		buildService().
		buildTLSCert().
		complete()
}

func (c *statefulComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Stateful
}

func (c *statefulComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, actionPtr(CREATE)); err != nil {
		return err
	}

	// do a double check
	if exist, err := c.Exist(reqCtx, cli); err != nil || exist {
		if err != nil {
			return err
		}
		return fmt.Errorf("component to be created is already exist, cluster: %s, component: %s",
			c.GetClusterName(), c.GetName())
	}

	return c.validateObjectsAction()
}

func (c *statefulComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, nil); err != nil {
		return err
	}

	if err := c.Restart(reqCtx, cli); err != nil {
		return err
	}

	// cluster.spec.componentSpecs[*].volumeClaimTemplates[*].spec.resources.requests[corev1.ResourceStorage]
	if err := c.ExpandVolume(reqCtx, cli); err != nil {
		return err
	}

	// cluster.spec.componentSpecs[*].replicas
	if err := c.HorizontalScale(reqCtx, cli); err != nil {
		return err
	}

	if err := c.updateUnderlyingResources(reqCtx, cli); err != nil {
		return err
	}

	return c.resolveObjectsAction(reqCtx, cli)
}

func (c *statefulComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(refactor): delete component owned resources
	return nil
}
