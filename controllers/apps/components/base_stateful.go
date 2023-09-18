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
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	lorry "github.com/apecloud/kubeblocks/lorry/client"
)

// rsmComponentBase as a base class for single rsm based component (stateful & replication & consensus).
type rsmComponentBase struct {
	componentBase
	// runningWorkload can be nil, and the replicas of workload can be nil (zero)
	runningWorkload *workloads.ReplicatedStateMachine
}

func (c *rsmComponentBase) init(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder, load bool) error {
	var err error
	if builder != nil {
		if err = builder.BuildEnv().
			BuildWorkload().
			BuildPDB().
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

func (c *rsmComponentBase) loadRunningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*workloads.ReplicatedStateMachine, error) {
	rsmList, err := listRSMOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
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

func (c *rsmComponentBase) GetBuiltObjects(builder componentWorkloadBuilder) ([]client.Object, error) {
	dagSnapshot := c.Dag
	defer func() {
		c.Dag = dagSnapshot
	}()

	c.Dag = graph.NewDAG()
	if err := c.init(intctrlutil.RequestCtx{}, nil, builder, false); err != nil {
		return nil, err
	}

	objs := make([]client.Object, 0)
	for _, v := range c.Dag.Vertices() {
		if vv, ok := v.(*ictrltypes.LifecycleVertex); ok {
			objs = append(objs, vv.Obj)
		}
	}
	return objs, nil
}

func (c *rsmComponentBase) Create(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, false); err != nil {
		return err
	}

	if err := c.ValidateObjectsAction(); err != nil {
		return err
	}

	return nil
}

func (c *rsmComponentBase) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(impl): delete component owned resources
	return nil
}

func (c *rsmComponentBase) Update(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, true); err != nil {
		return err
	}

	if c.runningWorkload != nil {
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
	}

	if err := c.updateUnderlyingResources(reqCtx, cli, c.runningWorkload); err != nil {
		return err
	}

	return c.ResolveObjectsAction(reqCtx, cli)
}

func (c *rsmComponentBase) Status(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, true); err != nil {
		return err
	}
	if c.runningWorkload == nil {
		return nil
	}

	isDeleting := func() bool {
		return !c.runningWorkload.DeletionTimestamp.IsZero()
	}()
	isZeroReplica := func() bool {
		return (c.runningWorkload.Spec.Replicas == nil || *c.runningWorkload.Spec.Replicas == 0) && c.Component.Replicas == 0
	}()
	pods, err := listPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return err
	}
	hasComponentPod := func() bool {
		return len(pods) > 0
	}()
	isRunning, err := c.ComponentSet.IsRunning(reqCtx.Ctx, c.runningWorkload)
	if err != nil {
		return err
	}
	isAllConfigSynced := c.isAllConfigSynced(reqCtx, cli)
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
				time := metav1.Now()
				status.PodsReadyTime = &time
			}
			return nil
		})
	}

	podsReady := false
	switch {
	case isDeleting:
		c.SetStatusPhase(appsv1alpha1.DeletingClusterCompPhase, nil, "Component is Deleting")
	case isZeroReplica && hasComponentPod:
		c.SetStatusPhase(appsv1alpha1.StoppingClusterCompPhase, nil, "Component is Stopping")
		podsReady = true
	case isZeroReplica:
		c.SetStatusPhase(appsv1alpha1.StoppedClusterCompPhase, nil, "Component is Stopped")
		podsReady = true
	case isRunning && isAllConfigSynced && !hasRunningVolumeExpansion:
		c.SetStatusPhase(appsv1alpha1.RunningClusterCompPhase, nil, "Component is Running")
		podsReady = true
	case !hasFailure && isInCreatingPhase:
		c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase, nil, "Create a new component")
	case !hasFailure:
		c.SetStatusPhase(appsv1alpha1.UpdatingClusterCompPhase, nil, "Component is Updating")
	case !isComponentAvailable:
		c.SetStatusPhase(appsv1alpha1.FailedClusterCompPhase, messages, "Component is Failed")
	default:
		c.SetStatusPhase(appsv1alpha1.AbnormalClusterCompPhase, nil, "unknown")
	}
	updatePodsReady(podsReady)

	// works should continue to be done after spec updated.
	if err := c.horizontalScale(reqCtx, cli); err != nil {
		return err
	}

	if vertexes, err := c.ComponentSet.HandleRoleChange(reqCtx.Ctx, c.runningWorkload); err != nil {
		return err
	} else {
		for _, v := range vertexes {
			c.Dag.AddVertex(v)
		}
	}

	c.updateWorkload(c.runningWorkload)

	// update component info to pods' annotations
	if err := updateComponentInfoToPods(reqCtx.Ctx, cli, c.Cluster, c.Component, c.Dag); err != nil {
		return err
	}

	// patch the current componentSpec workload's custom labels
	if err := updateCustomLabelToPods(reqCtx.Ctx, cli, c.Cluster, c.Component, c.Dag); err != nil {
		reqCtx.Event(c.Cluster, corev1.EventTypeWarning, "Component Workload Controller PatchWorkloadCustomLabelFailed", err.Error())
		return err
	}

	return nil
}

// isAvailable tells whether the component is basically available, ether working well or in a fragile state:
// 1. at least one pod is available
// 2. with latest revision
// 3. and with leader role label set
func (c *rsmComponentBase) isAvailable(reqCtx intctrlutil.RequestCtx, cli client.Client, pods []*corev1.Pod) (bool, error) {
	if isLatestRevision, err := IsComponentPodsWithLatestRevision(reqCtx.Ctx, cli, c.Cluster, c.runningWorkload); err != nil {
		return false, err
	} else if !isLatestRevision {
		return false, nil
	}

	shouldCheckLeader := func() bool {
		return c.Component.WorkloadType == appsv1alpha1.Consensus || c.Component.WorkloadType == appsv1alpha1.Replication
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
			continue
		}
		if _, ok := pod.Labels[constant.RoleLabelKey]; ok {
			continue
		}
		if hasLeaderRoleLabel(pod) {
			return true, nil
		}
	}
	return false, nil
}

func (c *rsmComponentBase) hasFailedPod(reqCtx intctrlutil.RequestCtx, cli client.Client, pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, error) {
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
	if c.Component.WorkloadType != appsv1alpha1.Consensus && c.Component.WorkloadType != appsv1alpha1.Replication {
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
			if isProbeTimeout(c.Component.Probes, podsReadyTime) {
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

func (c *rsmComponentBase) isScaleOutFailed(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if c.runningWorkload.Spec.Replicas == nil {
		return false, nil
	}
	if c.Component.Replicas <= *c.runningWorkload.Spec.Replicas {
		return false, nil
	}
	if c.WorkloadVertex == nil {
		return false, nil
	}
	stsObj := ConvertRSMToSTS(c.runningWorkload)
	rsmProto := c.WorkloadVertex.Obj.(*workloads.ReplicatedStateMachine)
	stsProto := ConvertRSMToSTS(rsmProto)
	backupKey := types.NamespacedName{
		Namespace: stsObj.Namespace,
		Name:      stsObj.Name + "-scaling",
	}
	d, err := newDataClone(reqCtx, cli, c.Cluster, c.Component, stsObj, stsProto, backupKey)
	if err != nil {
		return false, err
	}
	if status, err := d.checkBackupStatus(); err != nil {
		return false, err
	} else if status == backupStatusFailed {
		return true, nil
	}
	for _, name := range d.pvcKeysToRestore() {
		if status, err := d.checkRestoreStatus(name); err != nil {
			return false, err
		} else if status == backupStatusFailed {
			return true, nil
		}
	}
	return false, nil
}

func (c *rsmComponentBase) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return restartPod(&c.runningWorkload.Spec.Template)
}

func (c *rsmComponentBase) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	for _, vct := range c.runningWorkload.Spec.VolumeClaimTemplates {
		var proto *corev1.PersistentVolumeClaimTemplate
		for _, v := range c.Component.VolumeClaimTemplates {
			if v.Name == vct.Name {
				proto = &v
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

func (c *rsmComponentBase) expandVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client,
	vctName string, proto *corev1.PersistentVolumeClaimTemplate) error {
	pvcNotFound := false
	for i := *c.runningWorkload.Spec.Replicas - 1; i >= 0; i-- {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := types.NamespacedName{
			Namespace: c.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s-%d", vctName, c.runningWorkload.Name, i),
		}
		if err := cli.Get(reqCtx.Ctx, pvcKey, pvc); err != nil {
			if apierrors.IsNotFound(err) {
				pvcNotFound = true
			} else {
				return err
			}
		}
		if err := c.updatePVCSize(reqCtx, cli, pvcKey, pvc, pvcNotFound, proto); err != nil {
			return err
		}
	}
	return nil
}

func (c *rsmComponentBase) updatePVCSize(reqCtx intctrlutil.RequestCtx, cli client.Client, pvcKey types.NamespacedName,
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

	type pvcRecreateStep int
	const (
		pvPolicyRetainStep pvcRecreateStep = iota
		deletePVCStep
		removePVClaimRefStep
		createPVCStep
		pvRestorePolicyStep
	)

	addStepMap := map[pvcRecreateStep]func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex{
		pvPolicyRetainStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
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
			return c.PatchResource(retainPV, pv, fromVertex)
		},
		deletePVCStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 2: delete pvc, this will not delete pv because policy is 'retain'
			removeFinalizerPVC := pvc.DeepCopy()
			removeFinalizerPVC.SetFinalizers([]string{})
			removeFinalizerPVCVertex := c.PatchResource(removeFinalizerPVC, pvc, fromVertex)
			return c.DeleteResource(pvc, removeFinalizerPVCVertex)
		},
		removePVClaimRefStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 3: remove claimRef in pv
			removeClaimRefPV := pv.DeepCopy()
			if removeClaimRefPV.Spec.ClaimRef != nil {
				removeClaimRefPV.Spec.ClaimRef.UID = ""
				removeClaimRefPV.Spec.ClaimRef.ResourceVersion = ""
			}
			return c.PatchResource(removeClaimRefPV, pv, fromVertex)
		},
		createPVCStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 4: create new pvc
			newPVC.SetResourceVersion("")
			return c.CreateResource(newPVC, fromVertex)
		},
		pvRestorePolicyStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 5: restore to previous pv policy
			restorePV := pv.DeepCopy()
			policy := corev1.PersistentVolumeReclaimPolicy(restorePV.Annotations[constant.PVLastClaimPolicyAnnotationKey])
			if len(policy) == 0 {
				policy = corev1.PersistentVolumeReclaimDelete
			}
			restorePV.Spec.PersistentVolumeReclaimPolicy = policy
			return c.PatchResource(restorePV, pv, fromVertex)
		},
	}

	updatePVCByRecreateFromStep := func(fromStep pvcRecreateStep) {
		lastVertex := c.WorkloadVertex
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
		c.UpdateResource(newPVC, c.WorkloadVertex)
		return nil
	}
	// all the else means no need to update

	return nil
}

func (c *rsmComponentBase) isAllConfigSynced(reqCtx intctrlutil.RequestCtx, cli client.Client) bool {
	checkFinishedReconfigure := func(cm *corev1.ConfigMap) bool {
		labels := cm.GetLabels()
		annotations := cm.GetAnnotations()
		if len(annotations) == 0 || len(labels) == 0 {
			return false
		}
		hash, _ := util.ComputeHash(cm.Data)
		return labels[constant.CMInsConfigurationHashLabelKey] == hash
	}

	var (
		cmKey           client.ObjectKey
		cmObj           = &corev1.ConfigMap{}
		allConfigSynced = true
	)
	for _, configSpec := range c.Component.ConfigTemplates {
		cmKey = client.ObjectKey{
			Namespace: c.GetNamespace(),
			Name:      cfgcore.GetComponentCfgName(c.GetClusterName(), c.GetName(), configSpec.Name),
		}
		if err := cli.Get(reqCtx.Ctx, cmKey, cmObj); err != nil {
			return true
		}
		if !checkFinishedReconfigure(cmObj) {
			allConfigSynced = false
			break
		}
	}
	return allConfigSynced
}

func (c *rsmComponentBase) hasVolumeExpansionRunning(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, bool, error) {
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

func (c *rsmComponentBase) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.horizontalScale(reqCtx, cli)
}

func (c *rsmComponentBase) horizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	sts := ConvertRSMToSTS(c.runningWorkload)
	if sts.Status.ReadyReplicas == c.Component.Replicas {
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

	if err := c.updatePodReplicaLabel4Scaling(reqCtx, cli, c.Component.Replicas); err != nil {
		return err
	}

	// update KB_<component-type>_<pod-idx>_<hostname> env needed by pod to obtain hostname.
	c.updatePodEnvConfig()

	reqCtx.Recorder.Eventf(c.Cluster,
		corev1.EventTypeNormal,
		"HorizontalScale",
		"start horizontal scale component %s of cluster %s from %d to %d",
		c.GetName(), c.GetClusterName(), int(c.Component.Replicas)-ret, c.Component.Replicas)

	return nil
}

// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (c *rsmComponentBase) horizontalScaling(stsObj *appsv1.StatefulSet) int {
	return int(c.Component.Replicas - *stsObj.Spec.Replicas)
}

func (c *rsmComponentBase) updatePodEnvConfig() {
	for _, v := range ictrltypes.FindAll[*corev1.ConfigMap](c.Dag) {
		node := v.(*ictrltypes.LifecycleVertex)
		// TODO: need a way to reference the env config.
		envConfigName := fmt.Sprintf("%s-%s-env", c.GetClusterName(), c.GetName())
		if node.Obj.GetName() == envConfigName {
			node.Action = ictrltypes.ActionUpdatePtr()
		}
	}
}

func (c *rsmComponentBase) updatePodReplicaLabel4Scaling(reqCtx intctrlutil.RequestCtx, cli client.Client, replicas int32) error {
	pods, err := listPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return err
	}
	for _, pod := range pods {
		obj := pod.DeepCopy()
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		obj.Annotations[constant.ComponentReplicasAnnotationKey] = strconv.Itoa(int(replicas))
		c.UpdateResource(obj, c.WorkloadVertex)
	}
	return nil
}

func (c *rsmComponentBase) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	// if scale in to 0, do not delete pvcs
	if c.Component.Replicas == 0 {
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

func (c *rsmComponentBase) postScaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *rsmComponentBase) leaveMember4ScaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	pods, err := listPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return err
	}
	for _, pod := range pods {
		subs := strings.Split(pod.Name, "-")
		if ordinal, err := strconv.ParseInt(subs[len(subs)-1], 10, 32); err != nil {
			return err
		} else if int32(ordinal) < c.Component.Replicas {
			continue
		}
		lorryCli, err1 := lorry.NewClient(c.Component.CharacterType, *pod)
		if err1 != nil {
			if err == nil {
				err = err1
			}
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

func (c *rsmComponentBase) deletePVCs4ScaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	for i := c.Component.Replicas; i < *stsObj.Spec.Replicas; i++ {
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
			c.DeleteResource(&pvc, c.WorkloadVertex)
		}
	}
	return nil
}

func (c *rsmComponentBase) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
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

	c.WorkloadVertex.Immutable = true
	rsmProto := c.WorkloadVertex.Obj.(*workloads.ReplicatedStateMachine)
	stsProto := ConvertRSMToSTS(rsmProto)
	d, err := newDataClone(reqCtx, cli, c.Cluster, c.Component, stsObj, stsProto, backupKey)
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
		c.WorkloadVertex.Immutable = false
		return c.postScaleOut(reqCtx, cli, stsObj)
	} else {
		c.WorkloadVertex.Immutable = true
		// update objs will trigger cluster reconcile, no need to requeue error
		objs, err := d.cloneData(d)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			c.CreateResource(obj, nil)
		}
		return nil
	}
}

func (c *rsmComponentBase) postScaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	var (
		snapshotKey = types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
	)

	d, err := newDataClone(reqCtx, cli, c.Cluster, c.Component, stsObj, stsObj, snapshotKey)
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
		for _, obj := range tmpObjs {
			c.DeleteResource(obj, nil)
		}
	}

	return nil
}

func (c *rsmComponentBase) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client, rsmObj *workloads.ReplicatedStateMachine) error {
	if rsmObj == nil {
		c.createWorkload()
	} else {
		c.updateWorkload(rsmObj)
		// to work around that the scaled PVC will be deleted at object action.
		if err := c.updateVolumes(reqCtx, cli, rsmObj); err != nil {
			return err
		}
	}
	if err := c.UpdatePDB(reqCtx, cli); err != nil {
		return err
	}
	return nil
}

func (c *rsmComponentBase) createWorkload() {
	rsmProto := c.WorkloadVertex.Obj.(*workloads.ReplicatedStateMachine)
	buildWorkLoadAnnotations(rsmProto, c.Cluster)
	c.WorkloadVertex.Obj = rsmProto
	c.WorkloadVertex.Action = ictrltypes.ActionCreatePtr()
}

func (c *rsmComponentBase) updateWorkload(rsmObj *workloads.ReplicatedStateMachine) bool {
	rsmObjCopy := rsmObj.DeepCopy()
	rsmProto := c.WorkloadVertex.Obj.(*workloads.ReplicatedStateMachine)

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
	rsmObjCopy.Spec.Credential = rsmProto.Spec.Credential

	resolvePodSpecDefaultFields(rsmObj.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)

	delayUpdatePodSpecSystemFields(rsmObj.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)
	isTemplateUpdated := !reflect.DeepEqual(&rsmObj.Spec, &rsmObjCopy.Spec)
	if isTemplateUpdated {
		updatePodSpecSystemFields(&rsmObjCopy.Spec.Template.Spec)
	}
	if isTemplateUpdated || !reflect.DeepEqual(rsmObj.Annotations, rsmObjCopy.Annotations) {
		c.WorkloadVertex.Obj = rsmObjCopy
		c.WorkloadVertex.Action = ictrltypes.ActionPtr(ictrltypes.UPDATE)
		return true
	}
	return false
}

func (c *rsmComponentBase) updateUpdateStrategy(rsmObj, rsmProto *workloads.ReplicatedStateMachine) {
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

func (c *rsmComponentBase) updateVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, rsmObj *workloads.ReplicatedStateMachine) error {
	// PVCs which have been added to the dag because of volume expansion.
	pvcNameSet := sets.New[string]()
	for _, v := range ictrltypes.FindAll[*corev1.PersistentVolumeClaim](c.Dag) {
		pvcNameSet.Insert(v.(*ictrltypes.LifecycleVertex).Obj.GetName())
	}

	for _, vct := range c.Component.VolumeClaimTemplates {
		pvcs, err := c.getRunningVolumes(reqCtx, cli, vct.Name, rsmObj)
		if err != nil {
			return err
		}
		for _, pvc := range pvcs {
			if pvcNameSet.Has(pvc.Name) {
				continue
			}
			c.NoopResource(pvc, c.WorkloadVertex)
		}
	}
	return nil
}

func (c *rsmComponentBase) getRunningVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, vctName string,
	rsmObj *workloads.ReplicatedStateMachine) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs, err := listObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PersistentVolumeClaimSignature, c.GetNamespace(), c.GetMatchingLabels())
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
