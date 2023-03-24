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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type statefulComponent struct {
	componentBase
}

type statefulComponentBuilder struct {
	componentBuilderBase
	workload *appsv1.StatefulSet
}

func (b *statefulComponentBuilder) mutableWorkload(_ int32) client.Object {
	return b.workload
}

func (b *statefulComponentBuilder) mutablePodSpec(_ int32) *corev1.PodSpec {
	return &b.workload.Spec.Template.Spec
}

func (b *statefulComponentBuilder) buildWorkload(_ int32) componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.EnvConfig == nil {
			return nil, fmt.Errorf("build consensus workload but env config is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		sts, err := builder.BuildStsLow(b.ReqCtx, b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent(), b.EnvConfig.Name)
		if err != nil {
			return nil, err
		}

		b.workload = sts

		return nil, nil // don't return deploy here, and it will not add to resource queue now
	}
	return b.buildWrapper(buildfn)
}

func (c *statefulComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, action *Action) error {
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, cli, *c.Cluster, *c.Definition, *c.CompDef, *c.CompSpec, c.CompVer)
	if err != nil {
		return err
	}
	c.Component = synthesizedComp

	builder := &statefulComponentBuilder{
		componentBuilderBase: componentBuilderBase{
			ReqCtx:        reqCtx,
			Client:        cli,
			Comp:          c,
			defaultAction: action,
			Error:         nil,
			EnvConfig:     nil,
		},
		workload: nil,
	}
	builder.concreteBuilder = builder

	// runtime, config, script, env, volume, service, monitor, probe
	return builder.buildEnv(). // TODO: workload related, scaling related
					buildWorkload(0). // build workload here since other objects depend on it.
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

func (c *statefulComponent) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := listStsOwnedByComponent(reqCtx, cli, c.Cluster.Namespace, c.Cluster.Name, c.Component.Name); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *statefulComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, actionPtr(CREATE)); err != nil {
		return err
	}
	if exist, err := c.Exist(reqCtx, cli); err != nil || exist {
		if err != nil {
			return err
		}
		return fmt.Errorf("component to be created is already exist, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}
	return nil
}

func (c *statefulComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO: delete component managed resources
	return nil
}

func (c *statefulComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, nil); err != nil {
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

	return c.updateUnderlayResources(reqCtx, cli)
}

func (c *statefulComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO: impl, share with consensus component
	return nil
}

func (c *statefulComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO: impl, share with consensus component
	return nil
}

func (c *statefulComponent) runningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*appsv1.StatefulSet, error) {
	stsList, err := listStsOwnedByComponent(reqCtx, cli, c.Cluster.Namespace, c.Cluster.Name, c.Component.Name)
	if err != nil {
		return nil, err
	}

	cnt := len(stsList)
	if cnt == 0 {
		return nil, fmt.Errorf("no workload found for the consensus component, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	} else if cnt > 1 {
		return nil, fmt.Errorf("more than one workloads found for the consensus component, cluster: %s, component: %s, cnt: %d",
			c.Cluster.Name, c.CompSpec.Name, cnt)
	}

	sts := stsList[0]
	if sts.Spec.Replicas == nil {
		return nil, fmt.Errorf("running workload for the consensus component has no replica, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	return sts, nil
}

func (c *statefulComponent) updateUnderlayResources(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.updateWorkload(reqCtx, cli); err != nil {
		return err
	}
	if err := c.updateService(reqCtx, cli); err != nil {
		return err
	}
	return nil
}

func (c *statefulComponent) updateWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}
	return c.updateStatefulSetWorkload(stsObj, 0)
}
