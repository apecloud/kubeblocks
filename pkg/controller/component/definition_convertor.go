/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package component

import (
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/apiutil"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// TODO(component): type check

// buildComponentDefinitionByConversion builds a ComponentDefinition from a ClusterComponentDefinition and a ClusterComponentVersion.
func buildComponentDefinitionByConversion(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) (*appsv1alpha1.ComponentDefinition, error) {
	if clusterCompDef == nil {
		return nil, nil
	}
	convertors := map[string]convertor{
		"provider":               &compDefProviderConvertor{},
		"description":            &compDefDescriptionConvertor{},
		"servicekind":            &compDefServiceKindConvertor{},
		"serviceversion":         &compDefServiceVersionConvertor{},
		"runtime":                &compDefRuntimeConvertor{},
		"vars":                   &compDefVarsConvertor{},
		"volumes":                &compDefVolumesConvertor{},
		"hostnetwork":            &compDefHostNetworkConvertor{},
		"services":               &compDefServicesConvertor{},
		"configs":                &compDefConfigsConvertor{},
		"logconfigs":             &compDefLogConfigsConvertor{},
		"scripts":                &compDefScriptsConvertor{},
		"policyrules":            &compDefPolicyRulesConvertor{},
		"labels":                 &compDefLabelsConvertor{},
		"replicasLimit":          &compDefReplicasLimitConvertor{},
		"systemaccounts":         &compDefSystemAccountsConvertor{},
		"updatestrategy":         &compDefUpdateStrategyConvertor{},
		"roles":                  &compDefRolesConvertor{},
		"lifecycleactions":       &compDefLifecycleActionsConvertor{},
		"servicerefdeclarations": &compDefServiceRefDeclarationsConvertor{},
		"exporter":               &compDefExporterConvertor{},
	}
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := covertObject(convertors, &compDef.Spec, clusterCompDef); err != nil {
		return nil, err
	}
	return compDef, nil
}

// compDefProviderConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Provider.
type compDefProviderConvertor struct{}

func (c *compDefProviderConvertor) convert(args ...any) (any, error) {
	return "", nil
}

// compDefDescriptionConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Description.
type compDefDescriptionConvertor struct{}

func (c *compDefDescriptionConvertor) convert(args ...any) (any, error) {
	return "", nil
}

// compDefServiceKindConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ServiceKind.
type compDefServiceKindConvertor struct{}

func (c *compDefServiceKindConvertor) convert(args ...any) (any, error) {
	return "", nil
}

// compDefServiceVersionConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ServiceVersion.
type compDefServiceVersionConvertor struct{}

func (c *compDefServiceVersionConvertor) convert(args ...any) (any, error) {
	return "", nil
}

// compDefRuntimeConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Runtime.
type compDefRuntimeConvertor struct{}

func (c *compDefRuntimeConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	if clusterCompDef.PodSpec == nil {
		return nil, fmt.Errorf("no pod spec")
	}
	podSpec := clusterCompDef.PodSpec.DeepCopy()
	return *podSpec, nil
}

type compDefVarsConvertor struct{}

func (c *compDefVarsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return c.convertHostNetworkVars(clusterCompDef), nil
}

func (c *compDefVarsConvertor) convertHostNetworkVars(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) []appsv1alpha1.EnvVar {
	hostNetwork, err := convertHostNetwork(clusterCompDef)
	if err != nil || hostNetwork == nil || len(hostNetwork.ContainerPorts) == 0 {
		return nil
	}
	vars := make([]appsv1alpha1.EnvVar, 0)
	for _, cc := range hostNetwork.ContainerPorts {
		for _, port := range cc.Ports {
			vars = append(vars, appsv1alpha1.EnvVar{
				Name: apiutil.HostNetworkDynamicPortVarName(cc.Container, port),
				ValueFrom: &appsv1alpha1.VarSource{
					HostNetworkVarRef: &appsv1alpha1.HostNetworkVarSelector{
						ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
							Optional: func() *bool { optional := false; return &optional }(),
						},
						HostNetworkVars: appsv1alpha1.HostNetworkVars{
							Container: &appsv1alpha1.ContainerVars{
								Name: cc.Container,
								Port: &appsv1alpha1.NamedVar{
									Name:   port,
									Option: &appsv1alpha1.VarRequired,
								},
							},
						},
					},
				},
			})
		}
	}
	return vars
}

// compDefVolumesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Volumes.
type compDefVolumesConvertor struct{}

func (c *compDefVolumesConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefHostNetworkConvertor converts the given object into ComponentDefinition.Spec.HostNetwork.
type compDefHostNetworkConvertor struct{}

func (c *compDefHostNetworkConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return convertHostNetwork(clusterCompDef)
}

func convertHostNetwork(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) (*appsv1alpha1.HostNetwork, error) {
	if clusterCompDef.PodSpec == nil || !clusterCompDef.PodSpec.HostNetwork {
		return nil, nil
	}

	hostNetwork := &appsv1alpha1.HostNetwork{
		ContainerPorts: []appsv1alpha1.HostNetworkContainerPort{},
	}
	for _, container := range clusterCompDef.PodSpec.Containers {
		cp := appsv1alpha1.HostNetworkContainerPort{
			Container: container.Name,
			Ports:     []string{},
		}
		for _, port := range container.Ports {
			if apiutil.IsHostNetworkDynamicPort(port.ContainerPort) {
				cp.Ports = append(cp.Ports, port.Name)
			}
		}
		if len(cp.Ports) > 0 {
			hostNetwork.ContainerPorts = append(hostNetwork.ContainerPorts, cp)
		}
	}
	return hostNetwork, nil
}

// compDefServicesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Services.
type compDefServicesConvertor struct{}

func (c *compDefServicesConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefConfigsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Configs.
type compDefConfigsConvertor struct{}

func (c *compDefConfigsConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefLogConfigsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.LogConfigs.
type compDefLogConfigsConvertor struct{}

func (c *compDefLogConfigsConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefScriptsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Scripts.
type compDefScriptsConvertor struct{}

func (c *compDefScriptsConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefPolicyRulesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.PolicyRules.
type compDefPolicyRulesConvertor struct{}

func (c *compDefPolicyRulesConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefLabelsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Labels.
type compDefLabelsConvertor struct{}

func (c *compDefLabelsConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

type compDefReplicasLimitConvertor struct{}

func (c *compDefReplicasLimitConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefSystemAccountsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.SystemAccounts.
type compDefSystemAccountsConvertor struct{}

func (c *compDefSystemAccountsConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.UpdateStrategy.
type compDefUpdateStrategyConvertor struct{}

func (c *compDefUpdateStrategyConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	var strategy *appsv1alpha1.UpdateStrategy
	switch clusterCompDef.WorkloadType {
	case appsv1alpha1.Consensus:
		if clusterCompDef.RSMSpec != nil && clusterCompDef.RSMSpec.MemberUpdateStrategy != nil {
			strategy = func() *appsv1alpha1.UpdateStrategy {
				s := appsv1alpha1.UpdateStrategy(*clusterCompDef.RSMSpec.MemberUpdateStrategy)
				return &s
			}()
		}
		if clusterCompDef.ConsensusSpec != nil {
			strategy = &clusterCompDef.ConsensusSpec.UpdateStrategy
		}
	case appsv1alpha1.Replication:
		// be compatible with the behaviour of RSM in 0.7, set SerialStrategy for Replication workloads by default.
		serialStrategy := appsv1alpha1.SerialStrategy
		strategy = &serialStrategy
	// be compatible with the behaviour of RSM in 0.7, don't set update strategy for Stateful and Stateless workloads.
	case appsv1alpha1.Stateful:
		// do nothing
	case appsv1alpha1.Stateless:
		// do nothing
	default:
		panic(fmt.Sprintf("unknown workload type: %s", clusterCompDef.WorkloadType))
	}
	return strategy, nil
}

// compDefRolesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Roles.
type compDefRolesConvertor struct{}

func (c *compDefRolesConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)

	// if rsm spec is not nil, convert rsm role first.
	if clusterCompDef.RSMSpec != nil {
		return c.convertInstanceSetRole(clusterCompDef)
	}

	switch clusterCompDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return c.convertConsensusRole(clusterCompDef)
	case appsv1alpha1.Replication:
		defaultRoles := []appsv1alpha1.ReplicaRole{
			{
				Name:        constant.Primary,
				Serviceable: true,
				Writable:    true,
				Votable:     true,
			},
			{
				Name:        constant.Secondary,
				Serviceable: true,
				Writable:    false,
				Votable:     true,
			},
		}
		return defaultRoles, nil
	case appsv1alpha1.Stateful:
		return nil, nil
	case appsv1alpha1.Stateless:
		return nil, nil
	default:
		panic(fmt.Sprintf("unknown workload type: %s", clusterCompDef.WorkloadType))
	}
}

func (c *compDefRolesConvertor) convertInstanceSetRole(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) (any, error) {
	if clusterCompDef.RSMSpec == nil {
		return nil, nil
	}

	roles := make([]appsv1alpha1.ReplicaRole, 0)
	for _, workloadRole := range clusterCompDef.RSMSpec.Roles {
		roles = append(roles, appsv1alpha1.ReplicaRole{
			Name:        workloadRole.Name,
			Serviceable: workloadRole.AccessMode != workloads.NoneMode,
			Writable:    workloadRole.AccessMode == workloads.ReadWriteMode,
			Votable:     workloadRole.CanVote,
		})
	}

	return roles, nil
}

func (c *compDefRolesConvertor) convertConsensusRole(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) (any, error) {
	if clusterCompDef.ConsensusSpec == nil {
		return nil, nil
	}

	roles := make([]appsv1alpha1.ReplicaRole, 0)
	addRole := func(member appsv1alpha1.ConsensusMember, votable bool) {
		roles = append(roles, appsv1alpha1.ReplicaRole{
			Name:        member.Name,
			Serviceable: member.AccessMode != appsv1alpha1.None,
			Writable:    member.AccessMode == appsv1alpha1.ReadWrite,
			Votable:     votable,
		})
	}

	addRole(clusterCompDef.ConsensusSpec.Leader, true)
	for _, follower := range clusterCompDef.ConsensusSpec.Followers {
		addRole(follower, true)
	}
	if clusterCompDef.ConsensusSpec.Learner != nil {
		addRole(*clusterCompDef.ConsensusSpec.Learner, false)
	}

	return roles, nil
}

// compDefServiceRefDeclarationsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ServiceRefDeclarations.
type compDefServiceRefDeclarationsConvertor struct{}

func (c *compDefServiceRefDeclarationsConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefLifecycleActionsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.LifecycleActions.
type compDefLifecycleActionsConvertor struct{}

func (c *compDefLifecycleActionsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)

	lifecycleActions := &appsv1alpha1.ComponentLifecycleActions{}

	// RoleProbe can be defined in RSMSpec or ClusterComponentDefinition.Probes.
	if (clusterCompDef.RSMSpec != nil && clusterCompDef.RSMSpec.RoleProbe != nil) || (clusterCompDef.Probes != nil && clusterCompDef.Probes.RoleProbe != nil) {
		lifecycleActions.RoleProbe = c.convertRoleProbe(clusterCompDef)
	}

	if clusterCompDef.SwitchoverSpec != nil {
		lifecycleActions.Switchover = c.convertSwitchover(clusterCompDef.SwitchoverSpec)
	}

	lifecycleActions.PreTerminate = nil
	lifecycleActions.MemberJoin = nil
	lifecycleActions.MemberLeave = nil
	lifecycleActions.Readonly = nil
	lifecycleActions.Readwrite = nil
	lifecycleActions.DataDump = nil
	lifecycleActions.DataLoad = nil
	lifecycleActions.Reconfigure = nil
	lifecycleActions.AccountProvision = nil

	return lifecycleActions, nil // TODO
}

func (c *compDefLifecycleActionsConvertor) convertRoleProbe(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) *appsv1alpha1.Probe {
	builtinHandler := appsv1alpha1.UnknownBuiltinActionHandler
	// if RSMSpec has role probe CustomHandler, use it first.
	if clusterCompDef.RSMSpec != nil && clusterCompDef.RSMSpec.RoleProbe != nil && len(clusterCompDef.RSMSpec.RoleProbe.CustomHandler) > 0 {
		// TODO(xingran): RSMSpec.RoleProbe.CustomHandler support multiple images and commands, but ComponentDefinition.LifeCycleAction.RoleProbe only support one image and command now.
		return &appsv1alpha1.Probe{
			BuiltinHandler: &builtinHandler,
			Action: appsv1alpha1.Action{
				Exec: &appsv1alpha1.ExecAction{
					Image:   clusterCompDef.RSMSpec.RoleProbe.CustomHandler[0].Image,
					Command: clusterCompDef.RSMSpec.RoleProbe.CustomHandler[0].Command,
					Args:    clusterCompDef.RSMSpec.RoleProbe.CustomHandler[0].Args,
				},
			},
		}
	}

	if clusterCompDef == nil || clusterCompDef.Probes == nil || clusterCompDef.Probes.RoleProbe == nil {
		return nil
	}

	clusterCompDefRoleProbe := clusterCompDef.Probes.RoleProbe
	roleProbe := &appsv1alpha1.Probe{
		Action: appsv1alpha1.Action{
			TimeoutSeconds: clusterCompDefRoleProbe.TimeoutSeconds,
		},
		PeriodSeconds: clusterCompDefRoleProbe.PeriodSeconds,
	}

	roleProbe.BuiltinHandler = &builtinHandler
	if clusterCompDefRoleProbe.Commands == nil || len(clusterCompDefRoleProbe.Commands.Queries) == 0 {
		return roleProbe
	}

	commands := clusterCompDefRoleProbe.Commands.Writes
	if len(clusterCompDefRoleProbe.Commands.Writes) == 0 {
		commands = clusterCompDefRoleProbe.Commands.Queries
	}
	roleProbe.Action.Exec = &appsv1alpha1.ExecAction{
		Command: commands,
	}
	return roleProbe
}

func (c *compDefLifecycleActionsConvertor) convertSwitchover(switchover *appsv1alpha1.SwitchoverSpec) *appsv1alpha1.ComponentSwitchover {
	spec := *switchover
	if spec.WithCandidate == nil && spec.WithoutCandidate == nil {
		return nil
	}

	var (
		withCandidateAction    *appsv1alpha1.Action
		withoutCandidateAction *appsv1alpha1.Action
	)
	if spec.WithCandidate != nil && spec.WithCandidate.CmdExecutorConfig != nil {
		withCandidateAction = &appsv1alpha1.Action{
			Exec: &appsv1alpha1.ExecAction{
				Image:   spec.WithCandidate.CmdExecutorConfig.Image,
				Command: spec.WithCandidate.CmdExecutorConfig.Command,
				Args:    spec.WithCandidate.CmdExecutorConfig.Args,
				Env:     spec.WithCandidate.CmdExecutorConfig.Env,
			},
		}
	}
	if spec.WithoutCandidate != nil && spec.WithoutCandidate.CmdExecutorConfig != nil {
		withoutCandidateAction = &appsv1alpha1.Action{
			Exec: &appsv1alpha1.ExecAction{
				Image:   spec.WithoutCandidate.CmdExecutorConfig.Image,
				Command: spec.WithoutCandidate.CmdExecutorConfig.Command,
				Args:    spec.WithoutCandidate.CmdExecutorConfig.Args,
				Env:     spec.WithoutCandidate.CmdExecutorConfig.Env,
			},
		}
	}

	mergeScriptSpec := func() []appsv1alpha1.ScriptSpecSelector {
		if len(spec.WithCandidate.ScriptSpecSelectors) == 0 && len(spec.WithoutCandidate.ScriptSpecSelectors) == 0 {
			return nil
		}

		mergeScriptSpecMap := map[appsv1alpha1.ScriptSpecSelector]bool{}
		for _, val := range append(spec.WithCandidate.ScriptSpecSelectors, spec.WithoutCandidate.ScriptSpecSelectors...) {
			mergeScriptSpecMap[val] = true
		}

		scriptSpecList := make([]appsv1alpha1.ScriptSpecSelector, 0, len(mergeScriptSpecMap))
		for key := range mergeScriptSpecMap {
			scriptSpecList = append(scriptSpecList, key)
		}
		return scriptSpecList
	}

	return &appsv1alpha1.ComponentSwitchover{
		WithCandidate:       withCandidateAction,
		WithoutCandidate:    withoutCandidateAction,
		ScriptSpecSelectors: mergeScriptSpec(),
	}
}

type compDefExporterConvertor struct{}

func (c *compDefExporterConvertor) convert(args ...any) (any, error) {
	return nil, nil
}
