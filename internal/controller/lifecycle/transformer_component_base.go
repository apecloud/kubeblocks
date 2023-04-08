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
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	appsv1 "k8s.io/api/apps/v1"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

// TODO(refactor): copy from components.type, cleanup it
const (
	// PodContainerFailedTimeout the timeout for container of pod failures, the component phase will be set to Failed/Abnormal after this time.
	PodContainerFailedTimeout = time.Minute
)

type componentBase struct {
	client client.Client

	Definition *appsv1alpha1.ClusterDefinition
	Version    *appsv1alpha1.ClusterVersion
	Cluster    *appsv1alpha1.Cluster

	// TODO(refactor): should remove those members in future.
	CompDef  *appsv1alpha1.ClusterComponentDefinition
	CompVer  *appsv1alpha1.ClusterComponentVersion
	CompSpec *appsv1alpha1.ClusterComponentSpec

	// built synthesized component
	Component *component.SynthesizedComponent

	componentSet ComponentSet

	// DAG vertexes of main workload object(s)
	workloadVertexs []*ictrltypes.LifecycleVertex

	dag *graph.DAG
}

func (c *componentBase) GetClient() client.Client {
	return c.client
}

func (c *componentBase) GetName() string {
	return c.CompSpec.Name
}

func (c *componentBase) GetNamespace() string {
	return c.Cluster.Namespace
}

func (c *componentBase) GetClusterName() string {
	return c.Cluster.Name
}

func (c *componentBase) GetDefinition() *appsv1alpha1.ClusterDefinition {
	return c.Definition
}

func (c *componentBase) GetVersion() *appsv1alpha1.ClusterVersion {
	return c.Version
}

func (c *componentBase) GetCluster() *appsv1alpha1.Cluster {
	return c.Cluster
}

func (c *componentBase) GetSynthesizedComponent() *component.SynthesizedComponent {
	return c.Component
}

func (c *componentBase) GetMatchingLabels() client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
}

func (c *componentBase) addResource(obj client.Object, action *ictrltypes.LifecycleAction,
	parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	if obj == nil {
		panic("try to add nil object")
	}
	vertex := &ictrltypes.LifecycleVertex{
		Obj:    obj,
		Action: action,
	}
	c.dag.AddVertex(vertex)

	if parent != nil {
		c.dag.Connect(parent, vertex)
	}
	return vertex
}

func (c *componentBase) addWorkload(obj client.Object, action *ictrltypes.LifecycleAction, parent *ictrltypes.LifecycleVertex) {
	c.workloadVertexs = append(c.workloadVertexs, c.addResource(obj, action, parent))
}

func (c *componentBase) createResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectCreate(c.dag, obj, parent)
}

func (c *componentBase) deleteResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectDelete(c.dag, obj, parent)
}

func (c *componentBase) updateResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectUpdate(c.dag, obj, parent)
}

func (c *componentBase) noopResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectNoop(c.dag, obj, parent)
}

// validateObjectsAction validates the action of all objects in dag has been determined
func (c *componentBase) validateObjectsAction() error {
	for _, v := range c.dag.Vertices() {
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
func (c *componentBase) resolveObjectsAction(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	snapshot, err := readCacheSnapshot(reqCtx, cli, c.GetCluster())
	if err != nil {
		return err
	}
	for _, v := range c.dag.Vertices() {
		if node, ok := v.(*ictrltypes.LifecycleVertex); !ok {
			return fmt.Errorf("unexpected vertex type, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		} else if node.Action == nil {
			if action, err := resolveObjectAction(snapshot, node); err != nil {
				return err
			} else {
				node.Action = action
			}
		}
	}
	if isClusterStatusUpdating(*c.GetCluster()) {
		for _, vertex := range c.dag.Vertices() {
			v, _ := vertex.(*ictrltypes.LifecycleVertex)
			// TODO(refactor): fix me, workaround for h-scaling to update stateful set
			if _, ok := v.Obj.(*appsv1.StatefulSet); !ok {
				v.Immutable = true
			}
		}
	}
	return c.validateObjectsAction()
}

func (c *componentBase) composeSynthesizedComponent(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, cli, *c.Cluster, *c.Definition, *c.CompDef, *c.CompSpec, c.CompVer)
	if err != nil {
		return err
	}
	c.Component = synthesizedComp
	return nil
}

func (c *componentBase) updateService(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	labels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
	svcObjList, err := listObjWithLabelsInNamespace(reqCtx, cli, generics.ServiceSignature, c.GetNamespace(), labels)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	svcProtoList := findAll[*corev1.Service](c.dag)

	// create new services or update existed services
	for _, vertex := range svcProtoList {
		node, _ := vertex.(*ictrltypes.LifecycleVertex)
		svcProto, _ := node.Obj.(*corev1.Service)

		if pos := slices.IndexFunc(svcObjList, func(svc *corev1.Service) bool {
			return svc.GetName() == svcProto.GetName()
		}); pos < 0 {
			node.Action = ictrltypes.ActionCreatePtr()
		} else {
			svcProto.Annotations = mergeServiceAnnotations(svcObjList[pos].Annotations, svcProto.Annotations)
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
			c.deleteResource(svc, nil)
		}
	}
	return nil
}

func (c *componentBase) updateStatus(updatefn func(status *appsv1alpha1.ClusterComponentStatus)) {
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

func (c *componentBase) setStatusPhase(phase appsv1alpha1.ClusterComponentPhase) {
	c.setStatusPhaseWithMsg(phase, "", "")
}

func (c *componentBase) setStatusPhaseWithMsg(phase appsv1alpha1.ClusterComponentPhase, msgKey, msg string) {
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

func (c *componentBase) syncComponentStatusForEvent(phase appsv1alpha1.ClusterComponentPhase, event *corev1.Event) {
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

func (c *componentBase) phase() appsv1alpha1.ClusterComponentPhase {
	if c.Cluster.Status.Components == nil {
		return ""
	}
	if _, ok := c.Cluster.Status.Components[c.GetName()]; !ok {
		return ""
	}
	return c.Cluster.Status.Components[c.GetName()].Phase
}

func (c *componentBase) status(reqCtx intctrlutil.RequestCtx, cli client.Client, objs []client.Object) error {
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
	//phase, err := c.componentSet.GetPhaseWhenPodsNotReady(reqCtx.Ctx, c.GetName())
	//if err != nil {
	//	return err
	//}
	//// TODO(refactor): event -> status message
	//event := &corev1.Event{}
	//c.syncComponentStatusForEvent(phase, event)

	for _, obj := range objs {
		///// WorkloadController.handleWorkloadUpdate
		// patch role labels and update roles in component status
		if vertexes, err := c.componentSet.HandleRoleChange(reqCtx.Ctx, obj); err != nil {
			return err
		} else if vertexes != nil {
			for v := range vertexes {
				c.dag.AddVertex(v)
			}
		}

		// restart pod if needed
		if vertexes, err := c.componentSet.HandleRestart(reqCtx.Ctx, obj); err != nil {
			return err
		} else if vertexes != nil {
			for v := range vertexes {
				c.dag.AddVertex(v)
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

func (c *componentBase) rebuildLatestStatus(reqCtx intctrlutil.RequestCtx, cli client.Client, obj client.Object) (*appsv1alpha1.ClusterComponentStatus, error) {
	pods, err := util.ListPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}

	isRunning, err := c.componentSet.IsRunning(reqCtx.Ctx, obj)
	if err != nil {
		return nil, err
	}

	var podsReady *bool
	if c.Component.Replicas > 0 {
		podsReadyForComponent, err := c.componentSet.PodsReady(reqCtx.Ctx, obj)
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
			c.componentSet.HandleProbeTimeoutWhenPodsReady(pstatus, pods)
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
func (c *componentBase) rebuildComponentStatus(reqCtx intctrlutil.RequestCtx,
	running bool,
	podsAreReady *bool,
	hasFailedPodTimedOut bool,
	status *appsv1alpha1.ClusterComponentStatus) error {
	if !running {
		// if no operation is running in cluster or failed pod timed out,
		// means the component is Failed or Abnormal.
		if slices.Contains(appsv1alpha1.GetClusterUpRunningPhases(), c.Cluster.Status.Phase) || hasFailedPodTimedOut {
			if phase, err := c.componentSet.GetPhaseWhenPodsNotReady(reqCtx.Ctx, c.GetName()); err != nil {
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
func (c *componentBase) handleGarbageOfRestoreBeforeRunning() error {
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
func (c *componentBase) removeStsInitContainerForRestore(backupName string) error {
	doRemoveInitContainers := false
	for _, vertex := range c.workloadVertexs {
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
		c.setStatusPhase(appsv1alpha1.CreatingClusterCompPhase)
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
