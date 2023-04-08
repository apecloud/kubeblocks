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

package types

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type ComponentBase struct {
	Client client.Client

	Definition *appsv1alpha1.ClusterDefinition
	Version    *appsv1alpha1.ClusterVersion
	Cluster    *appsv1alpha1.Cluster

	// TODO(refactor): should remove those members in future.
	CompDef  *appsv1alpha1.ClusterComponentDefinition
	CompVer  *appsv1alpha1.ClusterComponentVersion
	CompSpec *appsv1alpha1.ClusterComponentSpec

	// built synthesized component
	Component *component.SynthesizedComponent

	ComponentSet ComponentSet

	Dag *graph.DAG
	// DAG vertexes of main workload object(s)
	WorkloadVertexs []*ictrltypes.LifecycleVertex
}

func (c *ComponentBase) GetClient() client.Client {
	return c.Client
}

func (c *ComponentBase) GetName() string {
	return c.CompSpec.Name
}

func (c *ComponentBase) GetNamespace() string {
	return c.Cluster.Namespace
}

func (c *ComponentBase) GetClusterName() string {
	return c.Cluster.Name
}

func (c *ComponentBase) GetDefinition() *appsv1alpha1.ClusterDefinition {
	return c.Definition
}

func (c *ComponentBase) GetVersion() *appsv1alpha1.ClusterVersion {
	return c.Version
}

func (c *ComponentBase) GetCluster() *appsv1alpha1.Cluster {
	return c.Cluster
}

func (c *ComponentBase) GetSynthesizedComponent() *component.SynthesizedComponent {
	return c.Component
}

func (c *ComponentBase) GetMatchingLabels() client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
}

func (c *ComponentBase) AddResource(obj client.Object, action *ictrltypes.LifecycleAction,
	parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	if obj == nil {
		panic("try to add nil object")
	}
	vertex := &ictrltypes.LifecycleVertex{
		Obj:    obj,
		Action: action,
	}
	c.Dag.AddVertex(vertex)

	if parent != nil {
		c.Dag.Connect(parent, vertex)
	}
	return vertex
}

func (c *ComponentBase) AddWorkload(obj client.Object, action *ictrltypes.LifecycleAction, parent *ictrltypes.LifecycleVertex) {
	c.WorkloadVertexs = append(c.WorkloadVertexs, c.AddResource(obj, action, parent))
}

func (c *ComponentBase) CreateResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectCreate(c.Dag, obj, parent)
}

func (c *ComponentBase) DeleteResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectDelete(c.Dag, obj, parent)
}

func (c *ComponentBase) UpdateResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectUpdate(c.Dag, obj, parent)
}

func (c *ComponentBase) NoopResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectNoop(c.Dag, obj, parent)
}

// ValidateObjectsAction validates the action of all objects in dag has been determined
func (c *ComponentBase) ValidateObjectsAction() error {
	for _, v := range c.Dag.Vertices() {
		if node, ok := v.(*ictrltypes.LifecycleVertex); !ok {
			return fmt.Errorf("unexpected vertex type, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		} else if node.Action == nil {
			return fmt.Errorf("unexpected nil vertex action, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		}
	}
	return nil
}

// resolveObjectsAction resolves the action of objects in dag to guarantee that all object actions will be determined
func (c *ComponentBase) ResolveObjectsAction(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	snapshot, err := util.ReadCacheSnapshot(reqCtx, cli, c.GetCluster())
	if err != nil {
		return err
	}
	for _, v := range c.Dag.Vertices() {
		if node, ok := v.(*ictrltypes.LifecycleVertex); !ok {
			return fmt.Errorf("unexpected vertex type, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		} else if node.Action == nil {
			if action, err := util.ResolveObjectAction(snapshot, node); err != nil {
				return err
			} else {
				node.Action = action
			}
		}
	}
	if util.IsClusterStatusUpdating(*c.GetCluster()) {
		for _, vertex := range c.Dag.Vertices() {
			v, _ := vertex.(*ictrltypes.LifecycleVertex)
			// TODO(refactor): fix me, workaround for h-scaling to update stateful set
			if _, ok := v.Obj.(*appsv1.StatefulSet); !ok {
				v.Immutable = true
			}
		}
	}
	return c.ValidateObjectsAction()
}

func (c *ComponentBase) ComposeSynthesizedComponent(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, cli, *c.Cluster, *c.Definition, *c.CompDef, *c.CompSpec, c.CompVer)
	if err != nil {
		return err
	}
	c.Component = synthesizedComp
	return nil
}

func (c *ComponentBase) UpdateService(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	labels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
	svcObjList, err := util.ListObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.ServiceSignature, c.GetNamespace(), labels)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	svcProtoList := ictrltypes.FindAll[*corev1.Service](c.Dag)

	// create new services or update existed services
	for _, vertex := range svcProtoList {
		node, _ := vertex.(*ictrltypes.LifecycleVertex)
		svcProto, _ := node.Obj.(*corev1.Service)

		if pos := slices.IndexFunc(svcObjList, func(svc *corev1.Service) bool {
			return svc.GetName() == svcProto.GetName()
		}); pos < 0 {
			node.Action = ictrltypes.ActionCreatePtr()
		} else {
			svcProto.Annotations = util.MergeServiceAnnotations(svcObjList[pos].Annotations, svcProto.Annotations)
			node.Action = ictrltypes.ActionUpdatePtr()
		}
	}

	// delete useless services
	for _, svc := range svcObjList {
		if pos := slices.IndexFunc(svcProtoList, func(vertex graph.Vertex) bool {
			node, _ := vertex.(*ictrltypes.LifecycleVertex)
			svcProto, _ := node.Obj.(*corev1.Service)
			return svcProto.GetName() == svc.GetName()
		}); pos < 0 {
			c.DeleteResource(svc, nil)
		}
	}
	return nil
}

func (c *ComponentBase) updateStatus(updatefn func(status *appsv1alpha1.ClusterComponentStatus)) {
	if updatefn == nil {
		return
	}

	if c.Cluster.Status.Components == nil {
		c.Cluster.Status.Components = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	if _, ok := c.Cluster.Status.Components[c.GetName()]; !ok {
		c.Cluster.Status.Components[c.GetName()] = appsv1alpha1.ClusterComponentStatus{}
	}

	status := c.Cluster.Status.Components[c.GetName()]
	updatefn(&status)
	c.Cluster.Status.Components[c.GetName()] = status
}

func (c *ComponentBase) SetStatusPhase(phase appsv1alpha1.ClusterComponentPhase) {
	c.setStatusPhaseWithMsg(phase, "", "")
}

func (c *ComponentBase) setStatusPhaseWithMsg(phase appsv1alpha1.ClusterComponentPhase, msgKey, msg string) {
	c.updateStatus(func(status *appsv1alpha1.ClusterComponentStatus) {
		if status.Phase == phase {
			return
		}

		// TODO(refactor): define the status phase transition diagram
		if phase == appsv1alpha1.SpecReconcilingClusterCompPhase && status.Phase != appsv1alpha1.RunningClusterCompPhase {
			return
		}

		status.Phase = phase
		if msgKey != "" {
			if status.Message == nil {
				status.Message = map[string]string{}
			}
			status.Message[msgKey] = msg
		}
	})
}

func (c *ComponentBase) syncComponentStatusForEvent(phase appsv1alpha1.ClusterComponentPhase, event *corev1.Event) {
	if phase == "" {
		return
	}
	c.updateStatus(func(status *appsv1alpha1.ClusterComponentStatus) {
		if status.Phase != phase {
			status.Phase = phase
			updateComponentStatusMessage(c.GetCluster(), c.GetName(), status, event)
			return
		}
		// check whether it is a new warning event and the component phase is running
		if !isExistsEventMsg(status.Message, event) && phase != appsv1alpha1.RunningClusterCompPhase {
			updateComponentStatusMessage(c.GetCluster(), c.GetName(), status, event)
			return
		}
	})
}

func (c *ComponentBase) phase() appsv1alpha1.ClusterComponentPhase {
	if c.Cluster.Status.Components == nil {
		return ""
	}
	if _, ok := c.Cluster.Status.Components[c.GetName()]; !ok {
		return ""
	}
	return c.Cluster.Status.Components[c.GetName()].Phase
}

func (c *ComponentBase) StatusImpl(reqCtx intctrlutil.RequestCtx, cli client.Client, objs []client.Object) error {
	///// ClusterStatusHandler.handleDeletePVCCronJobEvent
	checkPVCDeletionJobFail := func(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
		return false, nil
	}
	if failed, err := checkPVCDeletionJobFail(reqCtx, cli); err != nil {
		return err
	} else if failed {
		// c.SetObjectMessage(object.GetObjectKind().GroupVersionKind().Kind, object.GetName(), message)
		// CronJob kind and name
		// msgKey := fmt.Sprintf("%s/%s", object.GetObjectKind().GroupVersionKind().Kind, object.GetName())
		c.setStatusPhaseWithMsg(appsv1alpha1.AbnormalClusterCompPhase, "", "")
	}

	/////// ClusterStatusHandler.handleClusterStatusByEvent
	//phase, err := c.ComponentSet.GetPhaseWhenPodsNotReady(ReqCtx.Ctx, c.GetName())
	//if err != nil {
	//	return err
	//}
	//// TODO(refactor): event -> status message
	//event := &corev1.Event{}
	//c.syncComponentStatusForEvent(phase, event)

	for _, obj := range objs {
		///// WorkloadController.handleWorkloadUpdate
		// patch role labels and update roles in component status
		if vertexes, err := c.ComponentSet.HandleRoleChange(reqCtx.Ctx, obj); err != nil {
			return err
		} else if vertexes != nil {
			for v := range vertexes {
				c.Dag.AddVertex(v)
			}
		}

		// restart pod if needed
		if vertexes, err := c.ComponentSet.HandleRestart(reqCtx.Ctx, obj); err != nil {
			return err
		} else if vertexes != nil {
			for v := range vertexes {
				c.Dag.AddVertex(v)
			}
		}

		// update component status
		// TODO: wait & requeue
		newStatus, err := c.rebuildLatestStatus(reqCtx, cli, obj)
		if err != nil {
			return err
		}
		if newStatus != nil {
			c.updateStatus(func(status *appsv1alpha1.ClusterComponentStatus) {
				status.Phase = newStatus.Phase
				status.Message = newStatus.Message
				status.PodsReady = newStatus.PodsReady
				status.PodsReadyTime = newStatus.PodsReadyTime
			})
		}
	}
	return nil
}

func (c *ComponentBase) rebuildLatestStatus(reqCtx intctrlutil.RequestCtx, cli client.Client, obj client.Object) (*appsv1alpha1.ClusterComponentStatus, error) {
	pods, err := util.ListPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}

	isRunning, err := c.ComponentSet.IsRunning(reqCtx.Ctx, obj)
	if err != nil {
		return nil, err
	}

	var podsReady *bool
	if c.Component.Replicas > 0 {
		podsReadyForComponent, err := c.ComponentSet.PodsReady(reqCtx.Ctx, obj)
		if err != nil {
			return nil, err
		}
		podsReady = &podsReadyForComponent
	}

	// TODO(refactor): fix me
	status := c.Cluster.Status.Components[c.GetName()]
	pstatus := &status

	//var wait bool
	hasTimedOutPod := false
	if !isRunning {
		if podsReady != nil && *podsReady {
			// check if the role probe timed out when component phase is not Running but all pods of component are ready.
			// TODO(refactor): wait = true
			c.ComponentSet.HandleProbeTimeoutWhenPodsReady(pstatus, pods)
		} else {
			hasTimedOutPod, pstatus.Message, err = hasFailedAndTimedOutPod(pods)
			if err != nil {
				return nil, err
			}
			if !hasTimedOutPod {
				// TODO(refactor): wait = true
			}
		}
	}

	if err = c.rebuildComponentStatus(reqCtx, isRunning, podsReady, hasTimedOutPod, pstatus); err != nil {
		return nil, err
	}
	return pstatus, nil
}

// updateComponentsPhase updates the component status Phase etc. into the cluster.Status.Components map.
func (c *ComponentBase) rebuildComponentStatus(reqCtx intctrlutil.RequestCtx,
	running bool,
	podsAreReady *bool,
	hasFailedPodTimedOut bool,
	status *appsv1alpha1.ClusterComponentStatus) error {
	if !running {
		// if no operation is running in cluster or failed pod timed out,
		// means the component is Failed or Abnormal.
		if slices.Contains(appsv1alpha1.GetClusterUpRunningPhases(), c.Cluster.Status.Phase) || hasFailedPodTimedOut {
			if phase, err := c.ComponentSet.GetPhaseWhenPodsNotReady(reqCtx.Ctx, c.GetName()); err != nil {
				return err
			} else if phase != "" {
				status.Phase = phase
			}
		}
	} else {
		if c.Component.Replicas == 0 {
			// if replicas number of component is zero, the component has stopped.
			// 'Stopped' is a special 'Running' for workload(StatefulSet/Deployment).
			status.Phase = appsv1alpha1.StoppedClusterCompPhase
		} else {
			// change component phase to Running when workloads of component are running.
			status.Phase = appsv1alpha1.RunningClusterCompPhase
		}
		status.SetMessage(nil)
	}
	status.PodsReady = podsAreReady
	if podsAreReady != nil && *podsAreReady {
		status.PodsReadyTime = &metav1.Time{Time: time.Now()}
	} else {
		status.PodsReadyTime = nil
	}
	return nil
}

// REVIEW: this handling is rather hackish, call for refactor.
// handleRestoreGarbageBeforeRunning handles the garbage for restore before cluster phase changes to Running.
// @return ErrNoOps if no operation
// Deprecated: to be removed by PITR feature.
func (c *ComponentBase) HandleGarbageOfRestoreBeforeRunning() error {
	clusterBackupResourceMap, err := getClusterBackupSourceMap(c.GetCluster())
	if err != nil {
		return err
	}
	if clusterBackupResourceMap == nil {
		return nil
	}
	if c.phase() != appsv1alpha1.RunningClusterCompPhase {
		return nil
	}

	// remove the garbage for restore if the component restores from backup.
	for _, v := range clusterBackupResourceMap {
		// remove the init container for restore
		if err = c.removeStsInitContainerForRestore(v); err != nil {
			return err
		}
	}
	return nil
}

// removeStsInitContainerForRestore removes the statefulSet's init container which restores data from backup.
func (c *ComponentBase) removeStsInitContainerForRestore(backupName string) error {
	doRemoveInitContainers := false
	for _, vertex := range c.WorkloadVertexs {
		sts := vertex.Obj.(*appsv1.StatefulSet)
		initContainers := sts.Spec.Template.Spec.InitContainers
		restoreInitContainerName := component.GetRestoredInitContainerName(backupName)
		restoreInitContainerIndex, _ := intctrlutil.GetContainerByName(initContainers, restoreInitContainerName)
		if restoreInitContainerIndex == -1 {
			continue
		}
		doRemoveInitContainers = true
		initContainers = append(initContainers[:restoreInitContainerIndex], initContainers[restoreInitContainerIndex+1:]...)
		sts.Spec.Template.Spec.InitContainers = initContainers

		if *vertex.Action != ictrltypes.UPDATE {
			if *vertex.Action != ictrltypes.CREATE && *vertex.Action != ictrltypes.DELETE {
				vertex.Action = ictrltypes.ActionUpdatePtr()
			}
		}
	}
	if doRemoveInitContainers {
		// if need to remove init container, reset component to Creating.
		c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase)
	}
	return nil
}

// hasFailedAndTimedOutPod returns whether the pod of components is still failed after a PodFailedTimeout period.
// if return ture, component phase will be set to Failed/Abnormal.
func hasFailedAndTimedOutPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, error) {
	hasTimedoutPod := false
	messages := appsv1alpha1.ComponentMessageMap{}
	for _, pod := range pods {
		isFailed, isTimedOut, messageStr := isPodFailedAndTimedOut(pod)
		if !isFailed {
			continue
		}
		if isTimedOut {
			hasTimedoutPod = true
			messages.SetObjectMessage(pod.Kind, pod.Name, messageStr)
		}
	}
	return hasTimedoutPod, messages, nil
}

// isPodFailedAndTimedOut checks if the pod is failed and timed out.
func isPodFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
	initContainerFailed, message := isAnyContainerFailed(pod.Status.InitContainerStatuses)
	if initContainerFailed {
		return initContainerFailed, isContainerFailedAndTimedOut(pod, corev1.PodInitialized), message
	}
	containerFailed, message := isAnyContainerFailed(pod.Status.ContainerStatuses)
	if containerFailed {
		return containerFailed, isContainerFailedAndTimedOut(pod, corev1.ContainersReady), message
	}
	return false, false, ""
}

// isAnyContainerFailed checks whether any container in the list is failed.
func isAnyContainerFailed(containersStatus []corev1.ContainerStatus) (bool, string) {
	for _, v := range containersStatus {
		waitingState := v.State.Waiting
		if waitingState != nil && waitingState.Message != "" {
			return true, waitingState.Message
		}
		terminatedState := v.State.Terminated
		if terminatedState != nil && terminatedState.Message != "" {
			return true, terminatedState.Message
		}
	}
	return false, ""
}

// isContainerFailedAndTimedOut checks whether the failed container has timed out.
func isContainerFailedAndTimedOut(pod *corev1.Pod, podConditionType corev1.PodConditionType) bool {
	containerReadyCondition := intctrlutil.GetPodCondition(&pod.Status, podConditionType)
	if containerReadyCondition == nil || containerReadyCondition.LastTransitionTime.IsZero() {
		return false
	}
	return time.Now().After(containerReadyCondition.LastTransitionTime.Add(PodContainerFailedTimeout))
}

// updateComponentStatusMessage updates component status message map
func updateComponentStatusMessage(cluster *appsv1alpha1.Cluster,
	compName string,
	compStatus *appsv1alpha1.ClusterComponentStatus,
	event *corev1.Event) {
	var (
		kind = event.InvolvedObject.Kind
		name = event.InvolvedObject.Name
	)
	message := compStatus.GetObjectMessage(kind, name)
	// if the event message is not exists in message map, merge them.
	if !strings.Contains(message, event.Message) {
		message += event.Message + ";"
	}
	compStatus.SetObjectMessage(kind, name, message)
	cluster.Status.SetComponentStatus(compName, *compStatus)
}

// isExistsEventMsg checks whether the event is exists
func isExistsEventMsg(compStatusMessage map[string]string, event *corev1.Event) bool {
	if compStatusMessage == nil {
		return false
	}
	messageKey := util.GetComponentStatusMessageKey(event.InvolvedObject.Kind, event.InvolvedObject.Name)
	if message, ok := compStatusMessage[messageKey]; !ok {
		return false
	} else {
		return strings.Contains(message, event.Message)
	}

}

// getClusterBackupSourceMap gets the backup source map from cluster.annotations
func getClusterBackupSourceMap(cluster *appsv1alpha1.Cluster) (map[string]string, error) {
	compBackupMapString := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	if len(compBackupMapString) == 0 {
		return nil, nil
	}
	compBackupMap := map[string]string{}
	err := json.Unmarshal([]byte(compBackupMapString), &compBackupMap)
	return compBackupMap, err
}
