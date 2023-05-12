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
// i.e., reconciliation event -> retrieve current state -> do corresponding action
type HorizontalScalingTransformer struct{}

type preConditionChecker = func() bool
type postHandler = func() error

func (t *HorizontalScalingTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	csSet := transCtx.CSSet

	if model.IsObjectDeleting(csSet) {
		return nil
	}
	switch {
	case csSet.Spec.Replicas < csSet.Status.Replicas:
		return scaleIn(transCtx, dag)
	default:
		return scaleOut(transCtx, dag)
	}
}

func scaleIn(transCtx *CSSetTransformContext, dag *graph.DAG) error {
	// prepare meta info
	// get control jobs
	jobTypeList := []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
	jobLists, err := getJobList(transCtx, jobTypeList)
	if err != nil {
		return err
	}

	// get the underlying sts
	stsVertex, err := getStsVertex(dag)
	if err != nil {
		return err
	}

	// set immutable=true at beginning,
	// if abnormal analysis passes and switchover done and member leave notifying done,
	// immutable will be reset to false
	stsVertex.Immutable = true

	memberDeletionHandler := func() error {
		doMemberDeletion(stsVertex)
		return nil
	}
	scaleInChecker := func() bool {
		return transCtx.CSSet.Spec.Replicas < transCtx.CSSet.Status.Replicas
	}

	switch {
	case stateBeginning(scaleInChecker, jobLists...):
		return doScaleInBeginningAction(transCtx, dag, stsVertex)
	default:
		return doControlJobAction(transCtx, dag, jobTypeList, jobLists, memberDeletionHandler)
	}
}

func scaleOut(transCtx *CSSetTransformContext, dag *graph.DAG) error {
	// prepare meta info
	// get control jobs
	jobTypeList := []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
	jobLists, err := getJobList(transCtx, jobTypeList)
	if err != nil {
		return err
	}

	// get the underlying sts
	stsVertex, err := getStsVertex(dag)
	if err != nil {
		return err
	}

	// set immutable=true at beginning,
	// if abnormal analysis passes and switchover done and member leave notifying done,
	// immutable will be reset to false
	stsVertex.Immutable = true

	scaleOutBeginChecker := func() bool {
		return transCtx.CSSet.Spec.Replicas > transCtx.CSSet.Status.Replicas
	}
	scaleOutInProgressChecker := func() bool {
		return transCtx.CSSet.Spec.Replicas == transCtx.CSSet.Status.Replicas
	}
	switch {
	case stateBeginning(scaleOutBeginChecker, jobLists...):
		return doScaleOutBeginningAction(transCtx, dag, stsVertex, jobTypeList)
	case stateDoingMemberCreation(scaleOutInProgressChecker, jobLists...):
		return doMemberCreationAction(transCtx, dag, jobTypeList, jobLists)
	default:
		return doControlJobAction(transCtx, dag, jobTypeList, jobLists, nil)
	}
}

func stateBeginning(checker preConditionChecker, controlJobLists ...*batchv1.JobList) bool {
	if !checker() {
		return false
	}
	allListEmpty := true
	for _, list := range controlJobLists {
		if len(list.Items) > 0 {
			allListEmpty = false
			break
		}
	}
	return allListEmpty
}

func doScaleInBeginningAction(transCtx *CSSetTransformContext, dag *graph.DAG, stsVertex *model.ObjectVertex) error {
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
		if err := doControlJob(transCtx.CSSet, dag, jobTypeSwitchover, false); err != nil {
			return err
		}
		return nil
	}
	if shouldDoControlJob(transCtx.CSSet, jobTypeMemberLeaveNotifying) {
		if err := doControlJob(transCtx.CSSet, dag, jobTypeMemberLeaveNotifying, false); err != nil {
			return err
		}
		return nil
	}

	doMemberDeletion(stsVertex)
	return nil
}

func stateDoingMemberCreation(checker preConditionChecker, jobList ...*batchv1.JobList) bool {
	if !checker() {
		return false
	}
	hasJob := false
	for _, list := range jobList {
		if isControlJobRunning(*list) {
			return false
		}
		if len(list.Items) > 0 {
			hasJob = true
		}
	}
	return hasJob
}

func doMemberCreationAction(transCtx *CSSetTransformContext, dag *graph.DAG, jobTypeList []string, jobLists []*batchv1.JobList) error {
	if stateMemberCreationSucceed(transCtx.CSSet) {
		return startFirstPendingControlJob(transCtx, dag, jobTypeList, jobLists)
	}
	return nil
}

func doScaleOutBeginningAction(transCtx *CSSetTransformContext, dag *graph.DAG, stsVertex *model.ObjectVertex, jobTypeList []string) error {
	doMemberCreation(stsVertex)
	// TODO: one job for one pod
	for _, jobType := range jobTypeList {
		if shouldDoControlJob(transCtx.CSSet, jobType) {
			if err := doControlJob(transCtx.CSSet, dag, jobType, true); err != nil {
				return err
			}
		}
	}

	return nil
}

func stateMemberCreationSucceed(csSet *workloads.ConsensusSet) bool {
	return csSet.Spec.Replicas == csSet.Status.Replicas &&
		csSet.Status.ReadyReplicas == csSet.Status.Replicas
}

func getJobList(transCtx *CSSetTransformContext, jobTypeList []string) ([]*batchv1.JobList, error) {
	jobLists := make([]*batchv1.JobList, 0)
	ml := client.MatchingLabels{
		model.AppInstanceLabelKey: transCtx.CSSet.Name,
		model.KBManagedByKey:      kindConsensusSet,
		jobHandledLabel:           jobHandledFalse,
	}
	for _, jobType := range jobTypeList {
		ml[jobTypeLabel] = jobType
		jobList := &batchv1.JobList{}
		if err := transCtx.Client.List(transCtx.Context, jobList, ml); err != nil {
			return nil, err
		}
		jobLists = append(jobLists, jobList)
	}
	return jobLists, nil
}

func getStsVertex(dag *graph.DAG) (*model.ObjectVertex, error) {
	vertices := model.FindAll[*apps.StatefulSet](dag)
	if len(vertices) != 1 {
		return nil, fmt.Errorf("unexpected sts found, expected 1, but found: %d", len(vertices))
	}
	stsVertex, _ := vertices[0].(*model.ObjectVertex)
	return stsVertex, nil
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

func shouldDoControlJob(csSet *workloads.ConsensusSet, jobType string) bool {
	reconfiguration := csSet.Spec.MembershipReconfiguration
	if reconfiguration == nil {
		return false
	}
	switch jobType {
	case jobTypeSwitchover:
		return reconfiguration.SwitchoverAction != nil
	case jobTypeMemberJoinNotifying:
		return reconfiguration.MemberJoinAction != nil
	case jobTypeMemberLeaveNotifying:
		return reconfiguration.MemberLeaveAction != nil
	case jobTypeLogSync:
		return reconfiguration.LogSyncAction != nil
	case jobTypePromote:
		return reconfiguration.PromoteAction != nil
	}
	return false
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

func isControlJobRunning(jobList batchv1.JobList) bool {
	if len(jobList.Items) != 1 {
		return false
	}
	suspend := jobList.Items[0].Spec.Suspend
	if suspend != nil && *suspend {
		return false
	}
	return true
}

func isControlJobPending(jobList batchv1.JobList) bool {
	if len(jobList.Items) != 1 {
		return false
	}
	suspend := jobList.Items[0].Spec.Suspend
	if suspend != nil && *suspend {
		return true
	}
	return false
}

func doControlJobAction(transCtx *CSSetTransformContext, dag *graph.DAG, jobTypeList []string, jobLists []*batchv1.JobList, postHandler postHandler) error {
	// find running job
	var (
		jobType string
		jobList *batchv1.JobList
		index   int
	)
	for i, jt := range jobTypeList {
		if isControlJobRunning(*jobLists[i]) {
			jobType = jt
			jobList = jobLists[i]
			index = i
			break
		}
	}
	if jobLists == nil {
		return nil
	}

	// do action
	job := jobList.Items[0]
	if stateControlJobInProgress(job) {
		return nil
	}

	doControlJobCleanup(transCtx, jobType, dag, job)
	jobTypeList = jobTypeList[index+1:]
	jobLists = jobLists[index+1:]
	if len(jobTypeList) > 0 {
		return startFirstPendingControlJob(transCtx, dag, jobTypeList, jobLists)
	}
	if postHandler != nil {
		return postHandler()
	}
	return nil
}

func startFirstPendingControlJob(transCtx *CSSetTransformContext, dag *graph.DAG, jobTypeList []string, jobLists []*batchv1.JobList) error {
	for i := range jobTypeList {
		if !isControlJobPending(*jobLists[i]) {
			continue
		}
		return startControlJob(dag, jobLists[i].Items[0])
	}
	return nil
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
				leaderCount++
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

func doMemberCreation(stsVertex *model.ObjectVertex) {
	stsVertex.Immutable = false
}

func doMemberDeletion(stsVertex *model.ObjectVertex) {
	stsVertex.Immutable = false
}

func doControlJob(csSet *workloads.ConsensusSet, dag *graph.DAG, jobType string, suspend bool) error {
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
		SetSuspend(suspend).
		GetObject()
	jobVertex := &model.ObjectVertex{
		Obj:    job,
		Action: model.ActionPtr(model.CREATE),
	}
	dag.AddConnectRoot(jobVertex)
	return nil
}

func buildJobPodTemplate(csSet *workloads.ConsensusSet, jobType string) *corev1.PodTemplateSpec {
	reconfiguration := csSet.Spec.MembershipReconfiguration
	image := findJobImage(reconfiguration, jobType)
	container := corev1.Container{
		Name:            jobType,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         reconfiguration.SwitchoverAction.Command,
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

func startControlJob(dag *graph.DAG, job batchv1.Job) error {
	jobOld := job.DeepCopy()
	suspend := false
	job.Spec.Suspend = &suspend
	model.PrepareUpdate(dag, jobOld, &job)
	return nil
}

func emitControlJobFailedEvent(transCtx *CSSetTransformContext, jobType string) {
	transCtx.EventRecorder.Event(transCtx.CSSet, corev1.EventTypeWarning, strings.ToUpper(jobTypeSwitchover), jobType+" failed")
}

func doControlJobCleanup(transCtx *CSSetTransformContext, jobType string, dag *graph.DAG, job batchv1.Job) {
	switch {
	case stateControlJobFailed(job):
		// TODO(free6om): control job policy: stop, retry, ignore
		emitControlJobFailedEvent(transCtx, jobType)
		// failed job: update label
		jobOld := job.DeepCopy()
		job.Labels[jobHandledLabel] = jobHandledTrue
		model.PrepareUpdate(dag, jobOld, &job)
	case stateControlJobSuccess(job):
		// succeeded job: be deleted
		model.PrepareDelete(dag, &job)
	}
}

var _ graph.Transformer = &HorizontalScalingTransformer{}
