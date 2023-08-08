/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package components

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type componentBase struct {
	Client         client.Client
	Recorder       record.EventRecorder
	Cluster        *appsv1alpha1.Cluster
	ClusterVersion *appsv1alpha1.ClusterVersion    // building config needs the cluster version
	Component      *component.SynthesizedComponent // built synthesized component, replace it with component workload proto
	ComponentSet   componentSet
	Dag            *graph.DAG
	WorkloadVertex *ictrltypes.LifecycleVertex // DAG vertex of main workload object
}

func (c *componentBase) GetName() string {
	return c.Component.Name
}

func (c *componentBase) GetNamespace() string {
	return c.Cluster.Namespace
}

func (c *componentBase) GetClusterName() string {
	return c.Cluster.Name
}

func (c *componentBase) GetDefinitionName() string {
	return c.Component.ComponentDef
}

func (c *componentBase) GetCluster() *appsv1alpha1.Cluster {
	return c.Cluster
}

func (c *componentBase) GetClusterVersion() *appsv1alpha1.ClusterVersion {
	return c.ClusterVersion
}

func (c *componentBase) GetSynthesizedComponent() *component.SynthesizedComponent {
	return c.Component
}

func (c *componentBase) GetConsensusSpec() *appsv1alpha1.ConsensusSetSpec {
	return c.Component.ConsensusSpec
}

func (c *componentBase) GetMatchingLabels() client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
}

func (c *componentBase) GetPhase() appsv1alpha1.ClusterComponentPhase {
	if c.Cluster.Status.Components == nil {
		return ""
	}
	if _, ok := c.Cluster.Status.Components[c.GetName()]; !ok {
		return ""
	}
	return c.Cluster.Status.Components[c.GetName()].Phase
}

func (c *componentBase) SetWorkload(obj client.Object, action *ictrltypes.LifecycleAction, parent *ictrltypes.LifecycleVertex) {
	c.WorkloadVertex = c.AddResource(obj, action, parent)
}

func (c *componentBase) AddResource(obj client.Object, action *ictrltypes.LifecycleAction,
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

func (c *componentBase) CreateResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectCreate(c.Dag, obj, parent)
}

func (c *componentBase) DeleteResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectDelete(c.Dag, obj, parent)
}

func (c *componentBase) UpdateResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectUpdate(c.Dag, obj, parent)
}

func (c *componentBase) PatchResource(obj client.Object, objCopy client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectPatch(c.Dag, obj, objCopy, parent)
}

func (c *componentBase) NoopResource(obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectNoop(c.Dag, obj, parent)
}

// ValidateObjectsAction validates the action of objects in dag has been determined.
func (c *componentBase) ValidateObjectsAction() error {
	for _, v := range c.Dag.Vertices() {
		node, ok := v.(*ictrltypes.LifecycleVertex)
		if !ok {
			return fmt.Errorf("unexpected vertex type, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		}
		if node.Obj == nil {
			return fmt.Errorf("unexpected nil vertex object, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		}
		if node.Action == nil {
			return fmt.Errorf("unexpected nil vertex action, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		}
	}
	return nil
}

// ResolveObjectsAction resolves the action of objects in dag to guarantee that all object actions will be determined.
func (c *componentBase) ResolveObjectsAction(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	snapshot, err := readCacheSnapshot(reqCtx, cli, c.GetCluster())
	if err != nil {
		return err
	}
	for _, v := range c.Dag.Vertices() {
		node, ok := v.(*ictrltypes.LifecycleVertex)
		if !ok {
			return fmt.Errorf("unexpected vertex type, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		}
		if node.Action == nil {
			if action, err := resolveObjectAction(snapshot, node); err != nil {
				return err
			} else {
				node.Action = action
			}
		}
	}
	if c.GetCluster().IsStatusUpdating() {
		for _, vertex := range c.Dag.Vertices() {
			v, _ := vertex.(*ictrltypes.LifecycleVertex)
			// TODO(refactor): fix me, this is a workaround for h-scaling to update stateful set.
			if _, ok := v.Obj.(*appsv1.StatefulSet); !ok {
				v.Immutable = true
			}
		}
	}
	return c.ValidateObjectsAction()
}

func (c *componentBase) UpdatePDB(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	pdbObjList, err := listObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PodDisruptionBudgetSignature, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	for _, v := range ictrltypes.FindAll[*policyv1.PodDisruptionBudget](c.Dag) {
		node := v.(*ictrltypes.LifecycleVertex)
		pdbProto := node.Obj.(*policyv1.PodDisruptionBudget)

		if pos := slices.IndexFunc(pdbObjList, func(pdbObj *policyv1.PodDisruptionBudget) bool {
			return pdbObj.GetName() == pdbProto.GetName()
		}); pos < 0 {
			node.Action = ictrltypes.ActionCreatePtr() // TODO: Create or Noop?
		} else {
			pdbObj := pdbObjList[pos]
			if !reflect.DeepEqual(pdbObj.Spec, pdbProto.Spec) {
				pdbObj.Spec = pdbProto.Spec
				node.Obj = pdbObj
				node.Action = ictrltypes.ActionUpdatePtr()
			}
		}
	}
	return nil
}

func (c *componentBase) UpdateService(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	svcObjList, err := listObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.ServiceSignature, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	svcProtoList := ictrltypes.FindAll[*corev1.Service](c.Dag)

	// create new services or update existing services
	for _, vertex := range svcProtoList {
		node, _ := vertex.(*ictrltypes.LifecycleVertex)
		svcProto, _ := node.Obj.(*corev1.Service)

		if pos := slices.IndexFunc(svcObjList, func(svc *corev1.Service) bool {
			return svc.GetName() == svcProto.GetName()
		}); pos < 0 {
			node.Action = ictrltypes.ActionCreatePtr()
		} else {
			svcObj := svcObjList[pos]
			// remove original monitor annotations
			if len(svcObj.Annotations) > 0 {
				maps.DeleteFunc(svcObj.Annotations, func(k, v string) bool {
					return strings.HasPrefix(k, "monitor.kubeblocks.io")
				})
			}
			mergeAnnotations(svcObj.Annotations, &svcProto.Annotations)
			svcObj.Annotations = svcProto.Annotations
			svcObj.Spec = svcProto.Spec
			node.Obj = svcObj
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

// SetStatusPhase sets the cluster component phase and messages conditionally.
func (c *componentBase) SetStatusPhase(phase appsv1alpha1.ClusterComponentPhase,
	statusMessage appsv1alpha1.ComponentMessageMap, phaseTransitionMsg string) {
	updatefn := func(status *appsv1alpha1.ClusterComponentStatus) error {
		if status.Phase == phase {
			return nil
		}
		status.Phase = phase
		if status.Message == nil {
			status.Message = statusMessage
		} else {
			for k, v := range statusMessage {
				status.Message[k] = v
			}
		}
		return nil
	}
	if err := c.updateStatus(phaseTransitionMsg, updatefn); err != nil {
		panic(fmt.Sprintf("unexpected error occurred while updating component status: %s", err.Error()))
	}
}

func (c *componentBase) StatusWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client, obj client.Object, txn *statusReconciliationTxn) error {
	// if reflect.ValueOf(obj).Kind() == reflect.Ptr && reflect.ValueOf(obj).IsNil() {
	//	return nil
	// }

	pods, err := listPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return err
	}

	isRunning, err := c.ComponentSet.IsRunning(reqCtx.Ctx, obj)
	if err != nil {
		return err
	}

	var podsReady *bool
	if c.Component.Replicas > 0 {
		podsReadyForComponent, err := c.ComponentSet.PodsReady(reqCtx.Ctx, obj)
		if err != nil {
			return err
		}
		podsReady = &podsReadyForComponent
	}

	hasFailedPodTimedOut := false
	timedOutPodStatusMessage := appsv1alpha1.ComponentMessageMap{}
	var delayedRequeueError error
	isLatestWorkload := obj.GetAnnotations()[constant.KubeBlocksGenerationKey] == strconv.FormatInt(c.Cluster.Generation, 10)
	// check if it is the latest obj after cluster does updates.
	if !isRunning && !appsv1alpha1.ComponentPodsAreReady(podsReady) && isLatestWorkload {
		var requeueAfter time.Duration
		if hasFailedPodTimedOut, timedOutPodStatusMessage, requeueAfter = hasFailedAndTimedOutPod(pods); requeueAfter != 0 {
			delayedRequeueError = intctrlutil.NewDelayedRequeueError(requeueAfter, "requeue for workload status to reconcile.")
		}
	}

	phase, statusMessage, err := c.buildStatus(reqCtx.Ctx, pods, isRunning, podsReady, hasFailedPodTimedOut, timedOutPodStatusMessage)
	if err != nil {
		if !intctrlutil.IsDelayedRequeueError(err) {
			return err
		}
		delayedRequeueError = err
	}

	phaseTransitionCondMsg := ""
	if podsReady == nil {
		phaseTransitionCondMsg = fmt.Sprintf("Running: %v, PodsReady: nil, PodsTimedout: %v", isRunning, hasFailedPodTimedOut)
	} else {
		phaseTransitionCondMsg = fmt.Sprintf("Running: %v, PodsReady: %v, PodsTimedout: %v", isRunning, *podsReady, hasFailedPodTimedOut)
	}

	updatefn := func(status *appsv1alpha1.ClusterComponentStatus) error {
		if phase != "" {
			status.Phase = phase
		}
		status.SetMessage(statusMessage)
		if !appsv1alpha1.ComponentPodsAreReady(podsReady) {
			status.PodsReadyTime = nil
		} else if !appsv1alpha1.ComponentPodsAreReady(status.PodsReady) {
			// set podsReadyTime when pods of component are ready at the moment.
			status.PodsReadyTime = &metav1.Time{Time: time.Now()}
		}
		status.PodsReady = podsReady
		return nil
	}

	if txn != nil {
		txn.propose(phase, func() {
			if err = c.updateStatus(phaseTransitionCondMsg, updatefn); err != nil {
				panic(fmt.Sprintf("unexpected error occurred while updating component status: %s", err.Error()))
			}
		})
		return delayedRequeueError
	}
	// TODO(refactor): wait = true to requeue.
	if err = c.updateStatus(phaseTransitionCondMsg, updatefn); err != nil {
		return err
	}
	return delayedRequeueError
}

func (c *componentBase) buildStatus(ctx context.Context, pods []*corev1.Pod, isRunning bool, podsReady *bool,
	hasFailedPodTimedOut bool, timedOutPodStatusMessage appsv1alpha1.ComponentMessageMap) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error) {
	var (
		err           error
		phase         appsv1alpha1.ClusterComponentPhase
		statusMessage appsv1alpha1.ComponentMessageMap
	)
	if isRunning {
		if c.Component.Replicas == 0 {
			// if replicas number of component is zero, the component has stopped.
			// 'Stopped' is a special 'Running' status for workload(StatefulSet/Deployment).
			phase = appsv1alpha1.StoppedClusterCompPhase
		} else {
			// change component phase to Running when workloads of component are running.
			phase = appsv1alpha1.RunningClusterCompPhase
		}
		return phase, statusMessage, nil
	}

	if appsv1alpha1.ComponentPodsAreReady(podsReady) {
		// check if the role probe timed out when component phase is not Running but all pods of component are ready.
		phase, statusMessage = c.ComponentSet.GetPhaseWhenPodsReadyAndProbeTimeout(pods)
		// if component is not running and probe is not timed out, requeue.
		if phase == "" {
			c.Recorder.Event(c.Cluster, corev1.EventTypeNormal, "WaitingForProbeSuccess", "Waiting for probe success")
			return phase, statusMessage, intctrlutil.NewDelayedRequeueError(time.Second*10, "Waiting for probe success")
		}
		return phase, statusMessage, nil
	}

	// get the phase if failed pods have timed out or the pods are not running when there are no changes to the component.
	originPhaseIsUpRunning := slices.Contains(appsv1alpha1.GetComponentUpRunningPhase(), c.GetPhase())
	if hasFailedPodTimedOut || originPhaseIsUpRunning {
		phase, statusMessage, err = c.ComponentSet.GetPhaseWhenPodsNotReady(ctx, c.GetName(), originPhaseIsUpRunning)
		if err != nil {
			return "", nil, err
		}
	}
	if statusMessage == nil {
		statusMessage = timedOutPodStatusMessage
	} else {
		for k, v := range timedOutPodStatusMessage {
			statusMessage[k] = v
		}
	}
	return phase, statusMessage, nil
}

// updateStatus updates the cluster component status by @updatefn, with additional message to explain the transition occurred.
func (c *componentBase) updateStatus(phaseTransitionMsg string, updatefn func(status *appsv1alpha1.ClusterComponentStatus) error) error {
	if updatefn == nil {
		return nil
	}

	if c.Cluster.Status.Components == nil {
		c.Cluster.Status.Components = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	if _, ok := c.Cluster.Status.Components[c.GetName()]; !ok {
		c.Cluster.Status.Components[c.GetName()] = appsv1alpha1.ClusterComponentStatus{}
	}

	status := c.Cluster.Status.Components[c.GetName()]
	phase := status.Phase
	err := updatefn(&status)
	if err != nil {
		return err
	}
	c.Cluster.Status.Components[c.GetName()] = status

	if phase != status.Phase {
		// TODO: logging the event
		if c.Recorder != nil {
			c.Recorder.Eventf(c.Cluster, corev1.EventTypeNormal, ComponentPhaseTransition, phaseTransitionMsg)
		}
	}

	return nil
}

// hasFailedAndTimedOutPod returns whether the pods of components are still failed after a PodFailedTimeout period.
// if return true, component phase will be set to Failed/Abnormal.
func hasFailedAndTimedOutPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, time.Duration) {
	var (
		hasTimedOutPod bool
		messages       = appsv1alpha1.ComponentMessageMap{}
		hasFailedPod   bool
		requeueAfter   time.Duration
	)
	for _, pod := range pods {
		isFailed, isTimedOut, messageStr := IsPodFailedAndTimedOut(pod)
		if !isFailed {
			continue
		}
		if isTimedOut {
			hasTimedOutPod = true
			messages.SetObjectMessage(pod.Kind, pod.Name, messageStr)
		} else {
			hasFailedPod = true
		}
	}
	if hasFailedPod && !hasTimedOutPod {
		requeueAfter = PodContainerFailedTimeout
	}
	return hasTimedOutPod, messages, requeueAfter
}

// isPodScheduledFailedAndTimedOut checks whether the unscheduled pod has timed out.
func isPodScheduledFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
	for _, cond := range pod.Status.Conditions {
		if cond.Type != corev1.PodScheduled {
			continue
		}
		if cond.Status == corev1.ConditionTrue {
			return false, false, ""
		}
		return true, time.Now().After(cond.LastTransitionTime.Add(PodScheduledFailedTimeout)), cond.Message
	}
	return false, false, ""
}

// IsPodFailedAndTimedOut checks if the pod is failed and timed out.
func IsPodFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
	if isFailed, isTimedOut, message := isPodScheduledFailedAndTimedOut(pod); isFailed {
		return isFailed, isTimedOut, message
	}
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

type gvkName struct {
	gvk      schema.GroupVersionKind
	ns, name string
}

type clusterSnapshot map[gvkName]client.Object

func getGVKName(object client.Object, scheme *runtime.Scheme) (*gvkName, error) {
	gvk, err := apiutil.GVKForObject(object, scheme)
	if err != nil {
		return nil, err
	}
	return &gvkName{
		gvk:  gvk,
		ns:   object.GetNamespace(),
		name: object.GetName(),
	}, nil
}

func isOwnerOf(owner, obj client.Object, scheme *runtime.Scheme) bool {
	ro, ok := owner.(runtime.Object)
	if !ok {
		return false
	}
	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return false
	}
	ref := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        owner.GetUID(),
		Name:       owner.GetName(),
	}
	owners := obj.GetOwnerReferences()
	referSameObject := func(a, b metav1.OwnerReference) bool {
		aGV, err := schema.ParseGroupVersion(a.APIVersion)
		if err != nil {
			return false
		}

		bGV, err := schema.ParseGroupVersion(b.APIVersion)
		if err != nil {
			return false
		}

		return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
	}
	for _, ownerRef := range owners {
		if referSameObject(ownerRef, ref) {
			return true
		}
	}
	return false
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&appsv1.StatefulSetList{},
		&appsv1.DeploymentList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&corev1.PersistentVolumeClaimList{}, // TODO(merge): remove it?
		&policyv1.PodDisruptionBudgetList{},
		&dataprotectionv1alpha1.BackupPolicyList{},
	}
}

// read all objects owned by component
func readCacheSnapshot(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster) (clusterSnapshot, error) {
	// list what kinds of object cluster owns
	kinds := ownedKinds()
	snapshot := make(clusterSnapshot)
	ml := client.MatchingLabels{constant.AppInstanceLabelKey: cluster.GetName()}
	inNS := client.InNamespace(cluster.Namespace)
	for _, list := range kinds {
		if err := cli.List(reqCtx.Ctx, list, inNS, ml); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			// put to snapshot if owned by our cluster
			if isOwnerOf(cluster, object, k8sscheme.Scheme) {
				name, err := getGVKName(object, k8sscheme.Scheme)
				if err != nil {
					return nil, err
				}
				snapshot[*name] = object
			}
		}
	}
	return snapshot, nil
}

func resolveObjectAction(snapshot clusterSnapshot, vertex *ictrltypes.LifecycleVertex) (*ictrltypes.LifecycleAction, error) {
	gvk, err := getGVKName(vertex.Obj, k8sscheme.Scheme)
	if err != nil {
		return nil, err
	}
	if _, ok := snapshot[*gvk]; ok {
		return ictrltypes.ActionNoopPtr(), nil
	} else {
		return ictrltypes.ActionCreatePtr(), nil
	}
}
