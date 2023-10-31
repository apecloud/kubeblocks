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

package component

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

// TODO(component): type check

// buildComponentDefinitionFrom builds a ComponentDefinition from a ClusterComponentDefinition and a ClusterComponentVersion.
func buildComponentDefinitionFrom(clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompVer *appsv1alpha1.ClusterComponentVersion,
	clusterName string) (*appsv1alpha1.ComponentDefinition, error) {
	if clusterCompDef == nil {
		return nil, nil
	}
	convertors := map[string]convertor{
		"provider":               &compDefProviderConvertor{},
		"description":            &compDefDescriptionConvertor{},
		"servicekind":            &compDefServiceKindConvertor{},
		"serviceversion":         &compDefServiceVersionConvertor{},
		"runtime":                &compDefRuntimeConvertor{},
		"volumes":                &compDefVolumesConvertor{},
		"services":               &compDefServicesConvertor{},
		"configs":                &compDefConfigsConvertor{},
		"logconfigs":             &compDefLogConfigsConvertor{},
		"monitor":                &compDefMonitorConvertor{},
		"scripts":                &compDefScriptsConvertor{},
		"policyrules":            &compDefPolicyRulesConvertor{},
		"labels":                 &compDefLabelsConvertor{},
		"systemaccounts":         &compDefSystemAccountsConvertor{},
		"connectioncredentials":  &compDefConnCredentialsConvertor{},
		"updatestrategy":         &compDefUpdateStrategyConvertor{},
		"roles":                  &compDefRolesConvertor{},
		"rolearbitrator":         &compDefRoleArbitratorConvertor{},
		"lifecycleactions":       &compDefLifecycleActionsConvertor{},
		"servicerefdeclarations": &compDefServiceRefDeclarationsConvertor{},
	}
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := covertObject(convertors, &compDef.Spec, clusterCompDef, clusterCompVer, clusterName); err != nil {
		return nil, err
	}
	return compDef, nil
}

// compDefProviderConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Provider.
type compDefProviderConvertor struct{}

// compDefDescriptionConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Description.
type compDefDescriptionConvertor struct{}

// compDefServiceKindConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ServiceKind.
type compDefServiceKindConvertor struct{}

// compDefServiceVersionConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ServiceVersion.
type compDefServiceVersionConvertor struct{}

// compDefRuntimeConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Runtime.
type compDefRuntimeConvertor struct{}

// compDefVolumesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Volumes.
type compDefVolumesConvertor struct{}

// compDefServicesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Services.
type compDefServicesConvertor struct{}

// compDefConfigsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Configs.
type compDefConfigsConvertor struct{}

// compDefLogConfigsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.LogConfigs.
type compDefLogConfigsConvertor struct{}

// compDefConnCredentialsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ConnectionCredentials.
type compDefConnCredentialsConvertor struct{}

// compDefMonitorConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Monitor.
type compDefMonitorConvertor struct{}

// compDefScriptsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Scripts.
type compDefScriptsConvertor struct{}

// compDefPolicyRulesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.PolicyRules.
type compDefPolicyRulesConvertor struct{}

// compDefLabelsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Labels.
type compDefLabelsConvertor struct{}

// compDefUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.UpdateStrategy.
type compDefUpdateStrategyConvertor struct{}

// compDefSystemAccountsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.SystemAccounts.
type compDefSystemAccountsConvertor struct{}

// compDefRolesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Roles.
type compDefRolesConvertor struct{}

// compDefRoleArbitratorConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.RoleArbitrator.
type compDefRoleArbitratorConvertor struct{}

// compDefLifecycleActionsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.LifecycleActions.
type compDefLifecycleActionsConvertor struct{}

// compDefServiceRefDeclarationsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ServiceRefDeclarations.
type compDefServiceRefDeclarationsConvertor struct{}

func (c *compDefProviderConvertor) convert(args ...any) (any, error) {
	return "", nil
}

func (c *compDefDescriptionConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.Description, nil
}

func (c *compDefServiceKindConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.CharacterType, nil
}

func (c *compDefServiceVersionConvertor) convert(args ...any) (any, error) {
	return "", nil
}

func (c *compDefRuntimeConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	if len(args) > 1 {
		clusterCompVer = args[1].(*appsv1alpha1.ClusterComponentVersion)
	}
	if clusterCompDef.PodSpec == nil {
		return nil, fmt.Errorf("no pod spec")
	}

	podSpec := clusterCompDef.PodSpec.DeepCopy()
	if clusterCompVer != nil {
		for _, container := range clusterCompVer.VersionsCtx.InitContainers {
			podSpec.InitContainers = appendOrOverrideContainerAttr(podSpec.InitContainers, container)
		}
		for _, container := range clusterCompVer.VersionsCtx.Containers {
			podSpec.Containers = appendOrOverrideContainerAttr(podSpec.Containers, container)
		}
	}
	return *podSpec, nil
}

func (c *compDefVolumesConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	if clusterCompDef.VolumeTypes == nil {
		return nil, nil
	}

	volumes := make([]appsv1alpha1.ComponentVolume, 0)
	for _, vol := range clusterCompDef.VolumeTypes {
		volumes = append(volumes, appsv1alpha1.ComponentVolume{
			Name: vol.Name,
		})
	}

	if clusterCompDef.VolumeProtectionSpec != nil {
		defaultHighWatermark := clusterCompDef.VolumeProtectionSpec.HighWatermark
		for i := range volumes {
			volumes[i].HighWatermark = defaultHighWatermark
		}
		for _, v := range clusterCompDef.VolumeProtectionSpec.Volumes {
			if v.HighWatermark != nil && *v.HighWatermark != defaultHighWatermark {
				for i, vv := range volumes {
					if v.Name != vv.Name {
						continue
					}
					volumes[i].HighWatermark = *v.HighWatermark
				}
			}
		}
	}
	return volumes, nil
}

func (c *compDefServicesConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	clusterName := args[2].(string)
	if clusterCompDef.Service == nil {
		return nil, nil
	}

	svcName := fmt.Sprintf("%s-%s", clusterName, clusterCompDef.Name)
	svc := builder.NewServiceBuilder("", svcName).
		SetType(corev1.ServiceTypeClusterIP).
		AddPorts(clusterCompDef.Service.ToSVCSpec().Ports...).
		GetObject()

	headlessSvcName := fmt.Sprintf("%s-headless", svcName)
	headlessSvcBuilder := builder.NewHeadlessServiceBuilder("", headlessSvcName).
		AddPorts(clusterCompDef.Service.ToSVCSpec().Ports...)
	if clusterCompDef.PodSpec != nil {
		for _, container := range clusterCompDef.PodSpec.Containers {
			headlessSvcBuilder = headlessSvcBuilder.AddContainerPorts(container.Ports...)
		}
	}
	headlessSvc := headlessSvcBuilder.GetObject()

	services := []appsv1alpha1.ComponentService{
		{
			Name:         "default",
			ServiceName:  appsv1alpha1.BuiltInString(svc.Name),
			ServiceSpec:  svc.Spec,
			RoleSelector: "", // TODO(component): service selector
		},
		{
			Name:         "default-headless",
			ServiceName:  appsv1alpha1.BuiltInString(headlessSvc.Name),
			ServiceSpec:  headlessSvc.Spec,
			RoleSelector: "", // TODO(component): service selector
		},
	}
	return services, nil
}

func (c *compDefConfigsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	if len(args) > 1 {
		clusterCompVer = args[1].(*appsv1alpha1.ClusterComponentVersion)
	}
	if clusterCompVer == nil {
		return clusterCompDef.ConfigSpecs, nil
	}
	return cfgcore.MergeConfigTemplates(clusterCompVer.ConfigSpecs, clusterCompDef.ConfigSpecs), nil
}

func (c *compDefLogConfigsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.LogConfigs, nil
}

func (c *compDefMonitorConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.Monitor, nil
}

func (c *compDefScriptsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.ScriptSpecs, nil
}

func (c *compDefPolicyRulesConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

func (c *compDefLabelsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	if clusterCompDef.CustomLabelSpecs == nil {
		return nil, nil
	}

	labels := make(map[string]appsv1alpha1.BuiltInString, 0)
	for _, customLabel := range clusterCompDef.CustomLabelSpecs {
		labels[customLabel.Key] = appsv1alpha1.BuiltInString(customLabel.Value)
	}
	return labels, nil
}

func (c *compDefSystemAccountsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	if clusterCompDef.SystemAccounts == nil {
		return nil, nil
	}

	accounts := make([]appsv1alpha1.ComponentSystemAccount, 0)
	for _, account := range clusterCompDef.SystemAccounts.Accounts {
		accounts = append(accounts, appsv1alpha1.ComponentSystemAccount{
			Name:                     string(account.Name),
			PasswordGenerationPolicy: clusterCompDef.SystemAccounts.PasswordConfig,
			SecretRef:                account.ProvisionPolicy.SecretRef,
		})
		if account.ProvisionPolicy.Statements != nil {
			accounts[len(accounts)-1].Statement = account.ProvisionPolicy.Statements.CreationStatement
		}
	}
	return accounts, nil
}

func (c *compDefConnCredentialsConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

func (c *compDefUpdateStrategyConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	var compDefUpdateStrategy *appsv1alpha1.UpdateStrategy
	switch clusterCompDef.WorkloadType {
	case appsv1alpha1.Consensus:
		if clusterCompDef.ConsensusSpec != nil {
			updateStrategy := &clusterCompDef.ConsensusSpec.UpdateStrategy
			compDefUpdateStrategy = updateStrategy
		} else {
			updateStrategy := appsv1alpha1.BestEffortParallelStrategy
			compDefUpdateStrategy = &updateStrategy
		}
	case appsv1alpha1.Replication:
		if clusterCompDef.ReplicationSpec != nil {
			updateStrategy := &clusterCompDef.ReplicationSpec.UpdateStrategy
			compDefUpdateStrategy = updateStrategy
		} else {
			updateStrategy := appsv1alpha1.BestEffortParallelStrategy
			compDefUpdateStrategy = &updateStrategy
		}
	case appsv1alpha1.Stateful:
		if clusterCompDef.StatefulSpec != nil {
			updateStrategy := &clusterCompDef.StatefulSpec.UpdateStrategy
			compDefUpdateStrategy = updateStrategy
		} else {
			updateStrategy := appsv1alpha1.BestEffortParallelStrategy
			compDefUpdateStrategy = &updateStrategy
		}
	case appsv1alpha1.Stateless:
		// TODO: check the UpdateStrategy of stateless
		updateStrategy := appsv1alpha1.BestEffortParallelStrategy
		compDefUpdateStrategy = &updateStrategy
	default:
		panic(fmt.Sprintf("unknown workload type: %s", clusterCompDef.WorkloadType))
	}
	return compDefUpdateStrategy, nil
}

func (c *compDefRolesConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	switch clusterCompDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return c.convertConsensusRole(clusterCompDef)
	case appsv1alpha1.Replication:
		defaultRoles := []appsv1alpha1.ComponentReplicaRole{
			{
				Name:        constant.Primary,
				Serviceable: true,
				Writable:    true,
			},
			{
				Name:        constant.Secondary,
				Serviceable: false,
				Writable:    false,
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

func (c *compDefRolesConvertor) convertConsensusRole(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) (any, error) {
	if clusterCompDef.ConsensusSpec == nil {
		return nil, nil
	}

	roles := make([]appsv1alpha1.ComponentReplicaRole, 0)
	addRole := func(member appsv1alpha1.ConsensusMember) {
		roles = append(roles, appsv1alpha1.ComponentReplicaRole{
			Name:        member.Name,
			Serviceable: member.AccessMode != appsv1alpha1.None,
			Writable:    member.AccessMode == appsv1alpha1.ReadWrite,
		})
	}

	addRole(clusterCompDef.ConsensusSpec.Leader)
	for _, follower := range clusterCompDef.ConsensusSpec.Followers {
		addRole(follower)
	}
	if clusterCompDef.ConsensusSpec.Learner != nil {
		addRole(*clusterCompDef.ConsensusSpec.Learner)
	}

	return roles, nil
}

func (c *compDefRoleArbitratorConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

func (c *compDefServiceRefDeclarationsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.ServiceRefDeclarations, nil
}

func (c *compDefLifecycleActionsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	if len(args) > 1 {
		clusterCompVer = args[1].(*appsv1alpha1.ClusterComponentVersion)
	}

	lifecycleActions := &appsv1alpha1.ComponentLifecycleActions{}

	if clusterCompDef.Probes != nil && clusterCompDef.Probes.RoleProbe != nil {
		lifecycleActions.RoleProbe = c.convertRoleProbe(clusterCompDef.Probes.RoleProbe, clusterCompDef.CharacterType)
	}

	if clusterCompDef.SwitchoverSpec != nil {
		lifecycleActions.Switchover = c.convertSwitchover(clusterCompDef.SwitchoverSpec, clusterCompVer)
	}

	lifecycleActions.MemberJoin = nil
	lifecycleActions.MemberLeave = nil
	lifecycleActions.Readonly = nil
	lifecycleActions.Readwrite = nil
	lifecycleActions.DataPopulate = nil
	lifecycleActions.DataAssemble = nil
	lifecycleActions.Reconfigure = nil
	lifecycleActions.AccountProvision = nil

	return lifecycleActions, nil // TODO
}

func (c *compDefLifecycleActionsConvertor) convertRoleProbe(probe *appsv1alpha1.ClusterDefinitionProbe, characterType string) *appsv1alpha1.RoleProbeSpec {
	if probe == nil {
		return nil
	}

	roleProbeSpec := &appsv1alpha1.RoleProbeSpec{
		TimeoutSeconds:   probe.TimeoutSeconds,
		PeriodSeconds:    probe.PeriodSeconds,
		FailureThreshold: probe.FailureThreshold,
	}

	if probe.Commands == nil || len(probe.Commands.Writes) == 0 || len(probe.Commands.Queries) == 0 {
		buildInHandlerName := characterType
		roleProbeSpec.BuiltinHandler = &buildInHandlerName
		roleProbeSpec.CustomHandler = nil
		return roleProbeSpec
	}

	commands := probe.Commands.Writes
	if len(probe.Commands.Writes) == 0 {
		commands = probe.Commands.Queries
	}
	roleProbeSpec.BuiltinHandler = nil
	roleProbeSpec.CustomHandler = &appsv1alpha1.Action{
		Exec: &appsv1alpha1.ExecAction{
			Command: commands,
		},
	}
	return roleProbeSpec
}

func (c *compDefLifecycleActionsConvertor) convertSwitchover(switchover *appsv1alpha1.SwitchoverSpec,
	clusterCompVer *appsv1alpha1.ClusterComponentVersion) *appsv1alpha1.ComponentSwitchoverSpec {
	spec := *switchover
	if clusterCompVer != nil {
		overrideSwitchoverSpecAttr(&spec, clusterCompVer.SwitchoverSpec)
	}
	if spec.WithCandidate == nil && spec.WithoutCandidate == nil {
		return nil
	}

	var (
		withCandidateAction    *appsv1alpha1.Action
		withoutCandidateAction *appsv1alpha1.Action
	)
	if spec.WithCandidate != nil && spec.WithCandidate.CmdExecutorConfig != nil {
		withCandidateAction = &appsv1alpha1.Action{
			Image: spec.WithCandidate.CmdExecutorConfig.Image,
			Exec: &appsv1alpha1.ExecAction{
				Command: spec.WithCandidate.CmdExecutorConfig.Command,
				Args:    spec.WithCandidate.CmdExecutorConfig.Args,
			},
			Env: spec.WithCandidate.CmdExecutorConfig.Env,
		}
	}
	if spec.WithoutCandidate != nil && spec.WithoutCandidate.CmdExecutorConfig != nil {
		withoutCandidateAction = &appsv1alpha1.Action{
			Image: spec.WithoutCandidate.CmdExecutorConfig.Image,
			Exec: &appsv1alpha1.ExecAction{
				Command: spec.WithoutCandidate.CmdExecutorConfig.Command,
				Args:    spec.WithoutCandidate.CmdExecutorConfig.Args,
			},
			Env: spec.WithoutCandidate.CmdExecutorConfig.Env,
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

	return &appsv1alpha1.ComponentSwitchoverSpec{
		WithCandidate:       withCandidateAction,
		WithoutCandidate:    withoutCandidateAction,
		ScriptSpecSelectors: mergeScriptSpec(),
	}
}
