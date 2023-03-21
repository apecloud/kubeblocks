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

package component

import (
	"fmt"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewReplicationComponent(definition appsv1alpha1.ClusterDefinition,
	version appsv1alpha1.ClusterVersion,
	cluster appsv1alpha1.Cluster,
	compSpec appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) *replicationComponent {
	return &replicationComponent{
		componentBase: componentBase{
			Definition:     definition,
			Cluster:        cluster,
			CompDef:        *(&definition).GetComponentDefByName(compSpec.ComponentDefRef),
			CompVer:        version.GetDefNameMappingComponents()[compSpec.ComponentDefRef],
			CompSpec:       compSpec,
			Component:      nil,
			WorkloadVertex: nil,
			Dag:            dag,
		},
	}
}

type replicationComponent struct {
	componentBase
}

func (c *replicationComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	synthesizedComp, err := BuildSynthesizedComponent(reqCtx, cli, c.Cluster, c.Definition, c.CompDef, c.CompSpec, c.CompVer)
	if err != nil {
		return err
	}
	c.Component = synthesizedComp

	builder := &componentBuilder{
		ReqCtx:    reqCtx,
		Client:    cli,
		Comp:      c,
		Error:     nil,
		EnvConfig: nil,
		Workload:  nil,
	}
	// runtime, config, script, env, volume, service, monitor, probe
	return builder.buildEnv(). // TODO: workload related, scaling related
		buildWorkload(). // build workload here since other objects depend on it.
		buildHeadlessService().
		buildConfig().
		buildTlsVolume().
		buildPDB().
		buildService().
		buildVolumeMount().
		complete()
}

func (c *replicationComponent) GetName() string {
	return c.CompSpec.Name
}

func (c *replicationComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Replication
}

func (c *replicationComponent) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := listStsOwnedByComponent(reqCtx, cli, c.Cluster, c.CompSpec); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *replicationComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if exist, err := c.Exist(reqCtx, cli); err != nil || exist {
		if err != nil {
			return err
		}
		return fmt.Errorf("component to be created is aready exist, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	err := c.init(reqCtx, cli)
	if err != nil {
		return err
	}

	for i := int32(0); i < builder.Task.Component.Replicas; i++ {
		if err := builder.BuildEnv().
			BuildWorkload(). // build workload here since other objects depend on it.
			BuildHeadlessService().
			BuildConfig().
			BuildTlsVolume().
			BuildPDB().
			BuildService().
			BuildVolumeMount().
			Complete(); err != nil {
			return err
		}
	}

	//return plan.PrepareReplicationComponentResources(reqCtx, cli, &task)

	for index := int32(0); index < task.Component.Replicas; index++ {
		if err := prepareComponentWorkloads(reqCtx, cli, task,
			func(envConfig *corev1.ConfigMap) (client.Object, error) {
				return buildReplicationSet(reqCtx, task, envConfig.Name, index)
			}); err != nil {
			return err
		}
	}

	svcList, err := builder.BuildSvcList(task.GetBuilderParams())
	if err != nil {
		return err
	}
	for _, svc := range svcList {
		svc.Spec.Selector[constant.RoleLabelKey] = string(replicationset.Primary)
		task.AppendResource(svc)
	}

	return nil
}

func (c *replicationComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO: delete component managed resources
	return nil
}

func (c *replicationComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *replicationComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObj, _ := vertex.oriObj.(*appsv1.StatefulSet)
	stsProto, _ := vertex.obj.(*appsv1.StatefulSet)
	// check stsObj.Spec.VolumeClaimTemplates storage
	// request size and find attached PVC and patch request
	// storage size
	for _, vct := range stsObj.Spec.VolumeClaimTemplates {
		var vctProto *corev1.PersistentVolumeClaim
		for _, v := range stsProto.Spec.VolumeClaimTemplates {
			if v.Name == vct.Name {
				vctProto = &v
				break
			}
		}

		// REVIEW: how could VCT proto is nil?
		if vctProto == nil {
			continue
		}

		if vct.Spec.Resources.Requests[corev1.ResourceStorage] == vctProto.Spec.Resources.Requests[corev1.ResourceStorage] {
			continue
		}

		for i := *stsObj.Spec.Replicas - 1; i >= 0; i-- {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
			}
			if err := cli.Get(reqCtx.Ctx, pvcKey, pvc); err != nil {
				return err
			}
			obj := pvc.DeepCopy()
			obj.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
			v := &lifecycleVertex{
				obj:    obj,
				oriObj: pvc,
				action: actionPtr(UPDATE),
			}
			dag.AddVertex(v)
			dag.Connect(vertex, v)
		}
	}
	return nil
}

func (c *replicationComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	hasScaling, err := c.hasReplicationSetHScaling()
	if err != nil {
		return err
	}
	if !hasScaling {
		return nil
	}

	stsList, err := listStsOwnedByComponent(reqCtx, cli, c.Cluster, c.CompSpec)
	if err != nil {
		return err
	}
	// TODO: how about newly created sts?
	return replicationset.HandleReplicationSet(reqCtx.Ctx, cli, &c.Cluster, stsList)
}

// TODO: fix stale cache problem
// TODO: if sts created in last reconcile-loop not present in cache, hasReplicationSetHScaling return false positive
func (c *replicationComponent) hasReplicationSetHScaling(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	stsList, err := listStsOwnedByCluster(reqCtx, cli, c.Cluster)
	if err != nil {
		return false, err
	}
	if len(stsList) == 0 {
		return false, err
	}

	for _, compDef := range c.Definition.Spec.ComponentDefs {
		if compDef.WorkloadType == appsv1alpha1.Replication {
			return true, nil
		}
	}
	return false, nil
}

// buildReplicationSet builds a replication component on statefulSet.
func buildReplicationSet(reqCtx intctrlutil.RequestCtx,
	task *intctrltypes.ReconcileTask,
	envConfigName string,
	stsIndex int32) (*appsv1.StatefulSet, error) {

	sts, err := builder.BuildSts(reqCtx, task.GetBuilderParams(), envConfigName)
	if err != nil {
		return nil, err
	}
	// sts.Name renamed with suffix "-<stsIdx>" for subsequent sts workload
	if stsIndex != 0 {
		sts.ObjectMeta.Name = fmt.Sprintf("%s-%d", sts.ObjectMeta.Name, stsIndex)
	}
	if stsIndex == task.Component.GetPrimaryIndex() {
		sts.Labels[constant.RoleLabelKey] = string(replicationset.Primary)
	} else {
		sts.Labels[constant.RoleLabelKey] = string(replicationset.Secondary)
	}
	sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	// build replicationSet persistentVolumeClaim manually
	if err := buildReplicationSetPVC(task, sts); err != nil {
		return sts, err
	}
	return sts, nil
}

// buildReplicationSetPVC builds replicationSet persistentVolumeClaim manually,
// replicationSet does not manage pvc through volumeClaimTemplate defined on statefulSet,
// the purpose is convenient to convert between workloadTypes in the future (TODO).
func buildReplicationSetPVC(task *intctrltypes.ReconcileTask, sts *appsv1.StatefulSet) error {
	// generate persistentVolumeClaim objects used by replicationSet's pod from component.VolumeClaimTemplates
	// TODO: The pvc objects involved in all processes in the KubeBlocks will be reconstructed into a unified generation method
	pvcMap := replicationset.GeneratePVCFromVolumeClaimTemplates(sts, task.Component.VolumeClaimTemplates)
	for pvcTplName, pvc := range pvcMap {
		builder.BuildPersistentVolumeClaimLabels(sts, pvc, task.Component, pvcTplName)
		task.AppendResource(pvc)
	}

	// binding persistentVolumeClaim to podSpec.Volumes
	podSpec := &sts.Spec.Template.Spec
	if podSpec == nil {
		return nil
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
	return nil
}
