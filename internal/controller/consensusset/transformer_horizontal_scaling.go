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
	"regexp"
	"sort"
	"strconv"
	"strings"

	apps "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	"github.com/apecloud/kubeblocks/internal/controllerutil"
)

// HorizontalScalingTransformer handles horizontal scaling.
// mental model: event driven FSM,
// i.e., reconciliation event -> retrieve current state -> do corresponding action
type HorizontalScalingTransformer struct{}

type preConditionChecker = func() bool
type memberUpdateHandler = func()

var jobNameRegex = regexp.MustCompile("(.*)-([0-9]+)-([0-9a-zA-Z]+)$")

func (t *HorizontalScalingTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	csSet := transCtx.CSSet

	if model.IsObjectDeleting(csSet) {
		return nil
	}

	// get the underlying sts
	stsVertex, err := getStsVertex(dag)
	if err != nil {
		return err
	}
	if stsVertex.Action == nil || *stsVertex.Action != model.UPDATE {
		return nil
	}
	sts, _ := stsVertex.Obj.(*apps.StatefulSet)
	pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, sts)
	if err != nil {
		return err
	}

	// set immutable=true at beginning, to disable object Update.
	// if abnormal analysis passes and corresponding control jobs done,
	// immutable will be reset to false by following updateHandler
	stsVertex.Immutable = true
	updateHandler := func() {
		stsVertex.Immutable = false
	}
	switch {
	case csSet.Spec.Replicas < transCtx.CSSet.Status.Replicas:
		return scaleIn(transCtx, dag, pods, updateHandler)
	default:
		return scaleOut(transCtx, dag, pods, updateHandler)
	}
}

func scaleIn(transCtx *CSSetTransformContext, dag *graph.DAG, pods []corev1.Pod, memberUpdateHandler memberUpdateHandler) error {
	// prepare meta info
	// get control jobs
	jobTypeList := []string{jobTypeSwitchover, jobTypeMemberLeaveNotifying}
	jobLists, err := getJobList(transCtx, jobTypeList)
	if err != nil {
		return err
	}

	// sort descend
	sort.SliceStable(pods, func(i, j int) bool {
		return pods[j].Name < pods[i].Name
	})

	scaleInChecker := func() bool {
		return transCtx.CSSet.Spec.Replicas < transCtx.CSSet.Status.Replicas
	}

	switch {
	case stateBeginning(scaleInChecker, jobLists...):
		return doScaleInBeginningAction(transCtx, dag, pods, memberUpdateHandler)
	case stateDoingControlJob(scaleInChecker, jobLists...):
		return doControlJobAction(transCtx, dag, jobTypeList, jobLists, memberUpdateHandler)
	default:
		if memberUpdateHandler != nil {
			memberUpdateHandler()
		}
		return nil
	}
}

func scaleOut(transCtx *CSSetTransformContext, dag *graph.DAG, pods []corev1.Pod, memberUpdateHandler memberUpdateHandler) error {
	// prepare meta info
	// get control jobs
	jobTypeList := []string{jobTypeMemberJoinNotifying, jobTypeLogSync, jobTypePromote}
	jobLists, err := getJobList(transCtx, jobTypeList)
	if err != nil {
		return err
	}

	// sort ascend
	sort.SliceStable(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})

	scaleOutBeginChecker := func() bool {
		return transCtx.CSSet.Spec.Replicas > transCtx.CSSet.Status.Replicas
	}
	scaleOutInProgressChecker := func() bool {
		return transCtx.CSSet.Spec.Replicas == transCtx.CSSet.Status.Replicas
	}
	switch {
	case stateBeginning(scaleOutBeginChecker, jobLists...):
		return doScaleOutBeginningAction(transCtx, dag, pods, jobTypeList, memberUpdateHandler)
	case stateDoingMemberCreation(scaleOutInProgressChecker, jobLists...):
		return doMemberCreationAction(transCtx, dag, jobTypeList, jobLists)
	case stateDoingControlJob(scaleOutInProgressChecker, jobLists...):
		return doControlJobAction(transCtx, dag, jobTypeList, jobLists, nil)
	default:
		if memberUpdateHandler != nil {
			memberUpdateHandler()
		}
		return nil
	}
}

func stateBeginning(checker preConditionChecker, controlJobLists ...*batchv1.JobList) bool {
	if !checker() {
		return false
	}
	for _, list := range controlJobLists {
		if len(list.Items) > 0 {
			return false
		}
	}
	return true
}

func doScaleInBeginningAction(transCtx *CSSetTransformContext, dag *graph.DAG, pods []corev1.Pod, memberUpdateHandler memberUpdateHandler) error {
	// if scale to 0 replicas, delete all pods directly
	if transCtx.CSSet.Spec.Replicas == 0 {
		memberUpdateHandler()
		return nil
	}

	roleMap := composeRoleMap(*transCtx.CSSet)

	if err := doAbnormalAnalysis(transCtx, pods, roleMap); err != nil {
		return err
	}
	leaderPodName := getLeaderPodName(pods, roleMap)
	if shouldDoSwitchover(transCtx.CSSet, transCtx.OrigCSSet, pods, leaderPodName) {
		jobList, err := getJobList(transCtx, []string{jobTypeSwitchover})
		if err != nil {
			return err
		}
		if isControlJobExist(0, jobList[0]) {
			return nil
		}
		// choose pod-0 as new leader
		env := buildControlJobEnv(transCtx.CSSet, leaderPodName, pods[len(pods)-1].Name)
		if err := doControlJob(transCtx.CSSet, dag, env, jobTypeSwitchover, 0, false); err != nil {
			return err
		}
		return nil
	}
	jobType := jobTypeMemberLeaveNotifying
	if shouldDoControlJob(transCtx.CSSet, jobType) {
		jobList, err := getJobList(transCtx, []string{jobType})
		if err != nil {
			return err
		}
		for i := transCtx.OrigCSSet.Status.Replicas; i > transCtx.CSSet.Spec.Replicas; i-- {
			ordinal := int(i - 1)
			podName := fmt.Sprintf("%s-%d", transCtx.CSSet.Name, ordinal)

			if isControlJobExist(ordinal, jobList[0]) {
				continue
			}
			env := buildControlJobEnv(transCtx.CSSet, leaderPodName, podName)
			if err := doControlJob(transCtx.CSSet, dag, env, jobType, ordinal, false); err != nil {
				return err
			}
		}
		return nil
	}

	memberUpdateHandler()
	return nil
}

func isControlJobExist(ordinal int, jobList *batchv1.JobList) bool {
	if jobList == nil {
		return false
	}
	for _, job := range jobList.Items {
		if !job.DeletionTimestamp.IsZero() {
			continue
		}
		subMatches := jobNameRegex.FindStringSubmatch(job.Name)
		if len(subMatches) < 4 {
			continue
		}
		if i, err := strconv.Atoi(subMatches[2]); err == nil {
			if ordinal == i {
				return true
			}
		}
	}
	return false
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
		return startFirstPendingControlJob(dag, jobTypeList, jobLists)
	}
	return nil
}

func doScaleOutBeginningAction(transCtx *CSSetTransformContext, dag *graph.DAG, pods []corev1.Pod, jobTypeList []string, memberUpdateHandler memberUpdateHandler) error {
	roleMap := composeRoleMap(*transCtx.CSSet)
	if err := doAbnormalAnalysis(transCtx, pods, roleMap); err != nil {
		return err
	}

	// create new pods
	memberUpdateHandler()

	// create control jobs
	leaderPodName := getLeaderPodName(pods, roleMap)
	for _, jobType := range jobTypeList {
		if !shouldDoControlJob(transCtx.CSSet, jobType) {
			continue
		}
		jobList, err := getJobList(transCtx, []string{jobType})
		if err != nil {
			return err
		}
		for i := transCtx.OrigCSSet.Status.Replicas; i < transCtx.CSSet.Spec.Replicas; i++ {
			ordinal := int(i)
			podName := fmt.Sprintf("%s-%d", transCtx.CSSet.Name, ordinal)
			if isControlJobExist(ordinal, jobList[0]) {
				continue
			}
			env := buildControlJobEnv(transCtx.CSSet, leaderPodName, podName)
			if err := doControlJob(transCtx.CSSet, dag, env, jobType, ordinal, true); err != nil {
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

func shouldDoSwitchover(csSetNew, csSetOld *workloads.ConsensusSet, pods []corev1.Pod, leaderPodName string) bool {
	reconfiguration := csSetNew.Spec.MembershipReconfiguration
	if reconfiguration == nil {
		return false
	}
	if reconfiguration.SwitchoverAction == nil {
		return false
	}
	for i := csSetOld.Status.Replicas - 1; i >= csSetNew.Spec.Replicas; i-- {
		if pods[i].Name == leaderPodName {
			return true
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

func stateDoingControlJob(checker preConditionChecker, controlJobLists ...*batchv1.JobList) bool {
	if !checker() {
		return false
	}
	for _, jobList := range controlJobLists {
		if isControlJobRunning(*jobList) {
			return true
		}
	}
	return false
}

// in progress if any job is in progress
func stateControlJobInProgress(jobList *batchv1.JobList) bool {
	for _, job := range jobList.Items {
		if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
			return true
		}
	}
	return false
}

// failed if any job is failed
func stateControlJobFailed(jobList *batchv1.JobList) bool {
	for _, job := range jobList.Items {
		if job.Status.Failed > 0 {
			return true
		}
	}
	return false
}

// success if all jobs are succeeded
func stateControlJobSuccess(jobList *batchv1.JobList) bool {
	for _, job := range jobList.Items {
		if job.Status.Succeeded == 0 || job.Status.Failed > 0 {
			return false
		}
	}
	return true
}

func isControlJobRunning(jobList batchv1.JobList) bool {
	if len(jobList.Items) == 0 {
		return false
	}
	for _, job := range jobList.Items {
		if !job.DeletionTimestamp.IsZero() {
			continue
		}
		suspend := job.Spec.Suspend
		if suspend != nil && *suspend {
			return false
		}
	}
	return true
}

func isControlJobPending(jobList batchv1.JobList) bool {
	if len(jobList.Items) == 0 {
		return false
	}
	for _, job := range jobList.Items {
		if !job.DeletionTimestamp.IsZero() {
			continue
		}
		suspend := job.Spec.Suspend
		if suspend != nil && *suspend {
			return true
		}
	}
	return false
}

func doControlJobAction(transCtx *CSSetTransformContext, dag *graph.DAG, jobTypeList []string, jobLists []*batchv1.JobList, memberUpdateHandler memberUpdateHandler) error {
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
	if jobList == nil {
		return nil
	}

	// do action
	if stateControlJobInProgress(jobList) {
		return nil
	}

	doControlJobCleanup(transCtx, jobType, dag, jobList)
	jobTypeList = jobTypeList[index+1:]
	jobLists = jobLists[index+1:]
	if len(jobTypeList) > 0 {
		return startFirstPendingControlJob(dag, jobTypeList, jobLists)
	}
	if memberUpdateHandler != nil {
		memberUpdateHandler()
	}
	return nil
}

func startFirstPendingControlJob(dag *graph.DAG, jobTypeList []string, jobLists []*batchv1.JobList) error {
	for i := range jobTypeList {
		if !isControlJobPending(*jobLists[i]) {
			continue
		}
		return startControlJob(dag, jobLists[i])
	}
	return nil
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

func getLeaderPodName(pods []corev1.Pod, roleMap map[string]workloads.ConsensusRole) string {
	for _, pod := range pods {
		roleName := getRoleName(pod)
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		if role.IsLeader {
			return pod.Name
		}
	}
	return ""
}

// normal conditions: all pods with role label set and one is leader
func doAbnormalAnalysis(transCtx *CSSetTransformContext, pods []corev1.Pod, roleMap map[string]workloads.ConsensusRole) error {
	if len(pods) != int(transCtx.CSSet.Status.Replicas) {
		// TODO(free6om): should handle this error in a more user-friendly way
		// set condition, emit event if error happens consecutive x times.
		return fmt.Errorf("cluster unhealthy: # of pods %d not equals to replicas %d", len(pods), transCtx.OrigCSSet.Status.Replicas)
	}
	// if no pods, no need to check the following conditions
	if len(pods) == 0 {
		return nil
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

// ordinal is the ordinal of pod which this control job apply to
func doControlJob(csSet *workloads.ConsensusSet, dag *graph.DAG, env []corev1.EnvVar, jobType string, ordinal int, suspend bool) error {
	jobName := fmt.Sprintf("%s-%s-%d-%d", csSet.Name, jobType, ordinal, csSet.Generation)
	template := buildJobPodTemplate(csSet, env, jobType)
	job := builder.NewJobBuilder(csSet.Namespace, jobName).
		AddLabels(model.AppInstanceLabelKey, csSet.Name).
		AddLabels(model.KBManagedByKey, kindConsensusSet).
		AddLabels(jobTypeLabel, jobType).
		AddLabels(jobHandledLabel, jobHandledFalse).
		SetPodTemplateSpec(*template).
		SetSuspend(suspend).
		GetObject()
	if err := controllerutil.SetOwnership(csSet, job, model.GetScheme(), csSetFinalizerName); err != nil {
		return err
	}
	model.PrepareCreate(dag, job)
	return nil
}

func buildJobPodTemplate(csSet *workloads.ConsensusSet, env []corev1.EnvVar, jobType string) *corev1.PodTemplateSpec {
	credential := csSet.Spec.Credential
	credentialEnv := make([]corev1.EnvVar, 0)
	if credential != nil {
		credentialEnv = append(credentialEnv,
			corev1.EnvVar{
				Name:      usernameCredentialVarName,
				Value:     credential.Username.Value,
				ValueFrom: credential.Username.ValueFrom,
			},
			corev1.EnvVar{
				Name:      passwordCredentialVarName,
				Value:     credential.Password.Value,
				ValueFrom: credential.Password.ValueFrom,
			})
	}
	env = append(env, credentialEnv...)
	reconfiguration := csSet.Spec.MembershipReconfiguration
	image := findJobImage(reconfiguration, jobType)
	command := getJobCommand(reconfiguration, jobType)
	container := corev1.Container{
		Name:            jobType,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         command,
		Env:             env,
	}
	template := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers:    []corev1.Container{container},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
	return template
}

func buildControlJobEnv(csSet *workloads.ConsensusSet, leader, target string) []corev1.EnvVar {
	svcName := getHeadlessSvcName(*csSet)
	leaderHost := fmt.Sprintf("%s.%s", leader, svcName)
	targetHost := fmt.Sprintf("%s.%s", target, svcName)
	svcPort := findSvcPort(*csSet)
	return []corev1.EnvVar{
		{
			Name:  leaderHostVarName,
			Value: leaderHost,
		},
		{
			Name:  servicePortVarName,
			Value: strconv.Itoa(svcPort),
		},
		{
			Name:  targetHostVarName,
			Value: targetHost,
		},
	}
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

func getJobCommand(reconfiguration *workloads.MembershipReconfiguration, jobType string) []string {
	if reconfiguration == nil {
		return nil
	}
	getCommand := func(action *workloads.Action) []string {
		if action == nil {
			return nil
		}
		return action.Command
	}
	switch jobType {
	case jobTypeSwitchover:
		return getCommand(reconfiguration.SwitchoverAction)
	case jobTypeMemberJoinNotifying:
		return getCommand(reconfiguration.MemberJoinAction)
	case jobTypeMemberLeaveNotifying:
		return getCommand(reconfiguration.MemberLeaveAction)
	case jobTypeLogSync:
		return getCommand(reconfiguration.LogSyncAction)
	case jobTypePromote:
		return getCommand(reconfiguration.PromoteAction)
	}
	return nil
}

func startControlJob(dag *graph.DAG, jobList *batchv1.JobList) error {
	for _, job := range jobList.Items {
		jobOld := job.DeepCopy()
		jobNew := jobOld.DeepCopy()
		suspend := false
		jobNew.Spec.Suspend = &suspend
		model.PrepareUpdate(dag, jobOld, jobNew)
	}
	return nil
}

func emitControlJobFailedEvent(transCtx *CSSetTransformContext, jobType string) {
	transCtx.EventRecorder.Event(transCtx.CSSet, corev1.EventTypeWarning, strings.ToUpper(jobTypeSwitchover), jobType+" failed")
}

func doControlJobCleanup(transCtx *CSSetTransformContext, jobType string, dag *graph.DAG, jobList *batchv1.JobList) {
	switch {
	case stateControlJobFailed(jobList):
		// TODO(free6om): control job policy: stop, retry, ignore
		emitControlJobFailedEvent(transCtx, jobType)
		fallthrough
	case stateControlJobSuccess(jobList):
		for _, job := range jobList.Items {
			jobOld := job.DeepCopy()
			jobNew := jobOld.DeepCopy()
			jobNew.Labels[jobHandledLabel] = jobHandledTrue
			model.PrepareUpdate(dag, jobOld, jobNew)
		}
	}
}

var _ graph.Transformer = &HorizontalScalingTransformer{}
