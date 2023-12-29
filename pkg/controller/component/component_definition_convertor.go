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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

// TODO(component): type check

// buildComponentDefinitionByConversion builds a ComponentDefinition from a ClusterComponentDefinition and a ClusterComponentVersion.
func buildComponentDefinitionByConversion(clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompVer *appsv1alpha1.ClusterComponentVersion) (*appsv1alpha1.ComponentDefinition, error) {
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
		"services":               &compDefServicesConvertor{},
		"configs":                &compDefConfigsConvertor{},
		"logconfigs":             &compDefLogConfigsConvertor{},
		"monitor":                &compDefMonitorConvertor{},
		"scripts":                &compDefScriptsConvertor{},
		"policyrules":            &compDefPolicyRulesConvertor{},
		"labels":                 &compDefLabelsConvertor{},
		"replicasLimit":          &compDefReplicasLimitConvertor{},
		"systemaccounts":         &compDefSystemAccountsConvertor{},
		"updatestrategy":         &compDefUpdateStrategyConvertor{},
		"roles":                  &compDefRolesConvertor{},
		"rolearbitrator":         &compDefRoleArbitratorConvertor{},
		"lifecycleactions":       &compDefLifecycleActionsConvertor{},
		"servicerefdeclarations": &compDefServiceRefDeclarationsConvertor{},
	}
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := covertObject(convertors, &compDef.Spec, clusterCompDef, clusterCompVer); err != nil {
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
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.Description, nil
}

// compDefServiceKindConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ServiceKind.
type compDefServiceKindConvertor struct{}

func (c *compDefServiceKindConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.CharacterType, nil
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

type compDefVarsConvertor struct{}

func (c *compDefVarsConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefVolumesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Volumes.
type compDefVolumesConvertor struct{}

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

// compDefServicesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Services.
type compDefServicesConvertor struct{}

func (c *compDefServicesConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	if clusterCompDef.Service == nil {
		return nil, nil
	}

	svc := builder.NewServiceBuilder("", "").
		SetType(corev1.ServiceTypeClusterIP).
		AddPorts(clusterCompDef.Service.ToSVCSpec().Ports...).
		GetObject()

	headlessSvcBuilder := builder.NewHeadlessServiceBuilder("", "").
		AddPorts(clusterCompDef.Service.ToSVCSpec().Ports...)
	if clusterCompDef.PodSpec != nil {
		for _, container := range clusterCompDef.PodSpec.Containers {
			headlessSvcBuilder = headlessSvcBuilder.AddContainerPorts(container.Ports...)
		}
	}
	headlessSvc := c.removeDuplicatePorts(headlessSvcBuilder.GetObject())

	services := []appsv1alpha1.ComponentService{
		{
			Service: appsv1alpha1.Service{
				Name:         "default",
				ServiceName:  "",
				Spec:         svc.Spec,
				RoleSelector: c.roleSelector(clusterCompDef),
			},
		},
		{
			Service: appsv1alpha1.Service{
				Name:         "headless",
				ServiceName:  "headless",
				Spec:         headlessSvc.Spec,
				RoleSelector: c.roleSelector(clusterCompDef),
			},
		},
	}
	return services, nil
}

func (c *compDefServicesConvertor) removeDuplicatePorts(svc *corev1.Service) *corev1.Service {
	ports := make(map[int32]bool)
	servicePorts := make([]corev1.ServicePort, 0)
	for _, port := range svc.Spec.Ports {
		if !ports[port.Port] {
			ports[port.Port] = true
			servicePorts = append(servicePorts, port)
		}
	}
	svc.Spec.Ports = servicePorts
	return svc
}

func (c *compDefServicesConvertor) roleSelector(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) string {
	switch clusterCompDef.WorkloadType {
	case appsv1alpha1.Consensus:
		if clusterCompDef.ConsensusSpec == nil {
			return constant.Leader
		}
		return clusterCompDef.ConsensusSpec.Leader.Name
	case appsv1alpha1.Replication:
		return constant.Primary
	default:
		return ""
	}
}

// compDefConfigsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Configs.
type compDefConfigsConvertor struct{}

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

// compDefLogConfigsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.LogConfigs.
type compDefLogConfigsConvertor struct{}

func (c *compDefLogConfigsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.LogConfigs, nil
}

// compDefMonitorConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Monitor.
type compDefMonitorConvertor struct{}

func (c *compDefMonitorConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.Monitor, nil
}

// compDefScriptsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Scripts.
type compDefScriptsConvertor struct{}

func (c *compDefScriptsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.ScriptSpecs, nil
}

// compDefPolicyRulesConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.PolicyRules.
type compDefPolicyRulesConvertor struct{}

func (c *compDefPolicyRulesConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefLabelsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.Labels.
type compDefLabelsConvertor struct{}

func (c *compDefLabelsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	if clusterCompDef.CustomLabelSpecs == nil {
		return nil, nil
	}

	labels := make(map[string]string, 0)
	for _, customLabel := range clusterCompDef.CustomLabelSpecs {
		labels[customLabel.Key] = customLabel.Value
	}
	return labels, nil
}

type compDefReplicasLimitConvertor struct{}

func (c *compDefReplicasLimitConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// compDefSystemAccountsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.SystemAccounts.
type compDefSystemAccountsConvertor struct{}

func (c *compDefSystemAccountsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	if clusterCompDef.SystemAccounts == nil {
		return nil, nil
	}

	accounts := make([]appsv1alpha1.SystemAccount, 0)
	for _, account := range clusterCompDef.SystemAccounts.Accounts {
		accounts = append(accounts, appsv1alpha1.SystemAccount{
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

// compDefUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.UpdateStrategy.
type compDefUpdateStrategyConvertor struct{}

func (c *compDefUpdateStrategyConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	defaultUpdateStrategy := appsv1alpha1.SerialStrategy
	strategy := &defaultUpdateStrategy

	switch clusterCompDef.WorkloadType {
	case appsv1alpha1.Consensus:
		if clusterCompDef.ConsensusSpec != nil {
			strategy = &clusterCompDef.ConsensusSpec.UpdateStrategy
		}
	case appsv1alpha1.Replication:
		if clusterCompDef.ReplicationSpec != nil {
			strategy = &clusterCompDef.ReplicationSpec.UpdateStrategy
		}
	case appsv1alpha1.Stateful:
		if clusterCompDef.StatefulSpec != nil {
			strategy = &clusterCompDef.StatefulSpec.UpdateStrategy
		}
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
		return c.convertRsmRole(clusterCompDef)
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
				Serviceable: false,
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

func (c *compDefRolesConvertor) convertRsmRole(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) (any, error) {
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

// compDefRoleArbitratorConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.RoleArbitrator.
type compDefRoleArbitratorConvertor struct{}

func (c *compDefRoleArbitratorConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)

	// TODO(xingran): it is hacky, should be refactored
	if clusterCompDef.WorkloadType == appsv1alpha1.Replication && clusterCompDef.CharacterType == constant.RedisCharacterType {
		roleArbitrator := appsv1alpha1.LorryRoleArbitrator
		return &roleArbitrator, nil
	}

	return nil, nil
}

// compDefServiceRefDeclarationsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.ServiceRefDeclarations.
type compDefServiceRefDeclarationsConvertor struct{}

func (c *compDefServiceRefDeclarationsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.ServiceRefDeclarations, nil
}

// compDefLifecycleActionsConvertor is an implementation of the convertor interface, used to convert the given object into ComponentDefinition.Spec.LifecycleActions.
type compDefLifecycleActionsConvertor struct{}

func (c *compDefLifecycleActionsConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	var clusterCompVer *appsv1alpha1.ClusterComponentVersion
	if len(args) > 1 {
		clusterCompVer = args[1].(*appsv1alpha1.ClusterComponentVersion)
	}

	lifecycleActions := &appsv1alpha1.ComponentLifecycleActions{}

	// RoleProbe can be defined in RSMSpec or ClusterComponentDefinition.Probes.
	if (clusterCompDef.RSMSpec != nil && clusterCompDef.RSMSpec.RoleProbe != nil) || (clusterCompDef.Probes != nil && clusterCompDef.Probes.RoleProbe != nil) {
		lifecycleActions.RoleProbe = c.convertRoleProbe(clusterCompDef)
	}

	if clusterCompDef.SwitchoverSpec != nil {
		lifecycleActions.Switchover = c.convertSwitchover(clusterCompDef.SwitchoverSpec, clusterCompVer)
	}

	if clusterCompDef.PostStartSpec != nil {
		lifecycleActions.PostProvision = c.convertPostProvision(clusterCompDef.PostStartSpec)
	}

	lifecycleActions.PreTerminate = nil
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

func (c *compDefLifecycleActionsConvertor) convertBuiltinActionHandler(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) appsv1alpha1.BuiltinActionHandlerType {
	if clusterCompDef == nil || clusterCompDef.CharacterType == "" {
		return appsv1alpha1.UnknownBuiltinActionHandler
	}
	switch clusterCompDef.CharacterType {
	case constant.MySQLCharacterType:
		if clusterCompDef.WorkloadType == appsv1alpha1.Consensus {
			return appsv1alpha1.WeSQLBuiltinActionHandler
		} else {
			return appsv1alpha1.MySQLBuiltinActionHandler
		}
	case constant.PostgreSQLCharacterType:
		if clusterCompDef.WorkloadType == appsv1alpha1.Consensus {
			return appsv1alpha1.ApeCloudPostgresqlBuiltinActionHandler
		} else {
			return appsv1alpha1.PostgresqlBuiltinActionHandler
		}
	case constant.RedisCharacterType:
		return appsv1alpha1.RedisBuiltinActionHandler
	case constant.MongoDBCharacterType:
		return appsv1alpha1.MongoDBBuiltinActionHandler
	case constant.ETCDCharacterType:
		return appsv1alpha1.ETCDBuiltinActionHandler
	case constant.PolarDBXCharacterType:
		return appsv1alpha1.PolarDBXBuiltinActionHandler
	default:
		return appsv1alpha1.UnknownBuiltinActionHandler
	}
}

func (c *compDefLifecycleActionsConvertor) convertRoleProbe(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) *appsv1alpha1.RoleProbe {
	// if RSMSpec has role probe CustomHandler, use it first.
	if clusterCompDef.RSMSpec != nil && clusterCompDef.RSMSpec.RoleProbe != nil && len(clusterCompDef.RSMSpec.RoleProbe.CustomHandler) > 0 {
		// TODO(xingran): RSMSpec.RoleProbe.CustomHandler support multiple images and commands, but ComponentDefinition.LifeCycleAction.RoleProbe only support one image and command now.
		return &appsv1alpha1.RoleProbe{
			LifecycleActionHandler: appsv1alpha1.LifecycleActionHandler{
				BuiltinHandler: nil,
				CustomHandler: &appsv1alpha1.Action{
					Image: clusterCompDef.RSMSpec.RoleProbe.CustomHandler[0].Image,
					Exec: &appsv1alpha1.ExecAction{
						Command: clusterCompDef.RSMSpec.RoleProbe.CustomHandler[0].Command,
					},
				},
			},
		}
	}

	if clusterCompDef == nil || clusterCompDef.Probes == nil || clusterCompDef.Probes.RoleProbe == nil {
		return nil
	}

	clusterCompDefRoleProbe := clusterCompDef.Probes.RoleProbe
	roleProbe := &appsv1alpha1.RoleProbe{
		TimeoutSeconds:   clusterCompDefRoleProbe.TimeoutSeconds,
		PeriodSeconds:    clusterCompDefRoleProbe.PeriodSeconds,
		SuccessThreshold: 1, // default non-zero value
		FailureThreshold: clusterCompDefRoleProbe.FailureThreshold,
	}

	if clusterCompDefRoleProbe.Commands == nil || len(clusterCompDefRoleProbe.Commands.Writes) == 0 || len(clusterCompDefRoleProbe.Commands.Queries) == 0 {
		builtinHandler := c.convertBuiltinActionHandler(clusterCompDef)
		roleProbe.BuiltinHandler = &builtinHandler
		roleProbe.CustomHandler = nil
		return roleProbe
	}

	commands := clusterCompDefRoleProbe.Commands.Writes
	if len(clusterCompDefRoleProbe.Commands.Writes) == 0 {
		commands = clusterCompDefRoleProbe.Commands.Queries
	}
	roleProbe.BuiltinHandler = nil
	roleProbe.CustomHandler = &appsv1alpha1.Action{
		Exec: &appsv1alpha1.ExecAction{
			Command: commands,
		},
	}
	return roleProbe
}

func (c *compDefLifecycleActionsConvertor) convertPostProvision(postStart *appsv1alpha1.PostStartAction) *appsv1alpha1.LifecycleActionHandler {
	if postStart == nil {
		return nil
	}

	defaultPreCondition := appsv1alpha1.ComponentReadyPreConditionType
	return &appsv1alpha1.LifecycleActionHandler{
		CustomHandler: &appsv1alpha1.Action{
			Image: postStart.CmdExecutorConfig.Image,
			Exec: &appsv1alpha1.ExecAction{
				Command: postStart.CmdExecutorConfig.Command,
				Args:    postStart.CmdExecutorConfig.Args,
			},
			Env:          postStart.CmdExecutorConfig.Env,
			PreCondition: &defaultPreCondition,
		},
	}
}

func (c *compDefLifecycleActionsConvertor) convertSwitchover(switchover *appsv1alpha1.SwitchoverSpec,
	clusterCompVer *appsv1alpha1.ClusterComponentVersion) *appsv1alpha1.ComponentSwitchover {
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

	return &appsv1alpha1.ComponentSwitchover{
		WithCandidate:       withCandidateAction,
		WithoutCandidate:    withoutCandidateAction,
		ScriptSpecSelectors: mergeScriptSpec(),
	}
}
