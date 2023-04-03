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
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func newReplicationComponent(definition *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compVer *appsv1alpha1.ClusterComponentVersion,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) *replicationComponent {
	return &replicationComponent{
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
	}
}

type replicationComponent struct {
	componentBase
}

type replicationComponentBuilder struct {
	componentBuilderBase
	workloads []*appsv1.StatefulSet
}

func (b *replicationComponentBuilder) mutableWorkload(idx int32) client.Object {
	return b.workloads[idx]
}

func (b *replicationComponentBuilder) mutablePodSpec(idx int32) *corev1.PodSpec {
	return &b.workloads[idx].Spec.Template.Spec
}

func (b *replicationComponentBuilder) buildService() componentBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcListLow(b.comp.GetCluster(), b.comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, 0, len(svcList))
		for _, svc := range svcList {
			svc.Spec.Selector[constant.RoleLabelKey] = string(replicationset.Primary)
			objs = append(objs, svc)
		}
		return objs, err
	}
	return b.buildWrapper(buildfn)
}

func (b *replicationComponentBuilder) buildWorkload(idx int32) componentBuilder {
	buildfn := func() ([]client.Object, error) {
		component := b.comp.GetSynthesizedComponent()
		if b.envConfig == nil {
			return nil, fmt.Errorf("build replication workload but env config is nil, cluster: %s, component: %s",
				b.comp.GetClusterName(), component.Name)
		}

		sts, err := builder.BuildStsLow(b.reqCtx, b.comp.GetCluster(), component, b.envConfig.Name)
		if err != nil {
			return nil, err
		}

		// sts.Name renamed with suffix "-<sts-idx>" for subsequent sts workload
		if idx != 0 {
			sts.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.ObjectMeta.Name, idx)
		}
		if idx == component.GetPrimaryIndex() {
			sts.Labels[constant.RoleLabelKey] = string(replicationset.Primary)
		} else {
			sts.Labels[constant.RoleLabelKey] = string(replicationset.Secondary)
		}
		sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType

		b.workloads = append(b.workloads, sts)

		return nil, nil // don't return sts here, and it will not add to resource queue now
	}
	return b.buildWrapper(buildfn)
}

func (b *replicationComponentBuilder) buildVolume(idx int32) componentBuilder {
	buildfn := func() ([]client.Object, error) {
		workload := b.mutableWorkload(idx)
		// if workload == nil {
		// 	return nil, fmt.Errorf("build replication volumes but workload is nil, cluster: %s, component: %s",
		// 		b.comp.GetClusterName(), b.comp.GetName())
		// }

		component := b.comp.GetSynthesizedComponent()
		sts := workload.(*appsv1.StatefulSet)
		objs := make([]client.Object, 0)

		// TODO: The pvc objects involved in all processes in the KubeBlocks will be reconstructed into a unified generation method
		pvcMap := replicationset.GeneratePVCFromVolumeClaimTemplates(sts, component.VolumeClaimTemplates)
		for pvcTplName, pvc := range pvcMap {
			builder.BuildPersistentVolumeClaimLabels(sts, pvc, component, pvcTplName)
			objs = append(objs, pvc)
		}

		// binding persistentVolumeClaim to podSpec.Volumes
		podSpec := &sts.Spec.Template.Spec
		if podSpec == nil {
			return objs, nil
		}

		podVolumes := podSpec.Volumes
		for _, pvc := range pvcMap {
			volumeName := strings.Split(pvc.Name, "-")[0]
			podVolumes, _ = intctrlutil.CreateOrUpdateVolume(podVolumes, volumeName, func(volumeName string) corev1.Volume {
				return corev1.Volume{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				}
			}, nil)
		}
		podSpec.Volumes = podVolumes

		return objs, nil
	}
	return b.buildWrapper(buildfn)
}

func (b *replicationComponentBuilder) complete() error {
	if b.error != nil {
		return b.error
	}
	if len(b.workloads) == 0 || b.workloads[0] == nil {
		return fmt.Errorf("fail to create compoennt workloads, cluster: %s, component: %s",
			b.comp.GetClusterName(), b.comp.GetName())
	}

	for _, obj := range b.workloads {
		b.comp.addWorkload(obj, b.defaultAction, nil)
	}
	return nil
}

func (c *replicationComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, action *Action) error {
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, cli, *c.Cluster, *c.Definition, *c.CompDef, *c.CompSpec, c.CompVer)
	if err != nil {
		return err
	}
	c.Component = synthesizedComp

	builder := &replicationComponentBuilder{
		componentBuilderBase: componentBuilderBase{
			reqCtx:        reqCtx,
			client:        cli,
			comp:          c,
			defaultAction: action,
			error:         nil,
			envConfig:     nil,
		},
		workloads: make([]*appsv1.StatefulSet, 0),
	}
	builder.concreteBuilder = builder

	// env and headless service are component level resources
	builder.buildEnv(). // TODO: workload & scaling related
				buildHeadlessService()
	for i := int32(0); i < synthesizedComp.Replicas; i++ {
		builder.buildWorkload(i). // build workload here since other objects depend on it.
						buildVolume(i).
						buildConfig(i).
						buildTLSVolume(i).
						buildVolumeMount(i)
		if builder.error != nil {
			return builder.error
		}
	}
	return builder.buildService().buildTLSCert().complete()
}

func (c *replicationComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Replication
}

func (c *replicationComponent) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := listStsOwnedByComponent(reqCtx, cli, c.GetNamespace(), c.GetMatchingLabels()); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *replicationComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

func (c *replicationComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

func (c *replicationComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO: delete component owned resources
	return nil
}

func (c *replicationComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	workloads, err := c.runningWorkloads(reqCtx, cli)
	if err != nil {
		return err
	}

	for _, sts := range workloads {
		for _, vct := range c.Component.VolumeClaimTemplates {
			pvc := &corev1.PersistentVolumeClaim{}
			key := client.ObjectKey{
				Namespace: sts.GetNamespace(),
				Name:      replicationset.GetPersistentVolumeClaimName(sts, &vct, 0),
			}
			if err = cli.Get(reqCtx.Ctx, key, pvc); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			if apierrors.IsNotFound(err) {
				continue // new added pvc?
			}

			if pvc.Spec.Resources.Requests[corev1.ResourceStorage] == vct.Spec.Resources.Requests[corev1.ResourceStorage] {
				continue
			}

			if vertex := FindMatchedVertex[*corev1.PersistentVolumeClaim](c.dag, key); vertex == nil {
				return fmt.Errorf("cann't find PVC object when to update it, cluster: %s, component: %s, pvc: %s",
					c.Cluster.Name, c.Component.Name, key)
			} else {
				vertex.(*lifecycleVertex).action = actionPtr(UPDATE)
			}
		}
	}
	return nil
}

func (c *replicationComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsList, err := listStsOwnedByComponent(reqCtx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return err
	}

	ret := c.horizontalScaling(stsList)
	if ret == 0 {
		return nil
	} else if ret < 0 {
		if err := c.scaleIn(reqCtx, cli, stsList); err != nil {
			return err
		}
	} else {
		if err := c.scaleOut(reqCtx, cli); err != nil {
			return err
		}
	}

	reqCtx.Recorder.Eventf(c.Cluster,
		corev1.EventTypeNormal,
		"HorizontalScale",
		"start horizontal scale component %s of cluster %s from %d to %d",
		c.GetName(), c.GetClusterName(), int(c.Component.Replicas)-ret, c.Component.Replicas)

	return nil
}

func (c *replicationComponent) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsList, err := c.runningWorkloads(reqCtx, cli)
	if err != nil {
		return err
	}

	for _, sts := range stsList {
		if err := restartPod(&sts.Spec.Template); err != nil {
			return err
		}
	}
	return nil
}

func (c *replicationComponent) Snapshot(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil // TODO: impl
}

func (c *replicationComponent) runningWorkloads(reqCtx intctrlutil.RequestCtx, cli client.Client) ([]*appsv1.StatefulSet, error) {
	stsList, err := listStsOwnedByComponent(reqCtx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}
	if len(stsList) == 0 {
		return nil, fmt.Errorf("no workload found for the replication component, cluster: %s, component: %s",
			c.Cluster.Name, c.Component.Name)
	}
	return stsList, nil
}

// TODO: fix stale cache problem
// TODO: if sts created in last reconcile-loop not present in cache, hasReplicationSetHScaling return false positive
// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (c *replicationComponent) horizontalScaling(stsList []*appsv1.StatefulSet) int {
	// TODO: should use a more stable status
	return int(c.Component.Replicas) - len(stsList)
}

func (c *replicationComponent) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsList []*appsv1.StatefulSet) error {
	stsToDelete, err := replicationset.HandleComponentHorizontalScaleIn(reqCtx.Ctx, cli, c.Cluster, c.GetSynthesizedComponent(), stsList)
	if err != nil {
		return err
	}
	for _, sts := range stsToDelete {
		c.deleteResource(sts, nil)
	}

	return nil
}

func (c *replicationComponent) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *replicationComponent) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObjList, err := c.runningWorkloads(reqCtx, cli)
	if err != nil {
		return err
	}

	for i, stsObj := range stsObjList {
		c.updateWorkload(stsObj, int32(i))
	}

	if err := c.updateService(reqCtx, cli); err != nil {
		return err
	}

	return nil
}

func (c *replicationComponent) updateWorkload(stsObj *appsv1.StatefulSet, idx int32) {
	stsObjCopy := stsObj.DeepCopy()
	stsProto := c.workloadVertexs[idx].obj.(*appsv1.StatefulSet)

	// keep the original template annotations.
	// if annotations exist and are replaced, the statefulSet will be updated.
	mergeAnnotations(stsObjCopy.Spec.Template.Annotations, &stsProto.Spec.Template.Annotations)
	stsObjCopy.Spec.Template = stsProto.Spec.Template
	stsObjCopy.Spec.Replicas = stsProto.Spec.Replicas
	stsObjCopy.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
	if !reflect.DeepEqual(&stsObj.Spec, &stsObjCopy.Spec) {
		c.workloadVertexs[idx].obj = stsObjCopy
		c.workloadVertexs[idx].action = actionPtr(UPDATE)

		// sync component phase
		//updateComponentPhaseWithOperation2(c.GetCluster(), c.GetName())
	}
}
