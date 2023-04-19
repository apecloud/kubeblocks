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

package internal

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type ComponentBase struct {
	Client          client.Client
	Recorder        record.EventRecorder
	Cluster         *appsv1alpha1.Cluster
	ClusterVersion  *appsv1alpha1.ClusterVersion    // building config needs the cluster version
	Component       *component.SynthesizedComponent // built synthesized component, replace it with component workload proto
	ComponentSet    types.ComponentSet
	Dag             *graph.DAG
	WorkloadVertexs []*ictrltypes.LifecycleVertex // DAG vertexes of main workload object(s)
}

func (c *ComponentBase) GetName() string {
	return c.Component.Name
}

func (c *ComponentBase) GetNamespace() string {
	return c.Cluster.Namespace
}

func (c *ComponentBase) GetClusterName() string {
	return c.Cluster.Name
}

func (c *ComponentBase) GetCluster() *appsv1alpha1.Cluster {
	return c.Cluster
}

func (c *ComponentBase) GetClusterVersion() *appsv1alpha1.ClusterVersion {
	return c.ClusterVersion
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

func (c *ComponentBase) GetReplicas() int32 {
	return c.Component.Replicas
}

func (c *ComponentBase) GetConsensusSpec() *appsv1alpha1.ConsensusSetSpec {
	return c.Component.ConsensusSpec
}

func (c *ComponentBase) GetPrimaryIndex() int32 {
	return *c.Component.PrimaryIndex
}

func (c *ComponentBase) GetPhase() appsv1alpha1.ClusterComponentPhase {
	if c.Cluster.Status.Components == nil {
		return ""
	}
	if _, ok := c.Cluster.Status.Components[c.GetName()]; !ok {
		return ""
	}
	return c.Cluster.Status.Components[c.GetName()].Phase
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

// ValidateObjectsAction validates the action of objects in dag has been determined.
func (c *ComponentBase) ValidateObjectsAction() error {
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
func (c *ComponentBase) ResolveObjectsAction(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

// SetStatusPhase set the cluster component phase to @phase conditionally.
func (c *ComponentBase) SetStatusPhase(phase appsv1alpha1.ClusterComponentPhase, message string) {
	c.setStatusPhaseWithMsg(phase, "", "", message)
}

// setStatusPhaseWithMsg set the cluster component phase and messages to specified conditionally.
func (c *ComponentBase) setStatusPhaseWithMsg(phase appsv1alpha1.ClusterComponentPhase, statusMsgKey, statusMsg, phaseTransitionMsg string) {
	if err := c.updateStatus(func(status *appsv1alpha1.ClusterComponentStatus) error {
		if status.Phase == phase {
			return nil
		}

		// TODO(impl): define the status phase transition diagram
		if phase == appsv1alpha1.SpecReconcilingClusterCompPhase && status.Phase != appsv1alpha1.RunningClusterCompPhase {
			return nil
		}

		status.Phase = phase
		if statusMsgKey != "" {
			if status.Message == nil {
				status.Message = map[string]string{}
			}
			status.Message[statusMsgKey] = statusMsg
		}
		return nil
	}, phaseTransitionMsg); err != nil {
		panic(fmt.Sprintf("unexpected error occurred: %s", err.Error()))
	}
}

// updateStatus updates the cluster component status by @updatefn, with additional message to explain the transition occurred.
func (c *ComponentBase) updateStatus(updatefn func(status *appsv1alpha1.ClusterComponentStatus) error, message string) error {
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
			c.Recorder.Eventf(c.Cluster, corev1.EventTypeNormal, types.ComponentPhaseTransition, message)
		}
	}

	return nil
}

func (c *ComponentBase) BuildLatestStatus(reqCtx intctrlutil.RequestCtx, cli client.Client, obj client.Object) error {
	// TODO(refactor): should review it again
	if reflect.ValueOf(obj).Kind() == reflect.Ptr && reflect.ValueOf(obj).IsNil() {
		return nil
	}

	pods, err := util.ListPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
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
	if !isRunning && (podsReady == nil || !*podsReady) {
		hasFailedPodTimedOut, timedOutPodStatusMessage = hasFailedAndTimedOutPod(pods)
	}

	phaseTransitionCondMsg := ""
	if podsReady == nil {
		phaseTransitionCondMsg = fmt.Sprintf("Running: %v, PodsReady: nil, PodsTimedout: %v", isRunning, hasFailedPodTimedOut)
	} else {
		phaseTransitionCondMsg = fmt.Sprintf("Running: %v, PodsReady: %v, PodsTimedout: %v", isRunning, *podsReady, hasFailedPodTimedOut)
	}

	updatefn := func(status *appsv1alpha1.ClusterComponentStatus) error {
		return c.buildStatus(reqCtx.Ctx, pods, isRunning, podsReady, hasFailedPodTimedOut, timedOutPodStatusMessage, status)
	}
	// TODO(refactor): wait = true to requeue.
	return c.updateStatus(updatefn, phaseTransitionCondMsg)
}

func (c *ComponentBase) buildStatus(ctx context.Context, pods []*corev1.Pod, isRunning bool, podsReady *bool,
	hasFailedPodTimedOut bool, timedOutPodStatusMessage appsv1alpha1.ComponentMessageMap, status *appsv1alpha1.ClusterComponentStatus) error {
	if isRunning {
		if c.Component.Replicas == 0 {
			// if replicas number of component is zero, the component has stopped.
			// 'Stopped' is a special 'Running' for workload(StatefulSet/Deployment).
			status.Phase = appsv1alpha1.StoppedClusterCompPhase
		} else {
			// change component phase to Running when workloads of component are running.
			status.Phase = appsv1alpha1.RunningClusterCompPhase
		}
		status.SetMessage(nil)
	} else {
		if podsReady != nil && *podsReady {
			// check if the role probe timed out when component phase is not Running but all pods of component are ready.
			c.ComponentSet.HandleProbeTimeoutWhenPodsReady(status, pods)
		} else {
			//// if no operation is running in cluster or failed pod timed out,
			//// means the component is Failed or Abnormal.
			// if slices.Contains(appsv1alpha1.GetClusterUpRunningPhases(), c.Cluster.Status.Phase) || hasFailedPodTimedOut {
			// TODO(refactor): should review and check this pre-condition carefully.
			status.Message = timedOutPodStatusMessage
			if hasFailedPodTimedOut {
				phase, statusMessage, err := c.ComponentSet.GetPhaseWhenPodsNotReady(ctx, c.GetName())
				if err != nil {
					return err
				}
				if phase != "" {
					status.Phase = phase
				}
				for k, v := range statusMessage {
					status.Message[k] = v
				}
			}
		}
	}
	status.PodsReady = podsReady
	if podsReady != nil && *podsReady {
		status.PodsReadyTime = &metav1.Time{Time: time.Now()}
	} else {
		status.PodsReadyTime = nil
	}
	return nil
}

// hasFailedAndTimedOutPod returns whether the pod of components is still failed after a PodFailedTimeout period.
// if return true, component phase will be set to Failed/Abnormal.
func hasFailedAndTimedOutPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap) {
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
	return hasTimedoutPod, messages
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
	return time.Now().After(containerReadyCondition.LastTransitionTime.Add(types.PodContainerFailedTimeout))
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
		&corev1.PersistentVolumeClaimList{},
		&policyv1.PodDisruptionBudgetList{},
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
	if obj, ok := snapshot[*gvk]; ok {
		vertex.ObjCopy = obj
		return ictrltypes.ActionUpdatePtr(), nil
	} else {
		return ictrltypes.ActionCreatePtr(), nil
	}
}
