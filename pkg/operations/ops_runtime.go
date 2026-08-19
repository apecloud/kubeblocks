/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package operations

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type opsRuntime struct {
	ctx          context.Context
	cli          client.Client
	multiCluster bool
	dataCtx      context.Context
	dataGetOpts  []client.GetOption
	dataListOpts []client.ListOption
}

func buildOpsRuntimes(ctx context.Context, cli client.Client, opsRes *OpsResource) (map[string]OpsRuntime, error) {
	runtimes := map[string]OpsRuntime{}
	placement := ""
	if opsRes.Cluster != nil && opsRes.Cluster.Annotations != nil {
		placement = opsRes.Cluster.Annotations[constant.KBAppMultiClusterPlacementKey]
	}
	for _, comp := range opsRes.Cluster.Spec.ComponentSpecs {
		if enabledMultiCluster(opsRes.Cluster) {
			runtimes[comp.Name] = newOpsRuntime(ctx, cli, placement)
		} else {
			runtimes[comp.Name] = newOpsRuntime(ctx, cli, "")
		}
	}
	for _, sharding := range opsRes.Cluster.Spec.Shardings {
		if enabledMultiCluster(opsRes.Cluster) {
			runtimes[sharding.Name] = newOpsRuntime(ctx, cli, placement)
		} else {
			runtimes[sharding.Name] = newOpsRuntime(ctx, cli, "")
		}
	}
	return runtimes, nil
}

func newOpsRuntime(ctx context.Context, cli client.Client, placement string) *opsRuntime {
	if ctx == nil {
		ctx = context.Background()
	}
	r := &opsRuntime{
		ctx:          ctx,
		cli:          cli,
		multiCluster: len(strings.TrimSpace(placement)) > 0,
	}
	if !r.multiCluster {
		return r
	}
	r.dataCtx = multicluster.IntoContext(ctx, placement)
	r.dataGetOpts = []client.GetOption{multicluster.InDataContext()}
	r.dataListOpts = []client.ListOption{multicluster.InDataContext()}
	return r
}

func enabledMultiCluster(obj client.Object) bool {
	return multicluster.Enabled4Object(obj)
}

func (r *opsRuntime) GetWorkload(namespace, clusterName, compName string) (Workload, error) {
	itsName := constant.GenerateClusterComponentName(clusterName, compName)
	its := &workloads.InstanceSet{}
	if err := r.cli.Get(r.ctx, client.ObjectKey{Name: itsName, Namespace: namespace}, its); err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	workload := &defaultWorkload{
		currentRevisionMap: map[string]string{},
		notReadySet:        sets.New[string](),
		notAvailableSet:    sets.New[string](),
		failedSet:          sets.New[string](),
		instanceNames:      sets.New[string](),
	}
	if its.Name != "" {
		workload.name = its.Name
		workload.uid = its.UID
		workload.generation = its.Generation
		workload.minReadySeconds = its.Spec.MinReadySeconds
		for i := range its.Status.InstanceStatus {
			status := &its.Status.InstanceStatus[i]
			if status.DesiredState != "" || status.CurrentState != "" {
				workload.hasInstanceStatus = true
				break
			}
		}
		if !workload.hasInstanceStatus {
			currentRevisions, _ := instanceset.GetRevisions(its.Status.CurrentRevisions)
			workload.currentRevisionMap = currentRevisions
			workload.instanceNames = sets.KeySet(currentRevisions)
			workload.notReadySet = instanceset.GetPodNameSetFromInstanceSetCondition(its, workloads.InstanceReady)
			workload.notAvailableSet = instanceset.GetPodNameSetFromInstanceSetCondition(its, workloads.InstanceAvailable)
			workload.failedSet = instanceset.GetPodNameSetFromInstanceSetCondition(its, workloads.InstanceFailure)
			pods, err := component.ListOwnedPods(r.dataContext(), r.cli, namespace, clusterName, compName, r.dataListOpts...)
			if err != nil {
				return nil, err
			}
			for _, pod := range pods {
				templateName := pod.Labels[constant.KBAppInstanceTemplateLabelKey]
				if templateName == "" {
					templateName = appsv1.GetInstanceTemplateName(clusterName, compName, pod.Name)
				}
				workload.instanceStatuses = append(workload.instanceStatuses, workloads.InstanceStatus{
					PodName:         pod.Name,
					TemplateName:    &templateName,
					DesiredState:    workloads.InstanceDesiredStateActive,
					CurrentState:    workloads.InstanceCurrentStatePresent,
					CurrentRevision: currentRevisions[pod.Name],
					Ready:           !workload.notReadySet.Has(pod.Name),
					Available:       !workload.notAvailableSet.Has(pod.Name),
					Failed:          workload.failedSet.Has(pod.Name),
				})
			}
			return workload, nil
		}
		workload.instanceStatuses = make([]workloads.InstanceStatus, len(its.Status.InstanceStatus))
		for i := range its.Status.InstanceStatus {
			status := its.Status.InstanceStatus[i]
			status.DeepCopyInto(&workload.instanceStatuses[i])
			currentState := status.EffectiveCurrentState()
			if currentState == workloads.InstanceCurrentStateAbsent {
				continue
			}
			workload.currentRevisionMap[status.PodName] = status.CurrentRevision
			if currentState == workloads.InstanceCurrentStateTerminating {
				workload.notReadySet.Insert(status.PodName)
				workload.notAvailableSet.Insert(status.PodName)
				continue
			}
			workload.instanceNames.Insert(status.PodName)
			if !status.Ready {
				workload.notReadySet.Insert(status.PodName)
			}
			if !status.Available {
				workload.notAvailableSet.Insert(status.PodName)
			}
			if status.Failed {
				workload.failedSet.Insert(status.PodName)
			}
		}
		return workload, nil
	}
	pods, err := component.ListOwnedPods(r.dataContext(), r.cli, namespace, clusterName, compName, r.dataListOpts...)
	if err != nil {
		return nil, err
	}
	instances, err := r.buildInstances(namespace, clusterName, compName, pods)
	if err != nil {
		return nil, err
	}
	for _, instance := range instances {
		podInstance, ok := instance.(*defaultInstance)
		if !ok || podInstance.pod == nil {
			continue
		}
		name := podInstance.GetName()
		workload.instanceNames.Insert(name)
		workload.currentRevisionMap[name] = ""
		if !podutils.IsPodReady(podInstance.pod) {
			workload.notReadySet.Insert(name)
		}
		if !podutils.IsPodAvailable(podInstance.pod, 0, metav1.Now()) {
			workload.notAvailableSet.Insert(name)
		}
		if podInstance.IsFailedAndTimedOut() {
			workload.failedSet.Insert(name)
		}
	}
	return workload, nil
}

func (r *opsRuntime) GetInstance(namespace, clusterName, compName, instanceName string) (Instance, error) {
	pod := &corev1.Pod{}
	if err := r.cli.Get(r.dataContext(), client.ObjectKey{Name: instanceName, Namespace: namespace}, pod, r.dataGetOpts...); err != nil {
		if apierrors.IsNotFound(err) {
			pvcMap, loadErr := r.loadVolumes(namespace, clusterName, compName)
			if loadErr != nil {
				return nil, loadErr
			}
			instance := r.newPVCOnlyInstance(compName, instanceName, pvcMap)
			if len(instance.volumes) > 0 {
				return instance, nil
			}
		}
		return nil, err
	}
	if pod.Labels[constant.AppInstanceLabelKey] != clusterName || pod.Labels[constant.KBAppComponentLabelKey] != compName {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" does not belong to component "%s"`, instanceName, compName))
	}
	return r.newPodInstance(compName, pod)
}

func (r *opsRuntime) ListInstances(namespace, clusterName, compName string) ([]Instance, error) {
	pods, err := component.ListOwnedPods(r.dataContext(), r.cli, namespace, clusterName, compName, r.dataListOpts...)
	if err != nil {
		return nil, err
	}
	return r.buildInstances(namespace, clusterName, compName, pods)
}

func (r *opsRuntime) GenerateInstanceNamePlan(namespace, clusterName, compName string, compSpec appsv1.ClusterComponentSpec) (*InstanceNamePlan, error) {
	workloadName := constant.GenerateClusterComponentName(clusterName, compName)
	runningITS := &workloads.InstanceSet{}
	if err := r.cli.Get(r.ctx, client.ObjectKey{Namespace: namespace, Name: workloadName}, runningITS); err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	replicas := compSpec.Replicas
	templates := make([]workloads.InstanceTemplate, len(compSpec.Instances))
	for i := range compSpec.Instances {
		templates[i] = workloads.InstanceTemplate{
			Name:     compSpec.Instances[i].Name,
			Replicas: compSpec.Instances[i].Replicas,
			Ordinals: compSpec.Instances[i].Ordinals,
		}
	}
	protoITS := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: workloadName},
		Spec: workloads.InstanceSetSpec{
			Replicas:            &replicas,
			Instances:           templates,
			Ordinals:            compSpec.Ordinals,
			FlatInstanceOrdinal: compSpec.FlatInstanceOrdinal,
			OfflineInstances:    compSpec.OfflineInstances,
		},
	}
	if runningITS.Name != "" {
		protoITS.Status.AssignedOrdinals = runningITS.DeepCopy().Status.AssignedOrdinals
	}

	itsExt, err := instancetemplate.BuildInstanceSetExt(protoITS, nil)
	if err != nil {
		return nil, err
	}
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		return nil, err
	}
	names, err := nameBuilder.GenerateAllInstanceNames()
	if err != nil {
		return nil, err
	}
	nameToTemplate, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return nil, err
	}
	templateByName := make(map[string]string, len(nameToTemplate))
	for name, template := range nameToTemplate {
		templateByName[name] = template.Name
	}
	offlineTemplateByName := make(map[string]string, len(compSpec.OfflineInstances))
	if compSpec.FlatInstanceOrdinal && len(compSpec.OfflineInstances) > 0 {
		offlineNames := sets.New(compSpec.OfflineInstances...)
		volumes, err := r.loadVolumes(namespace, clusterName, compName)
		if err != nil {
			return nil, err
		}
		for _, pvc := range volumes {
			instanceName := pvc.Labels[constant.KBAppPodNameLabelKey]
			if offlineNames.Has(instanceName) {
				offlineTemplateByName[instanceName] = pvc.Labels[constant.KBAppInstanceTemplateLabelKey]
			}
		}
	} else {
		for _, instanceName := range compSpec.OfflineInstances {
			offlineTemplateByName[instanceName] = appsv1.GetInstanceTemplateName(clusterName, compName, instanceName)
		}
	}
	return &InstanceNamePlan{
		Names:                 names,
		TemplateByName:        templateByName,
		OfflineTemplateByName: offlineTemplateByName,
	}, nil
}

func (r *opsRuntime) Switchover(ctx context.Context, namespace, clusterName, compName, instanceName, candidateName string) error {
	synthesizedComp, err := r.buildSynthesizedCompByCompName(ctx, r.cli, namespace, clusterName, compName)
	if err != nil {
		return err
	}
	switchover := &opsv1alpha1.Switchover{
		ComponentName: compName,
		InstanceName:  instanceName,
		CandidateName: candidateName,
	}
	return r.doSwitchover(ctx, r.cli, synthesizedComp, switchover)
}

func (r *opsRuntime) buildSynthesizedCompByCompName(ctx context.Context, cli client.Client, namespace, clusterName, compName string) (*component.SynthesizedComponent, error) {
	compObj, compDefObj, err := component.GetCompNCompDefByName(ctx, cli, namespace, constant.GenerateClusterComponentName(clusterName, compName))
	if err != nil {
		return nil, err
	}
	return component.BuildSynthesizedComponent(ctx, cli, compDefObj, compObj)
}

// We consider a switchover action succeeds if the action returns without error.
// We don't need to know if a switchover is actually executed.
func (r *opsRuntime) doSwitchover(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent,
	switchover *opsv1alpha1.Switchover) error {
	pods, err := component.ListOwnedPods(r.dataContext(), cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, r.dataListOpts...)
	if err != nil {
		return err
	}

	pod := &corev1.Pod{}
	for _, p := range pods {
		if p.Name == switchover.InstanceName {
			pod = p
			break
		}
	}
	if pod.Name == "" {
		return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" not found`, switchover.InstanceName))
	}
	if switchover.CandidateName != "" {
		candidate, err := r.GetInstance(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, switchover.CandidateName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return intctrlutil.NewFatalError(fmt.Sprintf(`candidate instance "%s" not found`, switchover.CandidateName))
			}
			return err
		}
		if !candidate.HasPod() {
			return intctrlutil.NewFatalError(fmt.Sprintf(`candidate instance "%s" not found`, switchover.CandidateName))
		}
	}

	lfa, err := lifecycle.New(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name,
		synthesizedComp.LifecycleActions.ComponentLifecycleActions, synthesizedComp.TemplateVars, pod, pods)
	if err != nil {
		return err
	}

	// NOTE: switchover is a blocking action currently. May change to non-blocking for better performance.
	// Lifecycle preconditions still use the lifecycle reader contract as-is. If a multi-cluster
	// action needs data-plane runtime readiness checks, model that explicitly in the lifecycle API.
	return lfa.Switchover(ctx, cli, nil, switchover.CandidateName)
}

func (r *opsRuntime) buildInstances(namespace, clusterName, compName string, pods []*corev1.Pod) ([]Instance, error) {
	pvcMap, err := r.loadVolumes(namespace, clusterName, compName)
	if err != nil {
		return nil, err
	}
	instances := make([]Instance, 0, len(pods))
	for i := range pods {
		inst, err := r.newPodInstanceWithVolumes(compName, pods[i], pvcMap)
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

func (r *opsRuntime) loadVolumes(namespace, clusterName, compName string) (map[string]*corev1.PersistentVolumeClaim, error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			constant.AppInstanceLabelKey:    clusterName,
			constant.KBAppComponentLabelKey: compName,
		},
	}
	opts = append(opts, r.dataListOpts...)
	if err := r.cli.List(r.dataContext(), pvcList, opts...); err != nil {
		return nil, err
	}
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim, len(pvcList.Items))
	for i := range pvcList.Items {
		pvc := pvcList.Items[i]
		pvcMap[pvc.Name] = pvc.DeepCopy()
	}
	return pvcMap, nil
}

func (r *opsRuntime) newPodInstance(compName string, pod *corev1.Pod) (Instance, error) {
	pvcMap, err := r.loadVolumes(pod.Namespace, pod.Labels[constant.AppInstanceLabelKey], compName)
	if err != nil {
		return nil, err
	}
	return r.newPodInstanceWithVolumes(compName, pod, pvcMap)
}

func (r *opsRuntime) newPodInstanceWithVolumes(compName string, pod *corev1.Pod, pvcMap map[string]*corev1.PersistentVolumeClaim) (Instance, error) {
	inst := &defaultInstance{
		name:          pod.Name,
		componentName: compName,
		pod:           pod,
		volumes:       map[string]InstanceVolume{},
	}
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		pvc := pvcMap[volume.PersistentVolumeClaim.ClaimName]
		if pvc == nil {
			continue
		}
		inst.volumes[volume.Name] = &instanceVolume{pvc: pvc}
	}
	return inst, nil
}

func (r *opsRuntime) newPVCOnlyInstance(compName, instanceName string, pvcMap map[string]*corev1.PersistentVolumeClaim) *defaultInstance {
	inst := &defaultInstance{
		name:          instanceName,
		componentName: compName,
		volumes:       map[string]InstanceVolume{},
	}
	for _, pvc := range pvcMap {
		if !strings.HasSuffix(pvc.Name, "-"+instanceName) {
			continue
		}
		volumeName := pvc.Labels[constant.VolumeClaimTemplateNameLabelKey]
		if volumeName == "" {
			volumeName = strings.TrimSuffix(pvc.Name, "-"+instanceName)
		}
		inst.volumes[volumeName] = &instanceVolume{pvc: pvc}
	}
	return inst
}

func (r *opsRuntime) dataContext() context.Context {
	if !r.multiCluster {
		return r.ctx
	}
	return r.dataCtx
}

type defaultWorkload struct {
	name               string
	uid                types.UID
	generation         int64
	hasInstanceStatus  bool
	minReadySeconds    int32
	instanceStatuses   []workloads.InstanceStatus
	currentRevisionMap map[string]string
	notReadySet        sets.Set[string]
	notAvailableSet    sets.Set[string]
	failedSet          sets.Set[string]
	instanceNames      sets.Set[string]
}

func (w *defaultWorkload) GetName() string { return w.name }

func (w *defaultWorkload) GetUID() types.UID { return w.uid }

func (w *defaultWorkload) GetGeneration() int64 { return w.generation }

func (w *defaultWorkload) HasInstanceStatus() bool { return w.hasInstanceStatus }

func (w *defaultWorkload) GetMinReadySeconds() int32 { return w.minReadySeconds }

func (w *defaultWorkload) GetInstanceStatuses() []workloads.InstanceStatus {
	result := make([]workloads.InstanceStatus, len(w.instanceStatuses))
	for i := range w.instanceStatuses {
		w.instanceStatuses[i].DeepCopyInto(&result[i])
	}
	return result
}

func (w *defaultWorkload) GetCurrentRevisionMap() map[string]string { return w.currentRevisionMap }

func (w *defaultWorkload) GetNotReadyInstanceNameSet() sets.Set[string] {
	return w.notReadySet.Clone()
}

func (w *defaultWorkload) GetNotAvailableInstanceNameSet() sets.Set[string] {
	return w.notAvailableSet.Clone()
}

func (w *defaultWorkload) GetFailedInstanceNameSet() sets.Set[string] { return w.failedSet.Clone() }

func (w *defaultWorkload) GetInstanceNameSet() sets.Set[string] {
	return w.instanceNames.Clone()
}

type defaultInstance struct {
	name          string
	componentName string
	pod           *corev1.Pod
	volumes       map[string]InstanceVolume
}

func (i *defaultInstance) GetComponentName() string { return i.componentName }

func (i *defaultInstance) GetName() string { return i.name }

func (i *defaultInstance) GetCreationTimestamp() metav1.Time {
	if i.pod == nil {
		return metav1.Time{}
	}
	return i.pod.CreationTimestamp
}

func (i *defaultInstance) HasPod() bool {
	return i.pod != nil
}

func (i *defaultInstance) IsDeleting() bool {
	return i.pod != nil && !i.pod.DeletionTimestamp.IsZero()
}

func (i *defaultInstance) GetRole() string {
	if i.pod == nil {
		return ""
	}
	return i.pod.Labels[constant.RoleLabelKey]
}

func (i *defaultInstance) IsAvailable(minReadySeconds int32, roleAware bool) bool {
	if i.pod == nil || i.IsDeleting() {
		return false
	}
	if roleAware {
		return intctrlutil.PodIsReadyWithLabel(*i.pod)
	}
	return podutils.IsPodAvailable(i.pod, minReadySeconds, metav1.Now())
}

func (i *defaultInstance) IsFailedAndTimedOut() bool {
	if i.pod == nil {
		return false
	}
	isFailed, isTimeout, _ := intctrlutil.IsPodFailedAndTimedOut(i.pod)
	return isFailed && isTimeout
}

func (i *defaultInstance) GetImage(containerName string) string {
	container := i.getContainer(containerName)
	if container == nil {
		return ""
	}
	return container.Image
}

func (i *defaultInstance) GetStatusImage(containerName string) string {
	if i.pod == nil {
		return ""
	}
	for _, status := range i.pod.Status.ContainerStatuses {
		if status.Name == containerName {
			return status.Image
		}
	}
	if containerName == "" && len(i.pod.Status.ContainerStatuses) > 0 {
		return i.pod.Status.ContainerStatuses[0].Image
	}
	return ""
}

func (i *defaultInstance) GetResources(containerName string) corev1.ResourceRequirements {
	container := i.getContainer(containerName)
	if container == nil {
		return corev1.ResourceRequirements{}
	}
	return container.Resources
}

func (i *defaultInstance) GetNodeName() string {
	if i.pod == nil {
		return ""
	}
	return i.pod.Spec.NodeName
}

func (i *defaultInstance) GetTolerations() []corev1.Toleration {
	if i.pod == nil {
		return nil
	}
	return append([]corev1.Toleration{}, i.pod.Spec.Tolerations...)
}

func (i *defaultInstance) GetAffinity() *corev1.Affinity {
	if i.pod == nil || i.pod.Spec.Affinity == nil {
		return nil
	}
	return i.pod.Spec.Affinity.DeepCopy()
}

func (i *defaultInstance) GetTopologySpreadConstraints() []corev1.TopologySpreadConstraint {
	if i.pod == nil {
		return nil
	}
	return append([]corev1.TopologySpreadConstraint{}, i.pod.Spec.TopologySpreadConstraints...)
}

func (i *defaultInstance) GetPodVolumes() []corev1.Volume {
	if i.pod == nil {
		return nil
	}
	return append([]corev1.Volume{}, i.pod.Spec.Volumes...)
}

func (i *defaultInstance) GetVolumeMounts(containerName string) []corev1.VolumeMount {
	container := i.getContainer(containerName)
	if container == nil {
		return nil
	}
	return append([]corev1.VolumeMount{}, container.VolumeMounts...)
}

func (i *defaultInstance) GetVolume(name string) (InstanceVolume, bool) {
	volume, ok := i.volumes[name]
	return volume, ok
}

func (i *defaultInstance) getContainer(containerName string) *corev1.Container {
	if i.pod == nil {
		return nil
	}
	if containerName != "" {
		for idx := range i.pod.Spec.Containers {
			if i.pod.Spec.Containers[idx].Name == containerName {
				return &i.pod.Spec.Containers[idx]
			}
		}
	}
	if len(i.pod.Spec.Containers) == 0 {
		return nil
	}
	return &i.pod.Spec.Containers[0]
}

type instanceVolume struct {
	pvc *corev1.PersistentVolumeClaim
}

func (v *instanceVolume) GetClaimName() string {
	if v.pvc == nil {
		return ""
	}
	return v.pvc.Name
}

func (v *instanceVolume) GetRequestedStorage() resource.Quantity {
	if v.pvc == nil || v.pvc.Spec.Resources.Requests.Storage() == nil {
		return resource.Quantity{}
	}
	return *v.pvc.Spec.Resources.Requests.Storage()
}

func (v *instanceVolume) GetCapacity() resource.Quantity {
	if v.pvc == nil || v.pvc.Status.Capacity.Storage() == nil {
		return resource.Quantity{}
	}
	return *v.pvc.Status.Capacity.Storage()
}

func (v *instanceVolume) IsBound() bool {
	return v.pvc != nil && v.pvc.Status.Phase == corev1.ClaimBound
}

func (v *instanceVolume) IsExpanding() bool {
	if v.pvc == nil {
		return false
	}
	for _, condition := range v.pvc.Status.Conditions {
		if condition.Type == corev1.PersistentVolumeClaimResizing || condition.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			return true
		}
	}
	return false
}
