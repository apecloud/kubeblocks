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

package replicationset

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
	// OldPrimaryPod stores the old primary pod information
	OldPrimaryPod *SwitchPodInfo
	// CandidatePrimaryPod stores the new candidate primary pod information after election, if no new primary is elected, it would be nil
	CandidatePrimaryPod *SwitchPodInfo
	// SecondariesPod stores the information of secondary pods
	SecondariesPod []*SwitchPodInfo
}

// SwitchPodInfo is used to record the pod information including health detection, role detection, data delay detection info, etc.
type SwitchPodInfo struct {
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

// SwitchDetectManager is an interface to implement various detections that high-availability relies on, including health detection, role detection, data delay detection, etc.
type SwitchDetectManager interface {
	// HealthDetect is used to implement Pod health detection
	HealthDetect(pod *corev1.Pod) (*HealthDetectResult, error)
	// RoleDetect is used to detect the role of the Pod in the database kernel
	RoleDetect(pod *corev1.Pod) (*RoleDetectResult, error)
	// LagDetect is used to detect the data delay between the secondary and the primary
	LagDetect(pod *corev1.Pod) (*LagDetectResult, error)
}

type HealthDetectResult bool

type RoleDetectResult string

type LagDetectResult int32

// SwitchActionHandler is a handler interface for performing switching actions
type SwitchActionHandler interface {
	// BuildExecSwitchCommandEnvs builds the environment variables that switchActionHandler depends on,
	// including the database account and password, the candidate primary information after the election, the switchStatement declared by the user, etc.
	BuildExecSwitchCommandEnvs(s *Switch) ([]corev1.EnvVar, error)

	// ExecSwitchCommands executes the specific switching commands defined by the user in the clusterDefinition API, and the execution channel is determined by the specific implementation
	ExecSwitchCommands(switchEnvs []corev1.EnvVar, switchCmdExecutorConfig *appsv1alpha1.SwitchCmdExecutorConfig) error
}

// SwitchElectionFilter is an interface used to filter the candidate primary during the election process.
type SwitchElectionFilter interface {
	// Name defines the name of the election filter
	Name() string

	// Filter implements the filtering logic and returns the filtered PodInfoList List
	Filter(podInfoList []*SwitchPodInfo) ([]*SwitchPodInfo, error)
}

// Detection implements the detection logic and saves the detection results to the SwitchPodInfo of the corresponding role pod of the SwitchInstance,
// if skipSecondary is true, the detection logic of the secondaries will be skipped, which is used in some scenarios where there is no need to detect the secondary,
// currently supported detection types are health detection, role detection, and delay detection.
func (s *Switch) Detection(skipSecondary bool) {
	s.SwitchStatus.SwitchPhase = SwitchPhaseDetect
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting
	if s.SwitchInstance == nil {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s detection failed because switchInstance is nil, pls check", s.SwitchResource.CompSpec.Name)
		return
	}
	doDetection := func(spi *SwitchPodInfo) {
		hd, err := s.SwitchDetectManager.HealthDetect(spi.Pod)
		if err != nil {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = err.Error()
			return
		}
		spi.HealthDetectInfo = hd

		rd, err := s.SwitchDetectManager.RoleDetect(spi.Pod)
		if err != nil {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = err.Error()
			return
		}
		spi.RoleDetectInfo = rd

		ld, err := s.SwitchDetectManager.LagDetect(spi.Pod)
		if err != nil {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = err.Error()
			return
		}
		spi.LagDetectInfo = ld
	}
	if s.SwitchInstance.OldPrimaryPod != nil {
		doDetection(s.SwitchInstance.OldPrimaryPod)
	}
	if s.SwitchInstance.CandidatePrimaryPod != nil {
		doDetection(s.SwitchInstance.CandidatePrimaryPod)
	}
	if len(s.SwitchInstance.SecondariesPod) > 0 && !skipSecondary {
		for _, secondaryPod := range s.SwitchInstance.SecondariesPod {
			doDetection(secondaryPod)
		}
	}
	if s.SwitchStatus.SwitchPhaseStatus != SwitchPhaseStatusFailed {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
	}
}

// Election implements the logic of candidate primary selection.
// election is divided into two stages: filter and priority, The filter filters the candidate primary according to the rules,
// and the priority selects the most suitable candidate primary according to the priority and return it.
func (s *Switch) Election() *SwitchPodInfo {
	var (
		filterPods []*SwitchPodInfo
		err        error
	)
	s.SwitchStatus.SwitchPhase = SwitchPhaseElect
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting
	if s.SwitchInstance == nil || len(s.SwitchInstance.SecondariesPod) == 0 {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s election failed because there is no available secondary", s.SwitchResource.CompSpec.Name)
		return nil
	}

	// do election filter
	filterPods = s.SwitchInstance.SecondariesPod
	for _, filterFunc := range defaultSwitchElectionFilters {
		filter := filterFunc()
		filterPods, err = filter.Filter(filterPods)
		if err != nil {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
			s.SwitchStatus.Reason = fmt.Sprintf("component %s switch election filter %s failed, err: %s, pls check", s.SwitchResource.CompSpec.Name, filter.Name(), err.Error())
			return nil
		}
	}

	if len(filterPods) == 0 {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s election failed because there is no available secondary after filter", s.SwitchResource.CompSpec.Name)
		return nil
	}

	if len(filterPods) == 1 {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
		return filterPods[0]
	}

	// do election priority
	// TODO(xingran): the secondary with the smallest data delay is selected as the candidate primary currently, and more rules can be added in the future
	sort.Sort(SwitchPodInfoList(filterPods))
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
	return filterPods[0]
}

// Decision implements HA decision logic. decision will judge whether HA switching can be performed based on
// instance detection information (health detection, role detection, delay detection),
// user-defined switchPolicy strategy and other information.
// When returns true, it means switching is allowed, otherwise it fails and exits.
func (s *Switch) Decision() bool {
	s.SwitchStatus.SwitchPhase = SwitchPhaseDecision
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting
	if s.SwitchInstance.OldPrimaryPod == nil || s.SwitchInstance.CandidatePrimaryPod == nil {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s switchInstance oldPrimaryPod or NewPrimaryPod is nil, pls check", s.SwitchResource.CompSpec.Name)
		return false
	}

	// candidate primary healthy check
	if !*s.SwitchInstance.CandidatePrimaryPod.HealthDetectInfo {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s new primary pod %s is not healthy, can not do switch", s.SwitchResource.CompSpec.Name, s.SwitchInstance.CandidatePrimaryPod.Pod.Name)
		return false
	}

	// candidate primary role label check
	isPrimary, err := checkObjRoleLabelIsPrimary(s.SwitchInstance.CandidatePrimaryPod.Pod)
	if err != nil {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s candidate primary %s check role label failed, err %s", s.SwitchResource.CompSpec.Name, s.SwitchInstance.CandidatePrimaryPod.Pod.Name, err.Error())
		return false
	}
	if isPrimary {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s the role label of the candidate primary has changed to primary, and the expectation is secondary", s.SwitchResource.CompSpec.Name)
		return false
	}

	// candidate primary role in kernel check
	if string(*s.SwitchInstance.CandidatePrimaryPod.RoleDetectInfo) != string(Secondary) {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s the role of the candidate primary in the kernel is not secondary", s.SwitchResource.CompSpec.Name)
		return false
	}

	makeMaxAvailabilityDecision := func() bool {
		// old primary is healthy,
		if *s.SwitchInstance.OldPrimaryPod.HealthDetectInfo {
			// The LagDetectInfo is 0, which means that the primary and the secondary data are consistent and can be switched
			if *s.SwitchInstance.CandidatePrimaryPod.LagDetectInfo == 0 {
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
		if *s.SwitchInstance.CandidatePrimaryPod.LagDetectInfo == 0 {
			s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
			return true
		}
		// Regardless of whether the primary is alive or not, if the data consistency cannot be judged, the switch will not be performed.
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s primary and secondary data consistency cannot be judged, so the switch will not be performed with MaximumAvailability switchPolicy", s.SwitchResource.CompSpec.Name)
		return false
	}

	switch s.SwitchResource.CompSpec.SwitchPolicy.Type {
	case appsv1alpha1.MaximumAvailability:
		return makeMaxAvailabilityDecision()
	case appsv1alpha1.MaximumDataProtection:
		return makeMaxDataProtectionDecision()
	case appsv1alpha1.Manual:
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s manual switch policy will not perform high-availability switching", s.SwitchResource.CompSpec.Name)
		return false
	default:
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		s.SwitchStatus.Reason = fmt.Sprintf("component %s switch policy type is not supported, pls check", s.SwitchResource.CompSpec.Name)
		return false
	}
}

// DoSwitch performs the specific action of high-availability switching.
func (s *Switch) DoSwitch() error {
	s.SwitchStatus.SwitchPhase = SwitchPhaseDoAction
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting
	if s.SwitchInstance == nil {
		return fmt.Errorf("switch target instance cannot be nil")
	}
	switchEnvs, _ := s.SwitchActionHandler.BuildExecSwitchCommandEnvs(s)
	if err := s.SwitchActionHandler.ExecSwitchCommands(switchEnvs, s.SwitchResource.CompDef.ReplicationSpec.SwitchCmdExecutorConfig); err != nil {
		s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusFailed
		return err
	}
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusSucceed
	return nil
}

// UpdateRoleLabel is used to update the role label of statefulSets and Pods after the switching is completed.
func (s *Switch) UpdateRoleLabel() error {
	s.SwitchStatus.SwitchPhase = SwitchPhaseUpdateRole
	s.SwitchStatus.SwitchPhaseStatus = SwitchPhaseStatusExecuting
	if s.SwitchInstance == nil {
		return fmt.Errorf("switch target instance cannot be nil")
	}

	// TODO(xingran) exchange the role of old primary and new primary

	return nil
}

// InitSwitchInstance initializes the switchInstance object without detection info according to the pod list under the component,
// and the detection information will be filled in the detection phase.
func (s *Switch) InitSwitchInstance(oldPrimaryIndex, newPrimaryIndex int32) error {
	var stsList = &appsv1.StatefulSetList{}
	if err := utils.GetObjectListByComponentName(s.SwitchResource.Ctx, s.SwitchResource.Cli, s.SwitchResource.Cluster, stsList, s.SwitchResource.CompSpec.Name); err != nil {
		return err
	}
	if s.SwitchInstance == nil {
		s.SwitchInstance = &SwitchInstance{
			OldPrimaryPod:       nil,
			CandidatePrimaryPod: nil,
			SecondariesPod:      make([]*SwitchPodInfo, len(stsList.Items)-1),
		}
	}
	for _, sts := range stsList.Items {
		pod, err := GetAndCheckReplicationPodByStatefulSet(s.SwitchResource.Ctx, s.SwitchResource.Cli, &sts)
		if err != nil {
			return err
		}
		switchPodInfo := &SwitchPodInfo{
			Pod:              pod,
			HealthDetectInfo: nil,
			RoleDetectInfo:   nil,
			LagDetectInfo:    nil,
		}
		switch int32(utils.GetOrdinalSts(&sts)) {
		case oldPrimaryIndex:
			s.SwitchInstance.OldPrimaryPod = switchPodInfo
		case newPrimaryIndex:
			s.SwitchInstance.CandidatePrimaryPod = switchPodInfo
		default:
			s.SwitchInstance.SecondariesPod = append(s.SwitchInstance.SecondariesPod, switchPodInfo)
		}
	}
	return nil
}

// NewSwitch creates a new Switch obj.
func NewSwitch(ctx context.Context,
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
