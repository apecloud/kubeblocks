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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsmcore "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	lorry "github.com/apecloud/kubeblocks/pkg/lorry/client"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	// componentPhaseTransition the event reason indicates that the component transits to a new phase.
	componentPhaseTransition = "ComponentPhaseTransition"

	// podContainerFailedTimeout the timeout for container of pod failures, the component phase will be set to Failed/Abnormal after this time.
	podContainerFailedTimeout = 10 * time.Second

	// podScheduledFailedTimeout timeout for scheduling failure.
	podScheduledFailedTimeout = 30 * time.Second
)

// rsmComponent as a base class for single rsm based component (stateful & replication & consensus).
type rsmComponent struct {
	Client         client.Client
	Recorder       record.EventRecorder
	Cluster        *appsv1alpha1.Cluster
	clusterVersion *appsv1alpha1.ClusterVersion    // building config needs the cluster version
	component      *component.SynthesizedComponent // built synthesized component, replace it with component workload proto
	dag            *graph.DAG
	workload       client.Object // main workload object
	// runningWorkload can be nil, and the replicas of workload can be nil (zero)
	runningWorkload *workloads.ReplicatedStateMachine
}

var _ Component = &rsmComponent{}

func newRSMComponent(cli client.Client,
	recorder record.EventRecorder,
	cluster *appsv1alpha1.Cluster,
	clusterVersion *appsv1alpha1.ClusterVersion,
	synthesizedComponent *component.SynthesizedComponent,
	dag *graph.DAG) Component {
	comp := &rsmComponent{
		Client:         cli,
		Recorder:       recorder,
		Cluster:        cluster,
		clusterVersion: clusterVersion,
		component:      synthesizedComponent,
		dag:            dag,
		workload:       nil,
	}
	return comp
}

func (c *rsmComponent) GetName() string {
	return c.component.Name
}

func (c *rsmComponent) GetNamespace() string {
	return c.Cluster.Namespace
}

func (c *rsmComponent) GetClusterName() string {
	return c.Cluster.Name
}

func (c *rsmComponent) GetCluster() *appsv1alpha1.Cluster {
	return c.Cluster
}

func (c *rsmComponent) GetClusterVersion() *appsv1alpha1.ClusterVersion {
	return c.clusterVersion
}

func (c *rsmComponent) GetSynthesizedComponent() *component.SynthesizedComponent {
	return c.component
}

func (c *rsmComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.create(reqCtx, cli, c.newBuilder(reqCtx, cli, model.ActionCreatePtr()))
}

func (c *rsmComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.update(reqCtx, cli, c.newBuilder(reqCtx, cli, nil))
}

func (c *rsmComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(impl): delete component owned resources
	return c.newBuilder(reqCtx, cli, model.ActionNoopPtr()).BuildEnv().BuildWorkload().Complete()
}

func (c *rsmComponent) Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.status(reqCtx, cli, c.newBuilder(reqCtx, cli, nil))
}

func (c *rsmComponent) newBuilder(reqCtx intctrlutil.RequestCtx, cli client.Client,
	action *model.Action) componentWorkloadBuilder {
	return &rsmComponentWorkloadBuilder{
		reqCtx:        reqCtx,
		client:        cli,
		comp:          c,
		defaultAction: action,
		error:         nil,
		envConfig:     nil,
		workload:      nil,
	}
}

func (c *rsmComponent) setWorkload(obj client.Object, action *model.Action) {
	c.workload = obj
	graphCli := model.NewGraphClient(c.Client)
	graphCli.Root(c.dag, nil, obj, action)
}

func (c *rsmComponent) addResource(obj client.Object, action *model.Action) client.Object {
	if obj == nil {
		panic("try to add nil object")
	}
	model.NewGraphClient(c.Client).Do(c.dag, nil, obj, action, nil)
	return obj
}

func (c *rsmComponent) workloadVertex() *model.ObjectVertex {
	for _, vertex := range c.dag.Vertices() {
		v, _ := vertex.(*model.ObjectVertex)
		if v.Obj == c.workload {
			return v
		}
	}
	return nil
}

func (c *rsmComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder, load bool) error {
	var err error
	if builder != nil {
		if err = builder.BuildEnv().
			BuildWorkload().
			BuildPDB().
			BuildCustomVolumes().
			BuildConfig().
			BuildTLSVolume().
			BuildVolumeMount().
			BuildTLSCert().
			Complete(); err != nil {
			return err
		}
	}
	if load {
		c.runningWorkload, err = c.loadRunningWorkload(reqCtx, cli)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *rsmComponent) loadRunningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*workloads.ReplicatedStateMachine, error) {
	rsmList, err := listRSMOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.getMatchingLabels())
	if err != nil {
		return nil, err
	}
	cnt := len(rsmList)
	switch {
	case cnt == 0:
		return nil, nil
	case cnt == 1:
		return rsmList[0], nil
	default:
		return nil, fmt.Errorf("more than one workloads found for the component, cluster: %s, component: %s, cnt: %d",
			c.GetClusterName(), c.GetName(), cnt)
	}
}

func (c *rsmComponent) getMatchingLabels() client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
}

func (c *rsmComponent) create(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, false); err != nil {
		return err
	}

	if err := c.validateObjectsAction(); err != nil {
		return err
	}

	return nil
}

func (c *rsmComponent) update(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, true); err != nil {
		return err
	}

	if c.runningWorkload != nil {
		if err := c.restart(reqCtx, cli); err != nil {
			return err
		}

		// cluster.spec.componentSpecs[*].volumeClaimTemplates[*].spec.resources.requests[corev1.ResourceStorage]
		if err := c.expandVolume(reqCtx, cli); err != nil {
			return err
		}

		// cluster.spec.componentSpecs[*].replicas
		if err := c.horizontalScale(reqCtx, cli); err != nil {
			return err
		}
	}

	if err := c.updateUnderlyingResources(reqCtx, cli, c.runningWorkload); err != nil {
		return err
	}

	return c.resolveObjectsAction(reqCtx, cli)
}

func (c *rsmComponent) status(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, true); err != nil {
		return err
	}
	if c.runningWorkload == nil {
		return nil
	}
	c.noopAllNoneWorkloadObjects()

	isDeleting := func() bool {
		return !c.runningWorkload.DeletionTimestamp.IsZero()
	}()
	isZeroReplica := func() bool {
		return (c.runningWorkload.Spec.Replicas == nil || *c.runningWorkload.Spec.Replicas == 0) && c.component.Replicas == 0
	}()
	pods, err := listPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.getMatchingLabels())
	if err != nil {
		return err
	}
	hasComponentPod := func() bool {
		return len(pods) > 0
	}()
	isRunning, err := c.isRunning(reqCtx.Ctx, cli, c.runningWorkload)
	if err != nil {
		return err
	}
	isAllConfigSynced, err := c.isAllConfigSynced(reqCtx, cli)
	if err != nil {
		return err
	}
	hasFailedPod, messages, err := c.hasFailedPod(reqCtx, cli, pods)
	if err != nil {
		return err
	}
	isScaleOutFailed, err := c.isScaleOutFailed(reqCtx, cli)
	if err != nil {
		return err
	}
	hasRunningVolumeExpansion, hasFailedVolumeExpansion, err := c.hasVolumeExpansionRunning(reqCtx, cli)
	if err != nil {
		return err
	}
	hasFailure := func() bool {
		return hasFailedPod || isScaleOutFailed || hasFailedVolumeExpansion
	}()
	isComponentAvailable, err := c.isAvailable(reqCtx, cli, pods)
	if err != nil {
		return err
	}
	isInCreatingPhase := func() bool {
		phase := c.getComponentStatus().Phase
		return phase == "" || phase == appsv1alpha1.CreatingClusterCompPhase
	}()

	updatePodsReady := func(ready bool) {
		_ = c.updateStatus("", func(status *appsv1alpha1.ClusterComponentStatus) error {
			// if ready flag not changed, don't update the ready time
			if status.PodsReady != nil && *status.PodsReady == ready {
				return nil
			}
			status.PodsReady = &ready
			if ready {
				now := metav1.Now()
				status.PodsReadyTime = &now
			}
			return nil
		})
	}

	podsReady := false
	switch {
	case isDeleting:
		c.setStatusPhase(appsv1alpha1.DeletingClusterCompPhase, nil, "component is Deleting")
	case isZeroReplica && hasComponentPod:
		c.setStatusPhase(appsv1alpha1.StoppingClusterCompPhase, nil, "component is Stopping")
		podsReady = true
	case isZeroReplica:
		c.setStatusPhase(appsv1alpha1.StoppedClusterCompPhase, nil, "component is Stopped")
		podsReady = true
	case isRunning && isAllConfigSynced && !hasRunningVolumeExpansion:
		c.setStatusPhase(appsv1alpha1.RunningClusterCompPhase, nil, "component is Running")
		podsReady = true
	case !hasFailure && isInCreatingPhase:
		c.setStatusPhase(appsv1alpha1.CreatingClusterCompPhase, nil, "Create a new component")
	case !hasFailure:
		c.setStatusPhase(appsv1alpha1.UpdatingClusterCompPhase, nil, "component is Updating")
	case !isComponentAvailable:
		c.setStatusPhase(appsv1alpha1.FailedClusterCompPhase, messages, "component is Failed")
	default:
		c.setStatusPhase(appsv1alpha1.AbnormalClusterCompPhase, nil, "unknown")
	}
	updatePodsReady(podsReady)

	c.updateMembersStatus()

	// works should continue to be done after spec updated.
	if err := c.horizontalScale(reqCtx, cli); err != nil {
		return err
	}

	c.updateWorkload(c.runningWorkload)

	// update component info to pods' annotations
	if err := updateComponentInfoToPods(reqCtx.Ctx, cli, c.Cluster, c.component, c.dag); err != nil {
		return err
	}

	// patch the current componentSpec workload's custom labels
	if err := updateCustomLabelToPods(reqCtx.Ctx, cli, c.Cluster, c.component, c.dag); err != nil {
		reqCtx.Event(c.Cluster, corev1.EventTypeWarning, "component Workload Controller PatchWorkloadCustomLabelFailed", err.Error())
		return err
	}

	// set primary-pod annotation
	// TODO(free6om): primary-pod is only used in redis to bootstrap the redis cluster correctly.
	// it is too hacky to be replaced by a better design.
	if err := c.updatePrimaryIndex(reqCtx.Ctx, cli); err != nil {
		return err
	}

	graphCli := model.NewGraphClient(c.Client)
	if graphCli.IsAction(c.dag, c.workload, nil) {
		graphCli.Noop(c.dag, c.workload)
	}

	return nil
}

func (c *rsmComponent) updatePrimaryIndex(ctx context.Context, cli client.Client) error {
	if c.component.WorkloadType != appsv1alpha1.Replication {
		return nil
	}
	podList, err := listPodOwnedByComponent(ctx, cli, c.GetNamespace(), c.getMatchingLabels())
	if err != nil {
		return err
	}
	if len(podList) == 0 {
		return nil
	}
	slices.SortFunc(podList, func(a, b *corev1.Pod) bool {
		return a.GetName() < b.GetName()
	})
	primaryPods := make([]string, 0)
	emptyRolePods := make([]string, 0)
	for _, pod := range podList {
		role, ok := pod.Labels[constant.RoleLabelKey]
		if !ok || role == "" {
			emptyRolePods = append(emptyRolePods, pod.Name)
			continue
		}
		if role == constant.Primary {
			primaryPods = append(primaryPods, pod.Name)
		}
	}
	primaryPodName, err := func() (string, error) {
		switch {
		// if the workload is newly created, and the role label is not set, we set the pod with index=0 as the primary by default.
		case len(emptyRolePods) == len(podList):
			return podList[0].Name, nil
		case len(primaryPods) != 1:
			return "", fmt.Errorf("the number of primary pod is not equal to 1, primary pods: %v, emptyRole pods: %v", primaryPods, emptyRolePods)
		default:
			return primaryPods[0], nil
		}
	}()
	if err != nil {
		return err
	}
	graphCli := model.NewGraphClient(c.Client)
	for _, pod := range podList {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pi, ok := pod.Annotations[constant.PrimaryAnnotationKey]
		if !ok || pi != primaryPodName {
			origPod := pod.DeepCopy()
			pod.Annotations[constant.PrimaryAnnotationKey] = primaryPodName
			graphCli.Patch(c.dag, origPod, pod)
		}
	}
	return nil
}

// validateObjectsAction validates the action of objects in dag has been determined.
func (c *rsmComponent) validateObjectsAction() error {
	graphCli := model.NewGraphClient(c.Client)
	objects := graphCli.FindAll(c.dag, nil, model.HaveDifferentTypeWithOption)
	for _, object := range objects {
		if object == nil {
			return fmt.Errorf("unexpected nil object, cluster: %s, component: %s",
				c.GetClusterName(), c.GetName())
		}
		if graphCli.IsAction(c.dag, object, nil) {
			return fmt.Errorf("unexpected nil vertex action, cluster: %s, component: %s, object: %T",
				c.GetClusterName(), c.GetName(), object)
		}
	}
	return nil
}

// resolveObjectsAction resolves the action of objects in dag to guarantee that all object actions will be determined.
func (c *rsmComponent) resolveObjectsAction(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	snapshot, err := readCacheSnapshot(reqCtx, cli, c.GetCluster())
	if err != nil {
		return err
	}
	graphCli := model.NewGraphClient(c.Client)
	objects := graphCli.FindAll(c.dag, nil, model.HaveDifferentTypeWithOption)
	for _, object := range objects {
		if !graphCli.IsAction(c.dag, object, nil) {
			continue
		}
		switch action, err := resolveObjectAction(snapshot, object, cli.Scheme()); {
		case err != nil:
			return err
		case *action == model.CREATE:
			graphCli.Create(c.dag, object)
		default:
			graphCli.Noop(c.dag, object)

		}
	}
	if c.GetCluster().IsStatusUpdating() {
		// TODO(refactor): fix me, this is a workaround for h-scaling to update stateful set.
		c.noopAllNoneWorkloadObjects()
	}
	return c.validateObjectsAction()
}

func (c *rsmComponent) noopAllNoneWorkloadObjects() {
	graphCli := model.NewGraphClient(c.Client)
	objects := graphCli.FindAll(c.dag, &workloads.ReplicatedStateMachine{}, model.HaveDifferentTypeWithOption)
	for _, object := range objects {
		graphCli.Noop(c.dag, object)
	}
}

// setStatusPhase sets the cluster component phase and messages conditionally.
func (c *rsmComponent) setStatusPhase(phase appsv1alpha1.ClusterComponentPhase,
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

// updateStatus updates the cluster component status by @updatefn, with additional message to explain the transition occurred.
func (c *rsmComponent) updateStatus(phaseTransitionMsg string, updatefn func(status *appsv1alpha1.ClusterComponentStatus) error) error {
	if updatefn == nil {
		return nil
	}

	status := c.getComponentStatus()
	phase := status.Phase
	err := updatefn(&status)
	if err != nil {
		return err
	}
	c.Cluster.Status.Components[c.GetName()] = status

	if phase != status.Phase {
		// TODO: logging the event
		if c.Recorder != nil && phaseTransitionMsg != "" {
			c.Recorder.Eventf(c.Cluster, corev1.EventTypeNormal, componentPhaseTransition, phaseTransitionMsg)
		}
	}

	return nil
}

func (c *rsmComponent) isRunning(ctx context.Context, cli client.Client, obj client.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	rsm, ok := obj.(*workloads.ReplicatedStateMachine)
	if !ok {
		return false, nil
	}
	if isLatestRevision, err := IsComponentPodsWithLatestRevision(ctx, cli, c.Cluster, rsm); err != nil {
		return false, err
	} else if !isLatestRevision {
		return false, nil
	}

	// whether rsm is ready
	return rsmcore.IsRSMReady(rsm), nil
}

// isAvailable tells whether the component is basically available, ether working well or in a fragile state:
// 1. at least one pod is available
// 2. with latest revision
// 3. and with leader role label set
func (c *rsmComponent) isAvailable(reqCtx intctrlutil.RequestCtx, cli client.Client, pods []*corev1.Pod) (bool, error) {
	if isLatestRevision, err := IsComponentPodsWithLatestRevision(reqCtx.Ctx, cli, c.Cluster, c.runningWorkload); err != nil {
		return false, err
	} else if !isLatestRevision {
		return false, nil
	}

	shouldCheckLeader := func() bool {
		return c.component.WorkloadType == appsv1alpha1.Consensus || c.component.WorkloadType == appsv1alpha1.Replication
	}()
	hasLeaderRoleLabel := func(pod *corev1.Pod) bool {
		roleName, ok := pod.Labels[constant.RoleLabelKey]
		if !ok {
			return false
		}
		for _, replicaRole := range c.runningWorkload.Spec.Roles {
			if roleName == replicaRole.Name && replicaRole.IsLeader {
				return true
			}
		}
		return false
	}
	for _, pod := range pods {
		if !podutils.IsPodAvailable(pod, 0, metav1.Time{Time: time.Now()}) {
			continue
		}
		if !shouldCheckLeader {
			return true, nil
		}
		if hasLeaderRoleLabel(pod) {
			return true, nil
		}
	}
	return false, nil
}

func (c *rsmComponent) hasFailedPod(reqCtx intctrlutil.RequestCtx, cli client.Client, pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, error) {
	if isLatestRevision, err := IsComponentPodsWithLatestRevision(reqCtx.Ctx, cli, c.Cluster, c.runningWorkload); err != nil {
		return false, nil, err
	} else if !isLatestRevision {
		return false, nil, nil
	}

	var messages appsv1alpha1.ComponentMessageMap
	// check pod readiness
	hasFailedPod, msg, _ := hasFailedAndTimedOutPod(pods)
	if hasFailedPod {
		messages = msg
		return true, messages, nil
	}
	// check role probe
	if c.component.WorkloadType != appsv1alpha1.Consensus && c.component.WorkloadType != appsv1alpha1.Replication {
		return false, messages, nil
	}
	hasProbeTimeout := false
	for _, pod := range pods {
		if _, ok := pod.Labels[constant.RoleLabelKey]; ok {
			continue
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Type != corev1.PodReady || condition.Status != corev1.ConditionTrue {
				continue
			}
			podsReadyTime := &condition.LastTransitionTime
			if isProbeTimeout(c.component.Probes, podsReadyTime) {
				hasProbeTimeout = true
				if messages == nil {
					messages = appsv1alpha1.ComponentMessageMap{}
				}
				messages.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
			}
		}
	}
	return hasProbeTimeout, messages, nil
}

func (c *rsmComponent) isAllConfigSynced(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	var (
		cmKey client.ObjectKey
		cmObj = &corev1.ConfigMap{}
	)

	if len(c.component.ConfigTemplates) == 0 {
		return true, nil
	}

	configurationKey := client.ObjectKey{
		Namespace: c.GetNamespace(),
		Name:      cfgcore.GenerateComponentConfigurationName(c.GetClusterName(), c.GetName()),
	}
	configuration := &appsv1alpha1.Configuration{}
	if err := cli.Get(reqCtx.Ctx, configurationKey, configuration); err != nil {
		return false, err
	}
	for _, configSpec := range c.component.ConfigTemplates {
		item := configuration.Spec.GetConfigurationItem(configSpec.Name)
		status := configuration.Status.GetItemStatus(configSpec.Name)
		// for creating phase
		if item == nil || status == nil {
			return false, nil
		}
		cmKey = client.ObjectKey{
			Namespace: c.GetNamespace(),
			Name:      cfgcore.GetComponentCfgName(c.GetClusterName(), c.GetName(), configSpec.Name),
		}
		if err := cli.Get(reqCtx.Ctx, cmKey, cmObj); err != nil {
			return false, err
		}
		if intctrlutil.GetConfigSpecReconcilePhase(cmObj, *item, status) != appsv1alpha1.CFinishedPhase {
			return false, nil
		}
	}
	return true, nil
}

func (c *rsmComponent) updateMembersStatus() {
	// get component status
	componentStatus := c.getComponentStatus()

	// for compatibilities prior KB 0.7.0
	buildConsensusSetStatus := func(membersStatus []workloads.MemberStatus) *appsv1alpha1.ConsensusSetStatus {
		consensusSetStatus := &appsv1alpha1.ConsensusSetStatus{
			Leader: appsv1alpha1.ConsensusMemberStatus{
				Name:       "",
				Pod:        constant.ComponentStatusDefaultPodName,
				AccessMode: appsv1alpha1.None,
			},
		}
		for _, memberStatus := range membersStatus {
			status := appsv1alpha1.ConsensusMemberStatus{
				Name:       memberStatus.Name,
				Pod:        memberStatus.PodName,
				AccessMode: appsv1alpha1.AccessMode(memberStatus.AccessMode),
			}
			switch {
			case memberStatus.IsLeader:
				consensusSetStatus.Leader = status
			case memberStatus.CanVote:
				consensusSetStatus.Followers = append(consensusSetStatus.Followers, status)
			default:
				consensusSetStatus.Learner = &status
			}
		}
		return consensusSetStatus
	}
	buildReplicationSetStatus := func(membersStatus []workloads.MemberStatus) *appsv1alpha1.ReplicationSetStatus {
		replicationSetStatus := &appsv1alpha1.ReplicationSetStatus{
			Primary: appsv1alpha1.ReplicationMemberStatus{
				Pod: "Unknown",
			},
		}
		for _, memberStatus := range membersStatus {
			status := appsv1alpha1.ReplicationMemberStatus{
				Pod: memberStatus.PodName,
			}
			switch {
			case memberStatus.IsLeader:
				replicationSetStatus.Primary = status
			default:
				replicationSetStatus.Secondaries = append(replicationSetStatus.Secondaries, status)
			}
		}
		return replicationSetStatus
	}

	// update members status
	switch c.component.WorkloadType {
	case appsv1alpha1.Consensus:
		componentStatus.ConsensusSetStatus = buildConsensusSetStatus(c.runningWorkload.Status.MembersStatus)
	case appsv1alpha1.Replication:
		componentStatus.ReplicationSetStatus = buildReplicationSetStatus(c.runningWorkload.Status.MembersStatus)
	}
	componentStatus.MembersStatus = slices.Clone(c.runningWorkload.Status.MembersStatus)

	// set component status back
	c.Cluster.Status.Components[c.GetName()] = componentStatus
}

func (c *rsmComponent) getComponentStatus() appsv1alpha1.ClusterComponentStatus {
	if c.Cluster.Status.Components == nil {
		c.Cluster.Status.Components = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	if _, ok := c.Cluster.Status.Components[c.GetName()]; !ok {
		c.Cluster.Status.Components[c.GetName()] = appsv1alpha1.ClusterComponentStatus{}
	}
	return c.Cluster.Status.Components[c.GetName()]
}

func (c *rsmComponent) isScaleOutFailed(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if c.runningWorkload.Spec.Replicas == nil {
		return false, nil
	}
	if c.component.Replicas <= *c.runningWorkload.Spec.Replicas {
		return false, nil
	}
	if c.workload == nil {
		return false, nil
	}
	stsObj := ConvertRSMToSTS(c.runningWorkload)
	rsmProto := c.workload.(*workloads.ReplicatedStateMachine)
	stsProto := ConvertRSMToSTS(rsmProto)
	backupKey := types.NamespacedName{
		Namespace: stsObj.Namespace,
		Name:      stsObj.Name + "-scaling",
	}
	d, err := newDataClone(reqCtx, cli, c.Cluster, c.component, stsObj, stsProto, backupKey)
	if err != nil {
		return false, err
	}
	if status, err := d.checkBackupStatus(); err != nil {
		return false, err
	} else if status == backupStatusFailed {
		return true, nil
	}
	for i := *c.runningWorkload.Spec.Replicas; i < c.component.Replicas; i++ {
		if status, err := d.checkRestoreStatus(i); err != nil {
			return false, err
		} else if status == backupStatusFailed {
			return true, nil
		}
	}
	return false, nil
}

func (c *rsmComponent) restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return restartPod(&c.runningWorkload.Spec.Template)
}

func (c *rsmComponent) expandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	for _, vct := range c.runningWorkload.Spec.VolumeClaimTemplates {
		var proto *corev1.PersistentVolumeClaimTemplate
		for i, v := range c.component.VolumeClaimTemplates {
			if v.Name == vct.Name {
				proto = &c.component.VolumeClaimTemplates[i]
				break
			}
		}
		// REVIEW: seems we can remove a volume claim from templates at runtime, without any changes and warning messages?
		if proto == nil {
			continue
		}

		if err := c.expandVolumes(reqCtx, cli, vct.Name, proto); err != nil {
			return err
		}
	}
	return nil
}

func (c *rsmComponent) expandVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client,
	vctName string, proto *corev1.PersistentVolumeClaimTemplate) error {
	for i := *c.runningWorkload.Spec.Replicas - 1; i >= 0; i-- {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := types.NamespacedName{
			Namespace: c.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s-%d", vctName, c.runningWorkload.Name, i),
		}
		pvcNotFound := false
		if err := cli.Get(reqCtx.Ctx, pvcKey, pvc); err != nil {
			if apierrors.IsNotFound(err) {
				pvcNotFound = true
			} else {
				return err
			}
		}

		if !pvcNotFound {
			quantity := pvc.Spec.Resources.Requests.Storage()
			newQuantity := proto.Spec.Resources.Requests.Storage()
			if quantity.Cmp(*pvc.Status.Capacity.Storage()) == 0 && newQuantity.Cmp(*quantity) < 0 {
				errMsg := fmt.Sprintf("shrinking the volume is not supported, volume: %s, quantity: %s, new quantity: %s",
					pvc.GetName(), quantity.String(), newQuantity.String())
				reqCtx.Event(c.Cluster, corev1.EventTypeWarning, "VolumeExpansionFailed", errMsg)
				return fmt.Errorf("%s", errMsg)
			}
		}

		if err := c.updatePVCSize(reqCtx, cli, pvcKey, pvc, pvcNotFound, proto); err != nil {
			return err
		}
	}
	return nil
}

func (c *rsmComponent) updatePVCSize(reqCtx intctrlutil.RequestCtx, cli client.Client, pvcKey types.NamespacedName,
	pvc *corev1.PersistentVolumeClaim, pvcNotFound bool, vctProto *corev1.PersistentVolumeClaimTemplate) error {
	// reference: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#recovering-from-failure-when-expanding-volumes
	// 1. Mark the PersistentVolume(PV) that is bound to the PersistentVolumeClaim(PVC) with Retain reclaim policy.
	// 2. Delete the PVC. Since PV has Retain reclaim policy - we will not lose any data when we recreate the PVC.
	// 3. Delete the claimRef entry from PV specs, so as new PVC can bind to it. This should make the PV Available.
	// 4. Re-create the PVC with smaller size than PV and set volumeName field of the PVC to the name of the PV. This should bind new PVC to existing PV.
	// 5. Don't forget to restore the reclaim policy of the PV.
	newPVC := pvc.DeepCopy()
	if pvcNotFound {
		newPVC.Name = pvcKey.Name
		newPVC.Namespace = pvcKey.Namespace
		newPVC.SetLabels(vctProto.Labels)
		newPVC.Spec = vctProto.Spec
		ml := client.MatchingLabels{
			constant.PVCNameLabelKey: pvcKey.Name,
		}
		pvList := corev1.PersistentVolumeList{}
		if err := cli.List(reqCtx.Ctx, &pvList, ml); err != nil {
			return err
		}
		for _, pv := range pvList.Items {
			// find pv referenced this pvc
			if pv.Spec.ClaimRef == nil {
				continue
			}
			if pv.Spec.ClaimRef.Name == pvcKey.Name {
				newPVC.Spec.VolumeName = pv.Name
				break
			}
		}
	} else {
		newPVC.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
		// delete annotation to make it re-bind
		delete(newPVC.Annotations, "pv.kubernetes.io/bind-completed")
	}

	pvNotFound := false

	// step 1: update pv to retain
	pv := &corev1.PersistentVolume{}
	pvKey := types.NamespacedName{
		Namespace: pvcKey.Namespace,
		Name:      newPVC.Spec.VolumeName,
	}
	if err := cli.Get(reqCtx.Ctx, pvKey, pv); err != nil {
		if apierrors.IsNotFound(err) {
			pvNotFound = true
		} else {
			return err
		}
	}

	graphCli := model.NewGraphClient(c.Client)

	type pvcRecreateStep int
	const (
		pvPolicyRetainStep pvcRecreateStep = iota
		deletePVCStep
		removePVClaimRefStep
		createPVCStep
		pvRestorePolicyStep
	)

	addStepMap := map[pvcRecreateStep]func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex{
		pvPolicyRetainStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 1: update pv to retain
			retainPV := pv.DeepCopy()
			if retainPV.Labels == nil {
				retainPV.Labels = make(map[string]string)
			}
			// add label to pv, in case pvc get deleted, and we can't find pv
			retainPV.Labels[constant.PVCNameLabelKey] = pvcKey.Name
			if retainPV.Annotations == nil {
				retainPV.Annotations = make(map[string]string)
			}
			retainPV.Annotations[constant.PVLastClaimPolicyAnnotationKey] = string(pv.Spec.PersistentVolumeReclaimPolicy)
			retainPV.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			return graphCli.Do(c.dag, pv, retainPV, model.ActionPatchPtr(), fromVertex)
		},
		deletePVCStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 2: delete pvc, this will not delete pv because policy is 'retain'
			removeFinalizerPVC := pvc.DeepCopy()
			removeFinalizerPVC.SetFinalizers([]string{})
			removeFinalizerPVCVertex := graphCli.Do(c.dag, pvc, removeFinalizerPVC, model.ActionPatchPtr(), fromVertex)
			return graphCli.Do(c.dag, nil, removeFinalizerPVC, model.ActionDeletePtr(), removeFinalizerPVCVertex)
		},
		removePVClaimRefStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 3: remove claimRef in pv
			removeClaimRefPV := pv.DeepCopy()
			if removeClaimRefPV.Spec.ClaimRef != nil {
				removeClaimRefPV.Spec.ClaimRef.UID = ""
				removeClaimRefPV.Spec.ClaimRef.ResourceVersion = ""
			}
			return graphCli.Do(c.dag, pv, removeClaimRefPV, model.ActionPatchPtr(), fromVertex)
		},
		createPVCStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 4: create new pvc
			newPVC.SetResourceVersion("")
			return graphCli.Do(c.dag, nil, newPVC, model.ActionCreatePtr(), fromVertex)
		},
		pvRestorePolicyStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 5: restore to previous pv policy
			restorePV := pv.DeepCopy()
			policy := corev1.PersistentVolumeReclaimPolicy(restorePV.Annotations[constant.PVLastClaimPolicyAnnotationKey])
			if len(policy) == 0 {
				policy = corev1.PersistentVolumeReclaimDelete
			}
			restorePV.Spec.PersistentVolumeReclaimPolicy = policy
			return graphCli.Do(c.dag, pv, restorePV, model.ActionPatchPtr(), fromVertex)
		},
	}

	updatePVCByRecreateFromStep := func(fromStep pvcRecreateStep) {
		lastVertex := c.workloadVertex()
		for step := pvRestorePolicyStep; step >= fromStep && step >= pvPolicyRetainStep; step-- {
			lastVertex = addStepMap[step](lastVertex, step)
		}
	}

	targetQuantity := vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
	if pvcNotFound && !pvNotFound {
		// this could happen if create pvc step failed when recreating pvc
		updatePVCByRecreateFromStep(removePVClaimRefStep)
		return nil
	}
	if pvcNotFound && pvNotFound {
		// if both pvc and pv not found, do nothing
		return nil
	}
	if reflect.DeepEqual(pvc.Spec.Resources, newPVC.Spec.Resources) && pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain {
		// this could happen if create pvc succeeded but last step failed
		updatePVCByRecreateFromStep(pvRestorePolicyStep)
		return nil
	}
	if pvcQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; !viper.GetBool(constant.CfgRecoverVolumeExpansionFailure) &&
		pvcQuantity.Cmp(targetQuantity) == 1 && // check if it's compressing volume
		targetQuantity.Cmp(*pvc.Status.Capacity.Storage()) >= 0 { // check if target size is greater than or equal to actual size
		// this branch means we can update pvc size by recreate it
		updatePVCByRecreateFromStep(pvPolicyRetainStep)
		return nil
	}
	if pvcQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; pvcQuantity.Cmp(vctProto.Spec.Resources.Requests[corev1.ResourceStorage]) != 0 {
		// use pvc's update without anything extra
		graphCli.Update(c.dag, nil, newPVC)
		return nil
	}
	// all the else means no need to update

	return nil
}

func (c *rsmComponent) hasVolumeExpansionRunning(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, bool, error) {
	var (
		running bool
		failed  bool
	)
	for _, vct := range c.runningWorkload.Spec.VolumeClaimTemplates {
		volumes, err := c.getRunningVolumes(reqCtx, cli, vct.Name, c.runningWorkload)
		if err != nil {
			return false, false, err
		}
		for _, v := range volumes {
			if v.Status.Capacity == nil || v.Status.Capacity.Storage().Cmp(v.Spec.Resources.Requests[corev1.ResourceStorage]) >= 0 {
				continue
			}
			running = true
			// TODO: how to check the expansion failed?
		}
	}
	return running, failed, nil
}

func (c *rsmComponent) horizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	sts := ConvertRSMToSTS(c.runningWorkload)
	if sts.Status.ReadyReplicas == c.component.Replicas {
		return nil
	}
	ret := c.horizontalScaling(sts)
	if ret == 0 {
		if err := c.postScaleIn(reqCtx, cli); err != nil {
			return err
		}
		if err := c.postScaleOut(reqCtx, cli, sts); err != nil {
			return err
		}
		return nil
	}
	if ret < 0 {
		if err := c.scaleIn(reqCtx, cli, sts); err != nil {
			return err
		}
	} else {
		if err := c.scaleOut(reqCtx, cli, sts); err != nil {
			return err
		}
	}

	if err := c.updatePodReplicaLabel4Scaling(reqCtx, cli, c.component.Replicas); err != nil {
		return err
	}

	// update KB_<component-type>_<pod-idx>_<hostname> env needed by pod to obtain hostname.
	c.updatePodEnvConfig()

	reqCtx.Recorder.Eventf(c.Cluster,
		corev1.EventTypeNormal,
		"HorizontalScale",
		"start horizontal scale component %s of cluster %s from %d to %d",
		c.GetName(), c.GetClusterName(), int(c.component.Replicas)-ret, c.component.Replicas)

	return nil
}

// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (c *rsmComponent) horizontalScaling(stsObj *appsv1.StatefulSet) int {
	return int(c.component.Replicas - *stsObj.Spec.Replicas)
}

func (c *rsmComponent) updatePodEnvConfig() {
	graphCli := model.NewGraphClient(c.Client)
	for _, cm := range graphCli.FindAll(c.dag, &corev1.ConfigMap{}) {
		// TODO: need a way to reference the env config.
		envConfigName := fmt.Sprintf("%s-%s-env", c.GetClusterName(), c.GetName())
		if cm.GetName() == envConfigName {
			graphCli.Update(c.dag, cm, cm)
		}
	}
}

func (c *rsmComponent) updatePodReplicaLabel4Scaling(reqCtx intctrlutil.RequestCtx, cli client.Client, replicas int32) error {
	graphCli := model.NewGraphClient(c.Client)
	pods, err := listPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.getMatchingLabels())
	if err != nil {
		return err
	}
	for _, pod := range pods {
		obj := pod.DeepCopy()
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		obj.Annotations[constant.ComponentReplicasAnnotationKey] = strconv.Itoa(int(replicas))
		graphCli.Update(c.dag, nil, obj)
	}
	return nil
}

func (c *rsmComponent) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	// if scale in to 0, do not delete pvcs
	if c.component.Replicas == 0 {
		reqCtx.Log.Info("scale in to 0, keep all PVCs")
		return nil
	}
	// TODO: check the component definition to determine whether we need to call leave member before deleting replicas.
	err := c.leaveMember4ScaleIn(reqCtx, cli, stsObj)
	if err != nil {
		reqCtx.Log.Info(fmt.Sprintf("leave member at scaling-in error, retry later: %s", err.Error()))
		return err
	}
	return c.deletePVCs4ScaleIn(reqCtx, cli, stsObj)
}

func (c *rsmComponent) postScaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *rsmComponent) leaveMember4ScaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	pods, err := listPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.getMatchingLabels())
	if err != nil {
		return err
	}
	for _, pod := range pods {
		subs := strings.Split(pod.Name, "-")
		if ordinal, err := strconv.ParseInt(subs[len(subs)-1], 10, 32); err != nil {
			return err
		} else if int32(ordinal) < c.component.Replicas {
			continue
		}
		lorryCli, err1 := lorry.NewClient(c.component.CharacterType, *pod)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}

		if intctrlutil.IsNil(lorryCli) {
			// no lorry in the pod
			continue
		}

		if err2 := lorryCli.LeaveMember(reqCtx.Ctx); err2 != nil {
			if err == nil {
				err = err2
			}
		}
	}
	return err // TODO: use requeue-after
}

func (c *rsmComponent) deletePVCs4ScaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	graphCli := model.NewGraphClient(c.Client)
	for i := c.component.Replicas; i < *stsObj.Spec.Replicas; i++ {
		for _, vct := range stsObj.Spec.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
			}
			pvc := corev1.PersistentVolumeClaim{}
			if err := cli.Get(reqCtx.Ctx, pvcKey, &pvc); err != nil {
				return err
			}
			// Since there are no order guarantee between updating STS and deleting PVCs, if there is any error occurred
			// after updating STS and before deleting PVCs, the PVCs intended to scale-in will be leaked.
			// For simplicity, the updating dependency is added between them to guarantee that the PVCs to scale-in
			// will be deleted or the scaling-in operation will be failed.
			graphCli.Delete(c.dag, &pvc)
		}
	}
	return nil
}

func (c *rsmComponent) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	var (
		backupKey = types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
	)

	// sts's replicas=0 means it's starting not scaling, skip all the scaling work.
	if *stsObj.Spec.Replicas == 0 {
		return nil
	}

	graphCli := model.NewGraphClient(c.Client)
	graphCli.Noop(c.dag, c.workload)
	rsmProto := c.workload.(*workloads.ReplicatedStateMachine)
	stsProto := ConvertRSMToSTS(rsmProto)
	d, err := newDataClone(reqCtx, cli, c.Cluster, c.component, stsObj, stsProto, backupKey)
	if err != nil {
		return err
	}
	var succeed bool
	if d == nil {
		succeed = true
	} else {
		succeed, err = d.succeed()
		if err != nil {
			return err
		}
	}
	if succeed {
		// pvcs are ready, rsm.replicas should be updated
		graphCli.Update(c.dag, nil, c.workload)
		return c.postScaleOut(reqCtx, cli, stsObj)
	} else {
		graphCli.Noop(c.dag, c.workload)
		// update objs will trigger cluster reconcile, no need to requeue error
		objs, err := d.cloneData(d)
		if err != nil {
			return err
		}
		graphCli := model.NewGraphClient(c.Client)
		for _, obj := range objs {
			graphCli.Do(c.dag, nil, obj, model.ActionCreatePtr(), nil)
		}
		return nil
	}
}

func (c *rsmComponent) postScaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	var (
		snapshotKey = types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
	)

	d, err := newDataClone(reqCtx, cli, c.Cluster, c.component, stsObj, stsObj, snapshotKey)
	if err != nil {
		return err
	}
	if d != nil {
		// clean backup resources.
		// there will not be any backup resources other than scale out.
		tmpObjs, err := d.clearTmpResources()
		if err != nil {
			return err
		}
		graphCli := model.NewGraphClient(c.Client)
		for _, obj := range tmpObjs {
			graphCli.Do(c.dag, nil, obj, model.ActionDeletePtr(), nil)
		}
	}

	return nil
}

func (c *rsmComponent) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client, rsmObj *workloads.ReplicatedStateMachine) error {
	if rsmObj == nil {
		c.createWorkload()
	} else {
		c.updateWorkload(rsmObj)
		// to work around that the scaled PVC will be deleted at object action.
		if err := c.updateVolumes(reqCtx, cli, rsmObj); err != nil {
			return err
		}
	}
	if err := c.updatePDB(reqCtx, cli); err != nil {
		return err
	}
	return nil
}

func (c *rsmComponent) createWorkload() {
	rsmProto := c.workload.(*workloads.ReplicatedStateMachine)
	buildWorkLoadAnnotations(rsmProto, c.Cluster)
	model.NewGraphClient(c.Client).Create(c.dag, c.workload, model.ReplaceIfExistingOption)
}

func (c *rsmComponent) updateWorkload(rsmObj *workloads.ReplicatedStateMachine) bool {
	rsmObjCopy := rsmObj.DeepCopy()
	rsmProto := c.workload.(*workloads.ReplicatedStateMachine)

	// remove original monitor annotations
	if len(rsmObjCopy.Annotations) > 0 {
		maps.DeleteFunc(rsmObjCopy.Annotations, func(k, v string) bool {
			return strings.HasPrefix(k, "monitor.kubeblocks.io")
		})
	}
	mergeAnnotations(rsmObjCopy.Annotations, &rsmProto.Annotations)
	rsmObjCopy.Annotations = rsmProto.Annotations
	buildWorkLoadAnnotations(rsmObjCopy, c.Cluster)

	// keep the original template annotations.
	// if annotations exist and are replaced, the rsm will be updated.
	mergeAnnotations(rsmObjCopy.Spec.Template.Annotations, &rsmProto.Spec.Template.Annotations)
	rsmObjCopy.Spec.Template = rsmProto.Spec.Template
	rsmObjCopy.Spec.Replicas = rsmProto.Spec.Replicas
	c.updateUpdateStrategy(rsmObjCopy, rsmProto)
	rsmObjCopy.Spec.Service = rsmProto.Spec.Service
	rsmObjCopy.Spec.AlternativeServices = rsmProto.Spec.AlternativeServices
	rsmObjCopy.Spec.Roles = rsmProto.Spec.Roles
	rsmObjCopy.Spec.RoleProbe = rsmProto.Spec.RoleProbe
	rsmObjCopy.Spec.MembershipReconfiguration = rsmProto.Spec.MembershipReconfiguration
	rsmObjCopy.Spec.MemberUpdateStrategy = rsmProto.Spec.MemberUpdateStrategy
	rsmObjCopy.Spec.Paused = rsmProto.Spec.Paused
	rsmObjCopy.Spec.Credential = rsmProto.Spec.Credential

	resolvePodSpecDefaultFields(rsmObj.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)

	delayUpdatePodSpecSystemFields(rsmObj.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)
	isTemplateUpdated := !reflect.DeepEqual(&rsmObj.Spec, &rsmObjCopy.Spec)
	if isTemplateUpdated {
		updatePodSpecSystemFields(&rsmObjCopy.Spec.Template.Spec)
	}
	if isTemplateUpdated || !reflect.DeepEqual(rsmObj.Annotations, rsmObjCopy.Annotations) {
		c.workload = rsmObjCopy
		graphCli := model.NewGraphClient(c.Client)
		if graphCli.IsAction(c.dag, c.workload, model.ActionNoopPtr()) {
			return false
		}
		graphCli.Update(c.dag, nil, c.workload, model.ReplaceIfExistingOption)
		return true
	}
	return false
}

func (c *rsmComponent) updatePDB(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	pdbObjList, err := listObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PodDisruptionBudgetSignature, c.GetNamespace(), c.getMatchingLabels())
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	graphCli := model.NewGraphClient(c.Client)
	for _, obj := range graphCli.FindAll(c.dag, &policyv1.PodDisruptionBudget{}) {
		pdbProto, _ := obj.(*policyv1.PodDisruptionBudget)
		if pos := slices.IndexFunc(pdbObjList, func(pdbObj *policyv1.PodDisruptionBudget) bool {
			return pdbObj.GetName() == pdbProto.GetName()
		}); pos < 0 {
			graphCli.Create(c.dag, pdbProto)
		} else {
			pdbObj := pdbObjList[pos]
			if !reflect.DeepEqual(pdbObj.Spec, pdbProto.Spec) {
				pdbObj.Spec = pdbProto.Spec
				graphCli.Update(c.dag, nil, pdbObj, model.ReplaceIfExistingOption)
			}
		}
	}
	return nil
}

func (c *rsmComponent) updateUpdateStrategy(rsmObj, rsmProto *workloads.ReplicatedStateMachine) {
	var objMaxUnavailable *intstr.IntOrString
	if rsmObj.Spec.UpdateStrategy.RollingUpdate != nil {
		objMaxUnavailable = rsmObj.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable
	}
	rsmObj.Spec.UpdateStrategy = rsmProto.Spec.UpdateStrategy
	if objMaxUnavailable == nil && rsmObj.Spec.UpdateStrategy.RollingUpdate != nil {
		// HACK: This field is alpha-level (since v1.24) and is only honored by servers that enable the
		// MaxUnavailableStatefulSet feature.
		// When we get a nil MaxUnavailable from k8s, we consider that the field is not supported by the server,
		// and set the MaxUnavailable as nil explicitly to avoid the workload been updated unexpectedly.
		// Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
		rsmObj.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = nil
	}
}

func (c *rsmComponent) updateVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, rsmObj *workloads.ReplicatedStateMachine) error {
	graphCli := model.NewGraphClient(c.Client)
	// PVCs which have been added to the dag because of volume expansion.
	pvcNameSet := sets.New[string]()
	for _, obj := range graphCli.FindAll(c.dag, &corev1.PersistentVolumeClaim{}) {
		pvcNameSet.Insert(obj.GetName())
	}

	for _, vct := range c.component.VolumeClaimTemplates {
		pvcs, err := c.getRunningVolumes(reqCtx, cli, vct.Name, rsmObj)
		if err != nil {
			return err
		}
		for _, pvc := range pvcs {
			if pvcNameSet.Has(pvc.Name) {
				continue
			}
			graphCli.Noop(c.dag, pvc)
		}
	}
	return nil
}

func (c *rsmComponent) getRunningVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, vctName string,
	rsmObj *workloads.ReplicatedStateMachine) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs, err := listObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PersistentVolumeClaimSignature, c.GetNamespace(), c.getMatchingLabels())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	matchedPVCs := make([]*corev1.PersistentVolumeClaim, 0)
	prefix := fmt.Sprintf("%s-%s", vctName, rsmObj.Name)
	for _, pvc := range pvcs {
		if strings.HasPrefix(pvc.Name, prefix) {
			matchedPVCs = append(matchedPVCs, pvc)
		}
	}
	return matchedPVCs, nil
}

// hasFailedAndTimedOutPod returns whether the pods of components are still failed after a PodFailedTimeout period.
func hasFailedAndTimedOutPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, time.Duration) {
	var (
		hasTimedOutPod bool
		messages       = appsv1alpha1.ComponentMessageMap{}
		hasFailedPod   bool
		requeueAfter   time.Duration
	)
	for _, pod := range pods {
		isFailed, isTimedOut, messageStr := isPodFailedAndTimedOut(pod)
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
		requeueAfter = podContainerFailedTimeout
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
		return true, time.Now().After(cond.LastTransitionTime.Add(podScheduledFailedTimeout)), cond.Message
	}
	return false, false, ""
}

// isPodFailedAndTimedOut checks if the pod is failed and timed out.
func isPodFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
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
	return time.Now().After(containerReadyCondition.LastTransitionTime.Add(podContainerFailedTimeout))
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
		&dpv1alpha1.BackupPolicyList{},
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
			if isOwnerOf(cluster, object, cli.Scheme()) {
				name, err := getGVKName(object, cli.Scheme())
				if err != nil {
					return nil, err
				}
				snapshot[*name] = object
			}
		}
	}
	return snapshot, nil
}

func resolveObjectAction(snapshot clusterSnapshot, obj client.Object, scheme *runtime.Scheme) (*model.Action, error) {
	gvk, err := getGVKName(obj, scheme)
	if err != nil {
		return nil, err
	}
	if _, ok := snapshot[*gvk]; ok {
		return model.ActionNoopPtr(), nil
	} else {
		return model.ActionCreatePtr(), nil
	}
}
