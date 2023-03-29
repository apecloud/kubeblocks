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

type statelessComponent struct {
	componentBase
}

type statelessComponentBuilder struct {
	componentBuilderBase
	workload *appsv1.Deployment
}

func (b *statelessComponentBuilder) mutableWorkload(_ int32) client.Object {
	return b.workload
}

func (b *statelessComponentBuilder) mutablePodSpec(_ int32) *corev1.PodSpec {
	return &b.workload.Spec.Template.Spec
}

func (b *statelessComponentBuilder) buildWorkload(_ int32) componentBuilder {
	buildfn := func() ([]client.Object, error) {
		deploy, err := builder.BuildDeployLow(b.reqCtx, b.comp.GetCluster(), b.comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}

		b.workload = deploy

		return nil, nil // don't return deploy here, and it will not add to resource queue now
	}
	return b.buildWrapper(buildfn)
}

func (c *statelessComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, action *Action) error {
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, cli, *c.Cluster, *c.Definition, *c.CompDef, *c.CompSpec, c.CompVer)
	if err != nil {
		return err
	}
	c.Component = synthesizedComp

	builder := &statelessComponentBuilder{
		componentBuilderBase: componentBuilderBase{
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

	// runtime, config, script, env, volume, service, monitor, probe
	return builder.buildEnv(). // TODO: workload & scaling related
					buildWorkload(0). // build workload here since other objects depend on it.
					buildHeadlessService().
					buildConfig(0).
					buildTLSVolume(0).
					buildVolumeMount(0).
					buildService().
					buildTLSCert().
					complete()
}

func (c *statelessComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Stateless
}

func (c *statelessComponent) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := listDeployOwnedByComponent(reqCtx, cli, c.GetNamespace(), c.GetMatchingLabels()); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *statelessComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

	return c.validateObjectsAction()
}

func (c *statelessComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO: delete component owned resources
	return nil
}

func (c *statelessComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

func (c *statelessComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *statelessComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	deploy, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}
	if *deploy.Spec.Replicas != c.Component.Replicas {
		reqCtx.Recorder.Eventf(c.Cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"start horizontal scale component %s of cluster %s from %d to %d",
			c.GetName(), c.GetClusterName(), *deploy.Spec.Replicas, c.Component.Replicas)
	}
	return nil
}

func (c *statelessComponent) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	deploy, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}
	return restartPod(&deploy.Spec.Template)
}

func (c *statelessComponent) runningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*appsv1.Deployment, error) {
	deployList, err := listDeployOwnedByComponent(reqCtx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}

	cnt := len(deployList)
	if cnt == 0 {
		return nil, fmt.Errorf("no workload found for the stateless component, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	} else if cnt > 1 {
		return nil, fmt.Errorf("more than one workloads found for the stateless component, cluster: %s, component: %s, cnt: %d",
			c.Cluster.Name, c.CompSpec.Name, cnt)
	}

	deploy := deployList[0]
	if deploy.Spec.Replicas == nil {
		return nil, fmt.Errorf("running workload for the stateless component has no replica, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	return deploy, nil
}

func (c *statelessComponent) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	deployObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}

	c.updateDeploymentWorkload(deployObj)

	if err := c.updateService(reqCtx, cli); err != nil {
		return err
	}

	return nil
}
