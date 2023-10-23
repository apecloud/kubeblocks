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
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

// TODO(component): type check

func BuildComponentDefinitionFrom(clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompVer *appsv1alpha1.ClusterComponentVersion,
	clusterName string) (*appsv1alpha1.ComponentDefinition, error) {
	if clusterCompDef == nil {
		return nil, nil
	}
	convertors := map[string]convertor{
		"provider":               &providerConvertor{},
		"description":            &descriptionConvertor{},
		"servicekind":            &serviceKindConvertor{},
		"serviceversion":         &serviceVersionConvertor{},
		"runtime":                &runtimeConvertor{},
		"volumes":                &volumeConvertor{},
		"services":               &serviceConvertor{},
		"configs":                &configConvertor{},
		"logconfigs":             &logConfigConvertor{},
		"monitor":                &monitorConvertor{},
		"scripts":                &scriptConvertor{},
		"policyrules":            &policyRuleConvertor{},
		"labels":                 &labelConvertor{},
		"systemaccounts":         &systemAccountConvertor{},
		"connectioncredentials":  &connectionCredentialConvertor{},
		"updatestrategy":         &updateStrategyConvertor{},
		"roles":                  &roleConvertor{},
		"rolearbitrator":         &roleArbitratorConvertor{},
		"lifecycleactions":       &lifecycleActionConvertor{},
		"servicerefdeclarations": &serviceRefDeclarationConvertor{},
	}
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := covertObject(convertors, &compDef.Spec, clusterCompDef, clusterCompVer, clusterName); err != nil {
		return nil, err
	}
	return compDef, nil
}

func BuildComponentFrom(clusterCompDef *appsv1alpha1.ClusterComponentDefinition,
	clusterCompVer *appsv1alpha1.ClusterComponentVersion,
	clusterCompSpec *appsv1alpha1.ClusterComponentSpec) (*appsv1alpha1.Component, error) {
	if clusterCompDef == nil || clusterCompSpec == nil {
		return nil, nil
	}
	convertors := map[string]convertor{
		"cluster":              &clusterConvertor{},
		"compdef":              &compDefConvertor{},
		"classdefref":          &classDefRefConvertor{},
		"servicerefs":          &serviceRefConvertor{},
		"resources":            &resourceConvertor{},
		"volumeclaimtemplates": &volumeClaimTemplateConvertor{},
		"replicas":             &replicaConvertor{},
		"configs":              &configConvertor2{},
		"monitor":              &monitorConvertor2{},
		"enabledlogs":          &enabledLogConvertor{},
		"updatestrategy":       &updateStrategyConvertor2{},
		"serviceaccountname":   &serviceAccountNameConvertor{},
		"affinity":             &affinityConvertor{},
		"tolerations":          &tolerationConvertor{},
		"tls":                  &tlsConvertor{},
		"issuer":               &issuerConvertor{},
	}
	comp := &appsv1alpha1.Component{}
	if err := covertObject(convertors, &comp.Spec, clusterCompDef, clusterCompVer, clusterCompSpec); err != nil {
		return nil, err
	}
	return comp, nil
}

func covertObject(convertors map[string]convertor, obj any, args ...any) error {
	tp := typeofObject(obj)
	for i := 0; i < tp.NumField(); i++ {
		fieldName := tp.Field(i).Name
		c, ok := convertors[strings.ToLower(fieldName)]
		if !ok || c == nil {
			continue // leave the origin (default) value
		}
		val, err := c.convert(args...)
		if err != nil {
			return err
		}
		fieldValue := reflect.ValueOf(obj).Elem().FieldByName(fieldName)
		if fieldValue.IsValid() && fieldValue.Type().AssignableTo(reflect.TypeOf(val)) {
			fieldValue.Set(reflect.ValueOf(val))
		} else {
			panic("not assignable")
		}
	}
	return nil
}

func typeofObject(obj any) reflect.Type {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		panic("not a struct")
	}
	return reflect.TypeOf(val)
}

type convertor interface {
	convert(...any) (any, error)
}

type providerConvertor struct{}

func (c *providerConvertor) convert(args ...any) (any, error) {
	return "", nil
}

type descriptionConvertor struct{}

func (c *descriptionConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.Description, nil
}

type serviceKindConvertor struct{}

func (c *serviceKindConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.CharacterType, nil
}

type serviceVersionConvertor struct{}

func (c *serviceVersionConvertor) convert(args ...any) (any, error) {
	return "", nil
}

type runtimeConvertor struct{}

func (c *runtimeConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	clusterCompVer := args[1].(*appsv1alpha1.ClusterComponentVersion)
	if clusterCompDef.PodSpec == nil {
		return nil, fmt.Errorf("no pod spec")
	}

	podSpec := *clusterCompDef.PodSpec
	if clusterCompVer != nil {
		for _, container := range clusterCompVer.VersionsCtx.InitContainers {
			podSpec.InitContainers = appendOrOverrideContainerAttr(podSpec.InitContainers, container)
		}
		for _, container := range clusterCompVer.VersionsCtx.Containers {
			podSpec.Containers = appendOrOverrideContainerAttr(podSpec.Containers, container)
		}
	}
	return podSpec, nil
}

type volumeConvertor struct{}

func (c *volumeConvertor) convert(args ...any) (any, error) {
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

type serviceConvertor struct{}

func (c *serviceConvertor) convert(args ...any) (any, error) {
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
			RoleSelector: []string{}, // TODO(component): service selector
		},
		{
			Name:         "default-headless",
			ServiceName:  appsv1alpha1.BuiltInString(headlessSvc.Name),
			ServiceSpec:  headlessSvc.Spec,
			RoleSelector: []string{}, // TODO(component): service selector
		},
	}
	return services, nil
}

type configConvertor struct{}

func (c *configConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	clusterCompVer := args[1].(*appsv1alpha1.ClusterComponentVersion)
	if clusterCompVer == nil {
		return clusterCompDef.ConfigSpecs, nil
	} else {
		return cfgcore.MergeConfigTemplates(clusterCompVer.ConfigSpecs, clusterCompDef.ConfigSpecs), nil
	}
}

type logConfigConvertor struct{}

func (c *logConfigConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.LogConfigs, nil
}

type monitorConvertor struct{}

func (c *monitorConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.Monitor, nil
}

type scriptConvertor struct{}

func (c *scriptConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.ScriptSpecs, nil
}

type policyRuleConvertor struct{}

func (c *policyRuleConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

type labelConvertor struct{}

func (c *labelConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	if clusterCompDef.CustomLabelSpecs == nil {
		return nil, nil
	}

	labels := make(map[string]appsv1alpha1.BuiltInString, 0)
	// TODO: clusterCompDef.CustomLabelSpecs -> labels
	return labels, nil
}

type systemAccountConvertor struct{}

func (c *systemAccountConvertor) convert(args ...any) (any, error) {
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

type connectionCredentialConvertor struct{}

func (c *connectionCredentialConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

type updateStrategyConvertor struct{}

func (c *updateStrategyConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	switch clusterCompDef.WorkloadType {
	case appsv1alpha1.Consensus:
		if clusterCompDef.ConsensusSpec == nil {
			return nil, nil
		}
		return &clusterCompDef.ConsensusSpec.UpdateStrategy, nil
	case appsv1alpha1.Replication:
		if clusterCompDef.ReplicationSpec == nil {
			return nil, nil
		}
		return &clusterCompDef.ReplicationSpec.UpdateStrategy, nil
	case appsv1alpha1.Stateful:
		if clusterCompDef.StatefulSpec == nil {
			return nil, nil
		}
		return &clusterCompDef.StatefulSpec.UpdateStrategy, nil
	case appsv1alpha1.Stateless:
		if clusterCompDef.StatelessSpec == nil {
			return nil, nil
		}
		// TODO: check the UpdateStrategy
		return &clusterCompDef.StatelessSpec.UpdateStrategy.Type, nil
	default:
		panic(fmt.Sprintf("unknown workload type: %s", clusterCompDef.WorkloadType))
	}
}

type roleConvertor struct{}

func (c *roleConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	switch clusterCompDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return c.convertConsensusRole(clusterCompDef)
	case appsv1alpha1.Replication:
		return nil, nil
	case appsv1alpha1.Stateful:
		return nil, nil
	case appsv1alpha1.Stateless:
		return nil, nil
	default:
		panic(fmt.Sprintf("unknown workload type: %s", clusterCompDef.WorkloadType))
	}
}

func (c *roleConvertor) convertConsensusRole(clusterCompDef *appsv1alpha1.ClusterComponentDefinition) (any, error) {
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

type roleArbitratorConvertor struct{}

func (c *roleArbitratorConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

type lifecycleActionConvertor struct{}

func (c *lifecycleActionConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	clusterCompVer := args[1].(*appsv1alpha1.ClusterComponentVersion)

	lifecycleActions := &appsv1alpha1.ComponentLifecycleActions{}

	if clusterCompDef.Probes != nil && clusterCompDef.Probes.RoleProbe != nil {
		lifecycleActions.RoleProbe = c.convertRoleProbe(clusterCompDef.Probes.RoleProbe)
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

func (c *lifecycleActionConvertor) convertRoleProbe(probe *appsv1alpha1.ClusterDefinitionProbe) *corev1.Probe {
	if probe.Commands == nil || len(probe.Commands.Writes) == 0 || len(probe.Commands.Queries) == 0 {
		return nil
	}
	commands := probe.Commands.Writes
	if len(probe.Commands.Writes) == 0 {
		commands = probe.Commands.Queries
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: commands,
			},
		},
		TimeoutSeconds:   probe.TimeoutSeconds,
		PeriodSeconds:    probe.PeriodSeconds,
		FailureThreshold: probe.FailureThreshold,
	}
}

func (c *lifecycleActionConvertor) convertSwitchover(switchover *appsv1alpha1.SwitchoverSpec,
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

type serviceRefDeclarationConvertor struct{}

func (c *serviceRefDeclarationConvertor) convert(args ...any) (any, error) {
	clusterCompDef := args[0].(*appsv1alpha1.ClusterComponentDefinition)
	return clusterCompDef.ServiceRefDeclarations, nil
}

// TODO(component)
// func parseComponentConvertorArgs(args ...any) (*appsv1alpha1.ClusterComponentDefinition,
//	*appsv1alpha1.ClusterComponentVersion, *appsv1alpha1.ClusterComponentSpec) {
//	def := args[0].(*appsv1alpha1.ClusterComponentDefinition)
//	ver := args[1].(*appsv1alpha1.ClusterComponentVersion)
//	spec := args[2].(*appsv1alpha1.ClusterComponentSpec)
//	return def, ver, spec
// }

type clusterConvertor struct{}

func (c *clusterConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type compDefConvertor struct{}

func (c *compDefConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type classDefRefConvertor struct{}

func (c *classDefRefConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type serviceRefConvertor struct{}

func (c *serviceRefConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type resourceConvertor struct{}

func (c *resourceConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type volumeClaimTemplateConvertor struct{}

func (c *volumeClaimTemplateConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type replicaConvertor struct{}

func (c *replicaConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type configConvertor2 struct{}

func (c *configConvertor2) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type monitorConvertor2 struct{}

func (c *monitorConvertor2) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type enabledLogConvertor struct{}

func (c *enabledLogConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type updateStrategyConvertor2 struct{}

func (c *updateStrategyConvertor2) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type serviceAccountNameConvertor struct{}

func (c *serviceAccountNameConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type affinityConvertor struct{}

func (c *affinityConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type tolerationConvertor struct{}

func (c *tolerationConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type tlsConvertor struct{}

func (c *tlsConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}

type issuerConvertor struct{}

func (c *issuerConvertor) convert(args ...any) (any, error) {
	// clusterCompDef, clusterCompVer, clusterCompSpec := parseComponentConvertorArgs(args)
	return "", nil // TODO
}
