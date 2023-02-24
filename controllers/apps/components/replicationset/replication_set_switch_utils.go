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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	componetutil "github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	SwitchElectionRoleFilterName   = "SwitchElectionRoleFilter"
	SwitchElectionHealthFilterName = "SwitchElectionHealthFilter"
)

// Environment names for switchStatements
const (
	KBSwitchPromoteStmtEnvName = "KB_SWITCH_PROMOTE_STATEMENT"
	KBSwitchDemoteStmtEnvName  = "KB_SWITCH_DEMOTE_STATEMENT"
	KBSwitchFollowStmtEnvName  = "KB_SWITCH_FOLLOW_STATEMENT"

	KBSwitchOldPrimaryRoleName = "KB_OLD_PRIMARY_ROLE_NAME"
	KBSwitchNewPrimaryRoleName = "KB_NEW_PRIMARY_ROLE_NAME"
)

var defaultSwitchElectionFilters = []func() SwitchElectionFilter{
	NewSwitchElectionHealthFilter,
	NewSwitchElectionRoleFilter,
}

// SwitchHelper helps to build the switching dependencies
type SwitchHelper struct{}

// ProbeDetectManager implements the SwitchDetectManager interface with KubeBlocks Probe.
type ProbeDetectManager struct{}

// SwitchActionWithJobHandler implements the SwitchActionHandler interface with executing switch commands by k8s Job.
type SwitchActionWithJobHandler struct{}

// SwitchElectionRoleFilter implements the SwitchElectionFilter interface and is used to filter the instances which role cannot be elected as candidate primary.
type SwitchElectionRoleFilter struct{}

// SwitchElectionHealthFilter implements the SwitchElectionFilter interface and is used to filter unhealthy instances that cannot be selected as candidate primary.
type SwitchElectionHealthFilter struct{}

// SwitchRoleInfoList is a sort.Interface that Sorts a list of SwitchRoleInfo based on LagDetectInfo value.
type SwitchRoleInfoList []*SwitchRoleInfo

var _ SwitchDetectManager = &ProbeDetectManager{}

var _ SwitchActionHandler = &SwitchActionWithJobHandler{}

// Len is the implementation of the sort.Interface, calculate the length of the list of SwitchRoleInfoList.
func (sl SwitchRoleInfoList) Len() int {
	return len(sl)
}

// Swap is the implementation of the sort.Interface, exchange two items in SwitchRoleInfoList.
func (sl SwitchRoleInfoList) Swap(i, j int) {
	sl[i], sl[j] = sl[j], sl[i]
}

// Less is the implementation of the sort.Interface, sort the SwitchRoleInfo with LagDetectInfo.
func (sl SwitchRoleInfoList) Less(i, j int) bool {
	return *sl[i].LagDetectInfo < *sl[j].LagDetectInfo
}

func (f *SwitchElectionRoleFilter) Name() string {
	return SwitchElectionRoleFilterName
}

// Filter is used to filter the instance which role cannot be elected as candidate primary.
func (f *SwitchElectionRoleFilter) Filter(roleInfoList []*SwitchRoleInfo) ([]*SwitchRoleInfo, error) {
	var filterRoles []*SwitchRoleInfo
	for _, roleInfo := range roleInfoList {
		if roleInfo.RoleDetectInfo == nil {
			return nil, fmt.Errorf("pod %s RoleDetectInfo is nil, pls check", roleInfo.Pod.Name)
		}
		isPrimaryPod, err := checkObjRoleLabelIsPrimary(roleInfo.Pod)
		if err != nil {
			return filterRoles, err
		}
		if string(*roleInfo.RoleDetectInfo) != string(Primary) && !isPrimaryPod {
			filterRoles = append(filterRoles, roleInfo)
		}
	}
	return filterRoles, nil
}

// NewSwitchElectionRoleFilter initializes a SwitchElectionRoleFilter and returns it.
func NewSwitchElectionRoleFilter() SwitchElectionFilter {
	return &SwitchElectionHealthFilter{}
}

func (f *SwitchElectionHealthFilter) Name() string {
	return SwitchElectionHealthFilterName
}

// Filter is used to filter unhealthy instances that cannot be selected as candidate primary.
func (f *SwitchElectionHealthFilter) Filter(roleInfoList []*SwitchRoleInfo) ([]*SwitchRoleInfo, error) {
	var filterRoles []*SwitchRoleInfo
	for _, roleInfo := range roleInfoList {
		if roleInfo.HealthDetectInfo == nil {
			return nil, fmt.Errorf("pod %s HealthDetectInfo is nil, pls check", roleInfo.Pod.Name)
		}
		if *roleInfo.HealthDetectInfo {
			filterRoles = append(filterRoles, roleInfo)
		}
	}
	return filterRoles, nil
}

// NewSwitchElectionHealthFilter initializes a SwitchElectionHealthFilter and returns it.
func NewSwitchElectionHealthFilter() SwitchElectionFilter {
	return &SwitchElectionHealthFilter{}
}

// BuildExecSwitchCommandEnvs builds a series of envs for subsequent switching actions.
func (handler *SwitchActionWithJobHandler) BuildExecSwitchCommandEnvs(s *Switch) ([]corev1.EnvVar, error) {
	var switchEnvs []corev1.EnvVar

	// inject switchStatements as env variables
	switchStatements, err := getSwitchStatementsBySwitchPolicyType(s.SwitchResource.CompSpec.SwitchPolicy.Type, s.SwitchResource.CompDef.ReplicationSpec)
	if err != nil {
		return nil, err
	}
	promoteStmtEnv := corev1.EnvVar{
		Name:  KBSwitchPromoteStmtEnvName,
		Value: strings.Join(switchStatements.Promote, ";"),
	}
	demoteStmtEnv := corev1.EnvVar{
		Name:  KBSwitchDemoteStmtEnvName,
		Value: strings.Join(switchStatements.Demote, ";"),
	}
	followStmtEnv := corev1.EnvVar{
		Name:  KBSwitchFollowStmtEnvName,
		Value: strings.Join(switchStatements.Follow, ";"),
	}
	switchEnvs = append(switchEnvs, promoteStmtEnv, demoteStmtEnv, followStmtEnv)

	// replace secret env and merge envs defined in switchCmdExecutorConfig
	replaceSwitchCmdExecutorConfigEnv(s.SwitchResource.Cluster.Name, s.SwitchResource.CompDef.ReplicationSpec.SwitchCmdExecutorConfig)
	switchEnvs = append(switchEnvs, s.SwitchResource.CompDef.ReplicationSpec.SwitchCmdExecutorConfig.Env...)

	// inject the new primary info into the environment variable
	svcName := strings.Join([]string{s.SwitchResource.Cluster.Name, s.SwitchResource.CompSpec.Name, "headless"}, "-")
	primaryEnvs := []corev1.EnvVar{
		{
			Name:  KBSwitchOldPrimaryRoleName,
			Value: fmt.Sprintf("%s.%s", s.SwitchInstance.OldPrimaryRole.Pod.Name, svcName),
		},
		{
			Name:  KBSwitchNewPrimaryRoleName,
			Value: fmt.Sprintf("%s.%s", s.SwitchInstance.CandidatePrimaryRole.Pod.Name, svcName),
		},
	}
	switchEnvs = append(switchEnvs, primaryEnvs...)
	return switchEnvs, nil
}

// ExecSwitchCommands executes switch commands with k8s job.
func (handler *SwitchActionWithJobHandler) ExecSwitchCommands(switchEnvs []corev1.EnvVar,
	switchCmdExecutorConfig *appsv1alpha1.SwitchCmdExecutorConfig) error {
	if switchCmdExecutorConfig == nil || len(switchCmdExecutorConfig.SwitchSteps) == 0 {
		return fmt.Errorf("switchCmdExecutorConfig and SwitchSteps can not be nil")
	}
	for range switchCmdExecutorConfig.SwitchSteps {
		// TODO(xingran) render a job with to switchEnvs execute switch commands
	}
	return nil
}

// HealthDetect is the implementation of the SwitchDetectManager interface, which gets health detection information by actively calling the API provided by the probe
// TODO(xingran) Wait for the probe interface to be ready before implementation
func (pdm *ProbeDetectManager) HealthDetect(pod *corev1.Pod) (*HealthDetectResult, error) {
	var res HealthDetectResult = true
	return &res, nil
}

// RoleDetect is the implementation of the SwitchDetectManager interface, which gets role detection information by actively calling the API provided by the probe
// TODO(xingran) Wait for the probe interface to be ready before implementation
func (pdm *ProbeDetectManager) RoleDetect(pod *corev1.Pod) (*RoleDetectResult, error) {
	var res RoleDetectResult
	role := pod.Labels[intctrlutil.RoleLabelKey]
	res = DetectRoleSecondary
	if role == string(Primary) {
		res = DetectRolePrimary
	}
	return &res, nil
}

// LagDetect is the implementation of the SwitchDetectManager interface, which gets data delay detection information by actively calling the API provided by the probe
// TODO(xingran) Wait for the probe interface to be ready before implementation
func (pdm *ProbeDetectManager) LagDetect(pod *corev1.Pod) (*LagDetectResult, error) {
	var res LagDetectResult = 0
	return &res, nil
}

// getSwitchStatementsBySwitchPolicyType gets the SwitchStatements corresponding to switchPolicyType
func getSwitchStatementsBySwitchPolicyType(switchPolicyType appsv1alpha1.SwitchPolicyType,
	replicationSpec *appsv1alpha1.ReplicationSpec) (*appsv1alpha1.SwitchStatements, error) {
	if replicationSpec == nil || len(replicationSpec.SwitchPolicies) == 0 {
		return nil, fmt.Errorf("replicationSpec and replicationSpec.SwitchPolicies can not be nil")
	}
	for _, switchPolicy := range replicationSpec.SwitchPolicies {
		if switchPolicy.Type == switchPolicyType {
			return switchPolicy.SwitchStatements, nil
		}
	}
	return nil, fmt.Errorf("cannot find mapping switchStatements of switchPolicyType %s", switchPolicyType)
}

// replaceSwitchCmdExecutorConfigEnv replaces switch execute config secret env.
func replaceSwitchCmdExecutorConfigEnv(clusterName string, switchCmdExecuteConfig *appsv1alpha1.SwitchCmdExecutorConfig) {
	namedValuesMap := componetutil.GetEnvReplacementMapForConnCredential(clusterName)
	if switchCmdExecuteConfig != nil {
		switchCmdExecuteConfig.Env = componetutil.ReplaceSecretEnvVars(namedValuesMap, switchCmdExecuteConfig.Env)
	}
}

// checkSwitchStatus checks the status of every phase of Switch
func checkSwitchStatus(status *SwitchStatus) error {
	if status.SwitchPhaseStatus != SwitchPhaseStatusSucceed {
		return fmt.Errorf(status.Reason)
	}
	return nil
}
