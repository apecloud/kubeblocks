/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package replication

import (
	"context"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	utils "github.com/apecloud/kubeblocks/controllers/apps/components/util"
)

// Switch is the main high-availability switching implementation.
type Switch struct {
	SwitchResource      *SwitchResource
	SwitchInstance      *SwitchInstance
	SwitchStatus        *SwitchStatus
	SwitchDetectManager SwitchDetectManager
	SwitchActionHandler SwitchActionHandler
}

// SwitchResource is used to record resources that high-availability switching depends on, such as cluster information, component information, etc.
type SwitchResource struct {
	Ctx      context.Context
	Cli      client.Client
	Cluster  *appsv1alpha1.Cluster
	CompDef  *appsv1alpha1.ClusterComponentDefinition
	CompSpec *appsv1alpha1.ClusterComponentSpec
	Recorder record.EventRecorder
}

// SwitchInstance is used to record the instance information of switching.
type SwitchInstance struct {
	// OldPrimaryRole stores the old primary role information
	OldPrimaryRole *SwitchRoleInfo
	// CandidatePrimaryRole stores the new candidate primary role information after election, if no new primary is elected, it would be nil
	CandidatePrimaryRole *SwitchRoleInfo
	// SecondariesRole stores the information of secondary roles
	SecondariesRole []*SwitchRoleInfo
}

// SwitchRoleInfo is used to record the role information including health detection, role detection, data delay detection info, etc.
type SwitchRoleInfo struct {
	// k8s pod obj
	Pod *corev1.Pod
	// HealthDetectInfo stores the results of health detection
	HealthDetectInfo *HealthDetectResult
	// RoleDetectInfo stores the results of kernel role detection
	RoleDetectInfo *RoleDetectResult
	// LagDetectInfo stores the results of data delay detection
	LagDetectInfo *LagDetectResult
}

// SwitchStatus defines the status of high-availability switching.
type SwitchStatus struct {
	// SwitchPhase defines the various phases of high-availability switching
	SwitchPhase SwitchPhase
	// SwitchPhaseStatus defines the state of each phase of high-availability switching
	SwitchPhaseStatus SwitchPhaseStatus
	// a brief single-word reason of current SwitchPhase and SwitchPhaseStatus.
	Reason string
	// a brief message explaining of current SwitchPhase and SwitchPhaseStatus.
	Message string
}

// SwitchPhaseStatus defines the status of switching phase.
type SwitchPhaseStatus string

// SwitchPhase defines the phase of switching.
type SwitchPhase string

// SwitchDetectManager is an interface to implement various detections that high-availability depends on, including health detection, role detection, data delay detection, etc.
type SwitchDetectManager interface {
	// healthDetect is used to implement Pod health detection
	healthDetect(pod *corev1.Pod) (*HealthDetectResult, error)
	// roleDetect is used to detect the role of the Pod in the database kernel
	roleDetect(pod *corev1.Pod) (*RoleDetectResult, error)
	// lagDetect is used to detect the data delay between the secondary and the primary
	lagDetect(pod *corev1.Pod) (*LagDetectResult, error)
}

type HealthDetectResult bool

type RoleDetectResult string

type LagDetectResult int32

// SwitchActionHandler is a handler interface for performing switching actions
type SwitchActionHandler interface {
	// buildExecSwitchCommandEnvs builds the environment variables that switchActionHandler depends on,
	// including the database account and password, the candidate primary information after the election, the switchStatement declared by the user, etc.
	buildExecSwitchCommandEnvs(s *Switch) ([]corev1.EnvVar, error)

	// execSwitchCommands executes the specific switching commands defined by the user in the clusterDefinition API, and the execution channel is determined by the specific implementation
	execSwitchCommands(s *Switch, switchEnvs []corev1.EnvVar) error
}

// SwitchElectionFilter is an interface used to filter the candidate primary during the election process.
type SwitchElectionFilter interface {
	// name defines the name of the election filter
	name() string

	// filter implements the filtering logic and returns the filtered PodInfoList List
	filter(roleInfoList []*SwitchRoleInfo) ([]*SwitchRoleInfo, error)
}

const (
	SwitchPhasePrepare    SwitchPhase = "prepare"
	SwitchPhaseElect      SwitchPhase = "election"
	SwitchPhaseDetect     SwitchPhase = "detection"
	SwitchPhaseDecision   SwitchPhase = "decision"
	SwitchPhaseDoAction   SwitchPhase = "doAction"
	SwitchPhaseUpdateRole SwitchPhase = "updateRole"

	SwitchPhaseStatusExecuting SwitchPhaseStatus = "executing"
	SwitchPhaseStatusFailed    SwitchPhaseStatus = "failed"
	SwitchPhaseStatusSucceed   SwitchPhaseStatus = "succeed"
	SwitchPhaseStatusUnknown   SwitchPhaseStatus = "unknown"
)

const (
	DetectRolePrimary   RoleDetectResult = "primary"
	DetectRoleSecondary RoleDetectResult = "secondary"
)

// detection implements the detection logic and saves the detection results to the SwitchRoleInfo of the corresponding role pod of the SwitchInstance,
// if skipSecondary is true, the detection logic of the secondaries will be skipped, which is used in some scenarios where there is no need to detect the secondary,
// currently supported detection types are health detection, role detection, and delay detection.
func (s *Switch) detection(skipSecondary bool) {
	s.SwitchStatus.SwitchPhase = SwitchPhaseDetect
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting
	if s.SwitchInstance == nil {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s detection failed because switchInstance is nil, pls check", s.SwitchResource.CompSpec.Name)
		return
	}
	doDetection := func(sri *SwitchRoleInfo) {
		hd, err := s.SwitchDetectManager.healthDetect(sri.Pod)
		if err != nil {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = err.Error()
			return
		}
		sri.HealthDetectInfo = hd

		rd, err := s.SwitchDetectManager.roleDetect(sri.Pod)
		if err != nil {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = err.Error()
			return
		}
		sri.RoleDetectInfo = rd

		ld, err := s.SwitchDetectManager.lagDetect(sri.Pod)
		if err != nil {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = err.Error()
			return
		}
		sri.LagDetectInfo = ld
	}
	if s.SwitchInstance.OldPrimaryRole != nil {
		doDetection(s.SwitchInstance.OldPrimaryRole)
	}
	if s.SwitchInstance.CandidatePrimaryRole != nil {
		doDetection(s.SwitchInstance.CandidatePrimaryRole)
	}
	if !skipSecondary {
		for _, secondaryRole := range s.SwitchInstance.SecondariesRole {
			doDetection(secondaryRole)
		}
	}
	if s.SwitchStatus.SwitchPhaseStatus != SwitchPhaseStatusFailed {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
	}
}

// election implements the logic of candidate primary selection.
// election is divided into two stages: filter and priority, The filter filters the candidate primary according to the rules,
// and the priority selects the most suitable candidate primary according to the priority and return it.
func (s *Switch) election() *SwitchRoleInfo {
	var (
		filterRoles []*SwitchRoleInfo
		err         error
	)
	s.SwitchStatus.SwitchPhase = SwitchPhaseElect
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting
	if s.SwitchInstance == nil || len(s.SwitchInstance.SecondariesRole) == 0 {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s election failed because there is no available secondary", s.SwitchResource.CompSpec.Name)
		return nil
	}

	// do election filter
	filterRoles = s.SwitchInstance.SecondariesRole
	for _, filterFunc := range defaultSwitchElectionFilters {
		filter := filterFunc()
		filterRoles, err = filter.filter(filterRoles)
		if err != nil {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = fmt.Sprintf("component %s switch election filter %s failed, err: %s, pls check", s.SwitchResource.CompSpec.Name, filter.name(), err.Error())
			return nil
		}
	}

	if len(filterRoles) == 0 {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s election failed because there is no available secondary after filter", s.SwitchResource.CompSpec.Name)
		return nil
	}

	if len(filterRoles) == 1 {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
		return filterRoles[0]
	}

	// do election priority
	// TODO(xingran): the secondary with the smallest data delay is selected as the candidate primary currently, and more rules can be added in the future
	sort.Sort(SwitchRoleInfoList(filterRoles))
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
	return filterRoles[0]
}

// decision implements HA decision logic. decision will judge whether HA switching can be performed based on
// instance detection information (health detection, role detection, delay detection),
// user-defined switchPolicy strategy and other information.
// When returns true, it means switching is allowed, otherwise it fails and exits.
func (s *Switch) decision() bool {
	s.SwitchStatus.SwitchPhase = SwitchPhaseDecision
	if s.SwitchInstance.OldPrimaryRole == nil || s.SwitchInstance.CandidatePrimaryRole == nil {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s switchInstance OldPrimaryRole or NewPrimaryPod is nil, pls check", s.SwitchResource.CompSpec.Name)
		return false
	}

	// candidate primary healthy check
	if !*s.SwitchInstance.CandidatePrimaryRole.HealthDetectInfo {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s new primary pod %s is not healthy, can not do switch", s.SwitchResource.CompSpec.Name, s.SwitchInstance.CandidatePrimaryRole.Pod.Name)
		return false
	}

	// candidate primary role label check
	isPrimary, err := checkObjRoleLabelIsPrimary(s.SwitchInstance.CandidatePrimaryRole.Pod)
	if err != nil {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s candidate primary %s check role label failed, err %s", s.SwitchResource.CompSpec.Name, s.SwitchInstance.CandidatePrimaryRole.Pod.Name, err.Error())
		return false
	}
	if isPrimary {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s the role label of the candidate primary has changed to primary, and the expectation is secondary", s.SwitchResource.CompSpec.Name)
		return false
	}

	// candidate primary role in kernel check
	if string(*s.SwitchInstance.CandidatePrimaryRole.RoleDetectInfo) != string(Secondary) {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s the role of the candidate primary in the kernel is not secondary", s.SwitchResource.CompSpec.Name)
		return false
	}

	makeMaxAvailabilityDecision := func() bool {
		// old primary is alive, check the data delay of candidate primary
		if *s.SwitchInstance.OldPrimaryRole.HealthDetectInfo {
			// The LagDetectInfo is 0, which means that the primary and the secondary data are consistent and can be switched
			if *s.SwitchInstance.CandidatePrimaryRole.LagDetectInfo == 0 {
				s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
				return true
			}
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = fmt.Sprintf("component %s old primary is still alive, primary and secondary data are not consistent, can not do switch", s.SwitchResource.CompSpec.Name)
			return false
		}
		// old primary is down, perform high-availability switching immediately
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
		return true
	}

	makeMaxDataProtectionDecision := func() bool {
		// The LagDetectInfo is 0, which means that the primary and the secondary data are consistent and can be switched
		if *s.SwitchInstance.CandidatePrimaryRole.LagDetectInfo == 0 {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
			return true
		}
		// Regardless of whether the primary is alive or not, if the data consistency cannot be judged, the switch will not be performed.
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s primary and secondary data consistency cannot be judged, so the switch will not be performed with MaximumDataProtection switchPolicy", s.SwitchResource.CompSpec.Name)
		return false
	}

	switch s.SwitchResource.CompSpec.SwitchPolicy.Type {
	case appsv1alpha1.MaximumAvailability:
		return makeMaxAvailabilityDecision()
	case appsv1alpha1.MaximumDataProtection:
		return makeMaxDataProtectionDecision()
	case appsv1alpha1.Noop:
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s Noop switch policy will not perform high-availability switching", s.SwitchResource.CompSpec.Name)
		return false
	}
	return false
}

// doSwitch performs the specific action of high-availability switching.
func (s *Switch) doSwitch() error {
	s.SwitchStatus.SwitchPhase = SwitchPhaseDoAction
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting
	switchEnvs, _ := s.SwitchActionHandler.buildExecSwitchCommandEnvs(s)
	if err := s.SwitchActionHandler.execSwitchCommands(s, switchEnvs); err != nil {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		return err
	}
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
	return nil
}

// updateRoleLabel is used to update the role label of statefulSets and Pods after the switching is completed.
func (s *Switch) updateRoleLabel() error {
	s.SwitchStatus.SwitchPhase = SwitchPhaseUpdateRole
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting

	var stsList = &appsv1.StatefulSetList{}
	if err := utils.GetObjectListByComponentName(s.SwitchResource.Ctx, s.SwitchResource.Cli,
		*s.SwitchResource.Cluster, stsList, s.SwitchResource.CompSpec.Name); err != nil {
		return err
	}

	for _, sts := range stsList.Items {
		if utils.IsMemberOf(&sts, s.SwitchInstance.OldPrimaryRole.Pod) {
			if err := updateObjRoleLabel(s.SwitchResource.Ctx, s.SwitchResource.Cli, sts, string(Secondary)); err != nil {
				return err
			}
			if err := updateObjRoleLabel(s.SwitchResource.Ctx, s.SwitchResource.Cli, *s.SwitchInstance.OldPrimaryRole.Pod, string(Secondary)); err != nil {
				return err
			}
		}
		if utils.IsMemberOf(&sts, s.SwitchInstance.CandidatePrimaryRole.Pod) {
			if err := updateObjRoleLabel(s.SwitchResource.Ctx, s.SwitchResource.Cli, sts, string(Primary)); err != nil {
				return err
			}
			if err := updateObjRoleLabel(s.SwitchResource.Ctx, s.SwitchResource.Cli, *s.SwitchInstance.CandidatePrimaryRole.Pod, string(Primary)); err != nil {
				return err
			}
		}
	}
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
	return nil
}

// initSwitchInstance initializes the switchInstance object without detection info according to the pod list under the component,
// and the detection information will be filled in the detection phase.
func (s *Switch) initSwitchInstance(oldPrimaryIndex, newPrimaryIndex int32) error {
	var podList = &corev1.PodList{}
	if err := utils.GetObjectListByComponentName(s.SwitchResource.Ctx, s.SwitchResource.Cli,
		*s.SwitchResource.Cluster, podList, s.SwitchResource.CompSpec.Name); err != nil {
		return err
	}
	if s.SwitchInstance == nil {
		s.SwitchInstance = &SwitchInstance{
			OldPrimaryRole:       nil,
			CandidatePrimaryRole: nil,
			SecondariesRole:      make([]*SwitchRoleInfo, 0),
		}
	}
	for _, pod := range podList.Items {
		sri := &SwitchRoleInfo{
			Pod:              &pod,
			HealthDetectInfo: nil,
			RoleDetectInfo:   nil,
			LagDetectInfo:    nil,
		}
		_, o := utils.ParseParentNameAndOrdinal(pod.Name)
		switch o {
		case oldPrimaryIndex:
			s.SwitchInstance.OldPrimaryRole = sri
		case newPrimaryIndex:
			s.SwitchInstance.CandidatePrimaryRole = sri
		default:
			s.SwitchInstance.SecondariesRole = append(s.SwitchInstance.SecondariesRole, sri)
		}
	}
	return nil
}

// newSwitch creates a new Switch obj.
func newSwitch(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	recorder record.EventRecorder,
	switchInstance *SwitchInstance,
	switchStatus *SwitchStatus,
	switchDetectManager SwitchDetectManager,
	switchActionHandler SwitchActionHandler) *Switch {
	switchResource := &SwitchResource{
		Ctx:      ctx,
		Cli:      cli,
		Cluster:  cluster,
		CompDef:  compDef,
		CompSpec: compSpec,
		Recorder: recorder,
	}
	if switchStatus == nil {
		switchStatus = &SwitchStatus{
			SwitchPhase:       SwitchPhasePrepare,
			SwitchPhaseStatus: SwitchPhaseStatusUnknown,
			Reason:            "",
			Message:           "",
		}
	}
	if switchDetectManager == nil {
		switchDetectManager = &ProbeDetectManager{}
	}
	if switchActionHandler == nil {
		switchActionHandler = &SwitchActionWithJobHandler{}
	}
	return &Switch{
		SwitchResource:      switchResource,
		SwitchInstance:      switchInstance,
		SwitchStatus:        switchStatus,
		SwitchDetectManager: switchDetectManager,
		SwitchActionHandler: switchActionHandler,
	}
}
