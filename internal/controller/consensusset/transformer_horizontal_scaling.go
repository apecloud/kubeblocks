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

package consensusset

import (
	"fmt"
	"sort"
	"strings"

	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

// HorizontalScalingTransformer handles horizontal scaling.
// mental model: event driven FSM,
// i.e., reconciliation event -> calculate current state -> do corresponding action
type HorizontalScalingTransformer struct {}

func (t *HorizontalScalingTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	csSet := transCtx.CSSet

	if model.IsObjectDeleting(csSet) {
		return nil
	}

	if csSet.Spec.Replicas < csSet.Status.Replicas {
		return scaleIn(transCtx, dag, csSet)
	}
	return nil
}

func scaleIn(transCtx *CSSetTransformContext, dag *graph.DAG, csSet *workloads.ConsensusSet) error {
	switchoverJobList := &batchv1.JobList{}
	ml := client.MatchingLabels{
		model.AppInstanceLabelKey: csSet.Name,
		model.KBManagedByKey:      kindConsensusSet,
		jobTypeLabel:              jobTypeSwitchover,
		jobHandledLabel:           jobHandledFalse,
	}
	if err := transCtx.Client.List(transCtx.Context, switchoverJobList, ml); err != nil {
		return err
	}
	memberLeaveJobList := &batchv1.JobList{}
	ml[jobTypeLabel] = jobTypeMemberLeaveNotifying
	if err := transCtx.Client.List(transCtx.Context, memberLeaveJobList, ml); err != nil {
		return err
	}
	vertices := model.FindAll[*apps.StatefulSet](dag)
	if len(vertices) != 1 {
		return fmt.Errorf("unexpected sts found, expected 1, but found: %d", len(vertices))
	}
	stsVertex, _ := vertices[0].(*model.ObjectVertex)

	// set immutable=true at beginning,
	// if abnormal analysis passes and switchover done and member leave notifying done,
	// immutable will be reset to false
	stsVertex.Immutable = true

	memberLeaveControlAction := func(csSet *workloads.ConsensusSet, d *graph.DAG) (bool, error) {
		if shouldDoMemberLeaveNotifying(csSet) {
			if err := doMemberLeaveNotifying(csSet, dag); err != nil {
				return true, err
			}
			return true, nil
		}
		return false, nil
	}
	switch {
	case stateBeginning(switchoverJobList, memberLeaveJobList):
		return doBeginningAction(transCtx, dag, stsVertex)
	case stateSwitchover(switchoverJobList):
		return doAction(transCtx, dag, switchoverJobList, jobTypeSwitchover, stsVertex, memberLeaveControlAction)
	case stateMemberLeaveNotifying(memberLeaveJobList):
		return doAction(transCtx, dag, memberLeaveJobList, jobTypeMemberLeaveNotifying, stsVertex, nil)
	}
	return nil
}

func stateBeginning(switchoverJobList *batchv1.JobList, memberLeaveJobList *batchv1.JobList) bool {
	return len(switchoverJobList.Items) == 0 && len(memberLeaveJobList.Items) == 0
}

func stateSwitchover(jobList *batchv1.JobList) bool {
	return len(jobList.Items) == 1
}

func stateMemberLeaveNotifying(jobList *batchv1.JobList) bool {
	return len(jobList.Items) == 1
}

func doBeginningAction(transCtx *CSSetTransformContext, dag *graph.DAG, stsVertex *model.ObjectVertex) error {
	sts, _ := stsVertex.Obj.(*apps.StatefulSet)
	pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, sts)
	if err != nil {
		return err
	}

	roleMap := composeRoleMap(*transCtx.CSSet)

	if err := doAbnormalAnalysis(transCtx, pods, roleMap); err != nil {
		return err
	}
	if shouldDoSwitchover(transCtx.CSSet, pods, roleMap) {
		if err := doSwitchover(transCtx.CSSet, dag); err != nil {
			return err
		}
		return nil
	}
	if shouldDoMemberLeaveNotifying(transCtx.CSSet) {
		if err := doMemberLeaveNotifying(transCtx.CSSet, dag); err != nil {
			return err
		}
		return nil
	}
	return doMemberDeletion(stsVertex)
}

// normal conditions: all pods with role label set and one is leader
func doAbnormalAnalysis(transCtx *CSSetTransformContext, pods []corev1.Pod, roleMap map[string]workloads.ConsensusRole) error {
	if len(pods) != int(transCtx.CSSet.Status.Replicas) {
		// TODO(free6om): should handle this error in a more user-friendly way
		// set condition, emit event if error happens consecutive x times.
		return fmt.Errorf("cluster unhealthy: # of pods %d not equals to replicas %d", len(pods), transCtx.CSSet.Status.Replicas)
	}
	allRoleLabelSet := true
	leaderCount := 0
	for _, pod := range pods {
		roleName := getRoleName(pod)
		if len(roleName) == 0 {
			allRoleLabelSet = false
			break
		}
		if role, ok := roleMap[roleName]; ok {
			if role.IsLeader {
				leaderCount ++
			}
		}
	}
	if !allRoleLabelSet {
		return fmt.Errorf("cluster unhealthy: pod missing role label")
	}
	if leaderCount != 1 {
		return fmt.Errorf("cluster unhealthy: # of leader %d not equals 1", leaderCount)
	}
	return nil
}

// switchoverAction specified and leader in pods to be deleted
func shouldDoSwitchover(csSet *workloads.ConsensusSet, pods []corev1.Pod, roleMap map[string]workloads.ConsensusRole) bool {
	reconfiguration := csSet.Spec.MembershipReconfiguration
	if reconfiguration == nil {
		return false
	}
	if reconfiguration.SwitchoverAction == nil {
		return false
	}
	sort.SliceStable(pods, func(i, j int) bool {
		return pods[j].Name < pods[i].Name
	})
	for i := csSet.Status.Replicas - 1; i >= csSet.Spec.Replicas; i-- {
		roleName := getRoleName(pods[i])
		if role, ok := roleMap[roleName]; ok {
			if role.IsLeader {
				return true
			}
		}
	}
	return false
}

func doSwitchover(csSet *workloads.ConsensusSet, dag *graph.DAG) error {
	return doControlJob(csSet, dag, jobTypeSwitchover)
}

func doMemberLeaveNotifying(csSet *workloads.ConsensusSet, dag *graph.DAG) error {
	return doControlJob(csSet, dag, jobTypeMemberLeaveNotifying)
}

func doMemberDeletion(stsVertex *model.ObjectVertex) error {
	stsVertex.Immutable = false
	return nil
}

func doControlJob(csSet *workloads.ConsensusSet, dag *graph.DAG, jobType string) error {
	jobName := csSet.Name + "-" + jobType + "-" + rand.String(6)
	// TODO(free6om): env injection
	template := buildJobPodTemplate(csSet, jobType)
	job := builder.NewJobBuilder(csSet.Namespace, jobName).
		AddLabels(model.AppInstanceLabelKey, csSet.Name).
		AddLabels(model.KBManagedByKey, kindConsensusSet).
		AddLabels(jobTypeLabel, jobType).
		AddLabels(jobHandledLabel, jobHandledFalse).
		AddSelector(model.AppInstanceLabelKey, csSet.Name).
		AddSelector(model.KBManagedByKey, kindConsensusSet).
		AddSelector(jobTypeLabel, jobType).
		AddSelector(jobNameLabel, jobName).
		SetPodTemplateSpec(*template).
		GetObject()
	jobVertex := &model.ObjectVertex{
		Obj: job,
		Action: model.ActionPtr(model.CREATE),
	}
	dag.AddConnectRoot(jobVertex)
	return nil
}

func buildJobPodTemplate(csSet *workloads.ConsensusSet, jobType string) *corev1.PodTemplateSpec {
	reconfiguration := csSet.Spec.MembershipReconfiguration
	image := findJobImage(reconfiguration, jobType)
	container := corev1.Container{
		Name: jobType,
		Image: image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: reconfiguration.SwitchoverAction.Command,
	}
	template := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
		},
	}
	return template
}

func findJobImage(reconfiguration *workloads.MembershipReconfiguration, jobType string) string {
	if reconfiguration == nil {
		return ""
	}

	getImage := func(action *workloads.Action) string {
		if action != nil && len(action.Image) > 0 {
			return action.Image
		}
		return ""
	}
	switch jobType {
	case jobTypePromote:
		if image := getImage(reconfiguration.PromoteAction); len(image) > 0 {
			return image
		}
		fallthrough
	case jobTypeLogSync:
		if image := getImage(reconfiguration.LogSyncAction); len(image) > 0 {
			return image
		}
		fallthrough
	case jobTypeMemberLeaveNotifying:
		if image := getImage(reconfiguration.MemberLeaveAction); len(image) > 0 {
			return image
		}
		fallthrough
	case jobTypeMemberJoinNotifying:
		if image := getImage(reconfiguration.MemberJoinAction); len(image) > 0 {
			return image
		}
		fallthrough
	case jobTypeSwitchover:
		if image := getImage(reconfiguration.PromoteAction); len(image) > 0 {
			return image
		}
		return defaultActionImage
	}

	return ""
}

func shouldDoMemberLeaveNotifying(csSet *workloads.ConsensusSet) bool {
	reconfiguration := csSet.Spec.MembershipReconfiguration
	if reconfiguration != nil && reconfiguration.MemberLeaveAction != nil {
		return true
	}
	return false
}

type nextControlJob func(csSet *workloads.ConsensusSet, dag *graph.DAG) (bool, error)

func doAction(transCtx *CSSetTransformContext, dag *graph.DAG,
	jobList *batchv1.JobList, jobType string,
	stsVertex *model.ObjectVertex,
	nextControlJob nextControlJob) error {
	job := jobList.Items[0]
	switch {
	case stateControlJobInProgress(job):
		return nil
	case stateControlJobFailed(job):
		emitControlJobFailedEvent(transCtx, jobType)
		// TODO(free6om): control job policy: stop, retry, ignore
		fallthrough
	case stateControlJobSuccess(job):
		doControlJobCleanup(dag, job)
		if nextControlJob != nil {
			shouldReturn, err := nextControlJob(transCtx.CSSet, dag)
			if shouldReturn {
				return err
			}
		}
		return doMemberDeletion(stsVertex)
	}
	return nil
}

func stateControlJobInProgress(job batchv1.Job) bool {
	return job.Status.Succeeded == 0 && job.Status.Failed == 0
}

func stateControlJobFailed(job batchv1.Job) bool {
	return job.Status.Failed > 0
}

func stateControlJobSuccess(job batchv1.Job) bool {
	return job.Status.Succeeded > 0 && job.Status.Failed == 0
}

func emitControlJobFailedEvent(transCtx *CSSetTransformContext, jobType string) {
	transCtx.EventRecorder.Event(transCtx.CSSet, corev1.EventTypeWarning, strings.ToUpper(jobTypeSwitchover), jobType +" failed")
}

func doControlJobCleanup(dag *graph.DAG, job batchv1.Job) {
	// failed job: update label
	if stateControlJobFailed(job) {
		jobOld := job.DeepCopy()
		job.Labels[jobHandledLabel] = jobHandledTrue
		vertex := &model.ObjectVertex{
			Obj:    &job,
			OriObj: jobOld,
			Action: model.ActionPtr(model.UPDATE),
		}
		dag.AddConnectRoot(vertex)
		return
	}

	// succeeded job: be deleted
	vertex := &model.ObjectVertex{
		Obj: &job,
		Action: model.ActionPtr(model.DELETE),
	}
	dag.AddConnectRoot(vertex)
}

var _ graph.Transformer = &HorizontalScalingTransformer{}
