/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package multiversion

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/cmd/helmhook/hook"
	"github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// covert appsv1alpha1.componentdefinition resources to appsv1.componentdefinition

const (
	convertedFromAnnotationKey = "api.kubeblocks.io/converted-from"
)

var (
	cmpdResource = "componentdefinitions"
	cmpdGVR      = appsv1.GroupVersion.WithResource(cmpdResource)
)

func init() {
	hook.RegisterCRDConversion(cmpdGVR, hook.NewNoVersion(1, 0), cmpdHandler(),
		hook.NewNoVersion(0, 8),
		hook.NewNoVersion(0, 9))
}

func cmpdHandler() hook.ConversionHandler {
	return &convertor{
		kind: "ComponentDefinition",
		source: &cmpdConvertor{
			namespaces: []string{"default"}, // TODO: namespaces
		},
		target: &cmpdConvertor{},
	}
}

type cmpdConvertor struct {
	namespaces []string
}

func (c *cmpdConvertor) list(ctx context.Context, cli *versioned.Clientset, _ string) ([]client.Object, error) {
	list, err := cli.AppsV1alpha1().ComponentDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	addons := make([]client.Object, 0)
	for i := range list.Items {
		addons = append(addons, &list.Items[i])
	}
	return addons, nil
}

func (c *cmpdConvertor) used(ctx context.Context, cli *versioned.Clientset, _, name string) (bool, error) {
	selectors := []string{
		fmt.Sprintf("%s=%s", constant.AppManagedByLabelKey, constant.AppName),
		fmt.Sprintf("%s=%s", constant.ComponentDefinitionLabelKey, name),
	}
	opts := metav1.ListOptions{
		LabelSelector: strings.Join(selectors, ","),
	}

	used := false
	for _, namespace := range c.namespaces {
		compList, err := cli.AppsV1alpha1().Components(namespace).List(ctx, opts)
		if err != nil {
			return false, err
		}
		used = used || (len(compList.Items) > 0)
	}
	return used, nil
}

func (c *cmpdConvertor) get(ctx context.Context, cli *versioned.Clientset, _, name string) (client.Object, error) {
	return cli.AppsV1().ComponentDefinitions().Get(ctx, name, metav1.GetOptions{})
}

func (c *cmpdConvertor) convert(source client.Object) []client.Object {
	cmpd := source.(*appsv1alpha1.ComponentDefinition)
	return []client.Object{
		&appsv1.ComponentDefinition{
			Spec: appsv1.ComponentDefinitionSpec{
				Provider:               cmpd.Spec.Provider,
				Description:            cmpd.Spec.Description,
				ServiceKind:            cmpd.Spec.ServiceKind,
				ServiceVersion:         cmpd.Spec.ServiceVersion,
				Runtime:                cmpd.Spec.Runtime,
				Vars:                   c.vars(cmpd.Spec.Vars),
				Volumes:                c.volumes(cmpd.Spec.Volumes),
				HostNetwork:            c.hostNetwork(cmpd.Spec.HostNetwork),
				Services:               componentServices(cmpd.Spec.Services),
				Configs:                c.configs(cmpd.Spec.Configs),
				Scripts:                c.scripts(cmpd.Spec.Scripts),
				MetricExporter:         c.exporter(cmpd.Spec.Exporter),
				PolicyRules:            cmpd.Spec.PolicyRules,
				ReplicasLimit:          c.replicasLimit(cmpd.Spec.ReplicasLimit),
				SystemAccounts:         c.systemAccounts(cmpd.Spec.SystemAccounts),
				UpdateStrategy:         c.updateStrategy(cmpd.Spec.UpdateStrategy),
				PodManagementPolicy:    cmpd.Spec.PodManagementPolicy,
				MinReadySeconds:        cmpd.Spec.MinReadySeconds,
				Roles:                  c.roles(cmpd.Spec.Roles),
				LifecycleActions:       c.lifecycleActions(cmpd.Spec.LifecycleActions),
				ServiceRefDeclarations: c.serviceRefDeclarations(cmpd.Spec.ServiceRefDeclarations),
			},
		},
	}
}

func (c *cmpdConvertor) vars(vars []appsv1alpha1.EnvVar) []appsv1.EnvVar {
	if len(vars) == 0 {
		return nil
	}
	newVars := make([]appsv1.EnvVar, 0)
	for i := range vars {
		newVars = append(newVars, appsv1.EnvVar{
			Name:       vars[i].Name,
			Value:      vars[i].Value,
			ValueFrom:  c.valueFrom(vars[i].ValueFrom),
			Expression: vars[i].Expression,
		})
	}
	return newVars
}

func (c *cmpdConvertor) valueFrom(valueFrom *appsv1alpha1.VarSource) *appsv1.VarSource {
	if valueFrom == nil {
		return nil
	}
	return &appsv1.VarSource{
		ConfigMapKeyRef:   valueFrom.ConfigMapKeyRef,
		SecretKeyRef:      valueFrom.SecretKeyRef,
		HostNetworkVarRef: c.hostNetworkVar(valueFrom.HostNetworkVarRef),
		ServiceVarRef:     c.serviceVar(valueFrom.ServiceVarRef),
		CredentialVarRef:  c.credentialVar(valueFrom.CredentialVarRef),
		ServiceRefVarRef:  c.serviceRefVar(valueFrom.ServiceRefVarRef),
		ComponentVarRef:   c.componentVar(valueFrom.ComponentVarRef),
	}
}

func (c *cmpdConvertor) hostNetworkVar(hostNetwork *appsv1alpha1.HostNetworkVarSelector) *appsv1.HostNetworkVarSelector {
	if hostNetwork == nil {
		return nil
	}
	newHostNetwork := &appsv1.HostNetworkVarSelector{
		ClusterObjectReference: c.clusterObjectReference(hostNetwork.ClusterObjectReference),
		HostNetworkVars: appsv1.HostNetworkVars{
			Container: nil,
		},
	}
	if hostNetwork.Container != nil {
		newHostNetwork.HostNetworkVars.Container = &appsv1.ContainerVars{
			Name: hostNetwork.Container.Name,
			Port: c.namedVar(hostNetwork.Container.Port),
		}
	}
	return newHostNetwork
}

func (c *cmpdConvertor) serviceVar(service *appsv1alpha1.ServiceVarSelector) *appsv1.ServiceVarSelector {
	if service == nil {
		return nil
	}
	return &appsv1.ServiceVarSelector{
		ClusterObjectReference: c.clusterObjectReference(service.ClusterObjectReference),
		ServiceVars: appsv1.ServiceVars{
			Host:         c.varOption(service.Host),
			LoadBalancer: c.varOption(service.LoadBalancer),
			Port:         c.namedVar(service.Port),
		},
	}
}

func (c *cmpdConvertor) credentialVar(credential *appsv1alpha1.CredentialVarSelector) *appsv1.CredentialVarSelector {
	if credential == nil {
		return nil
	}
	return &appsv1.CredentialVarSelector{
		ClusterObjectReference: c.clusterObjectReference(credential.ClusterObjectReference),
		CredentialVars: appsv1.CredentialVars{
			Username: c.varOption(credential.Username),
			Password: c.varOption(credential.Password),
		},
	}
}

func (c *cmpdConvertor) serviceRefVar(serviceRef *appsv1alpha1.ServiceRefVarSelector) *appsv1.ServiceRefVarSelector {
	if serviceRef == nil {
		return nil
	}
	return &appsv1.ServiceRefVarSelector{
		ClusterObjectReference: c.clusterObjectReference(serviceRef.ClusterObjectReference),
		ServiceRefVars: appsv1.ServiceRefVars{
			Endpoint: c.varOption(serviceRef.Endpoint),
			Host:     c.varOption(serviceRef.Host),
			Port:     c.varOption(serviceRef.Port),
			CredentialVars: appsv1.CredentialVars{
				Username: c.varOption(serviceRef.Username),
				Password: c.varOption(serviceRef.Password),
			},
		},
	}
}

func (c *cmpdConvertor) componentVar(comp *appsv1alpha1.ComponentVarSelector) *appsv1.ComponentVarSelector {
	if comp == nil {
		return nil
	}
	return &appsv1.ComponentVarSelector{
		ClusterObjectReference: c.clusterObjectReference(comp.ClusterObjectReference),
		ComponentVars: appsv1.ComponentVars{
			ComponentName: c.varOption(comp.ComponentName),
			Replicas:      c.varOption(comp.Replicas),
			InstanceNames: c.varOption(comp.InstanceNames),
			PodFQDNs:      c.varOption(comp.PodFQDNs),
		},
	}
}

func (c *cmpdConvertor) clusterObjectReference(objRef appsv1alpha1.ClusterObjectReference) appsv1.ClusterObjectReference {
	return appsv1.ClusterObjectReference{
		CompDef:                     objRef.CompDef,
		Name:                        objRef.Name,
		Optional:                    objRef.Optional,
		MultipleClusterObjectOption: c.multipleClusterObjectOption(objRef.MultipleClusterObjectOption),
	}
}

func (c *cmpdConvertor) multipleClusterObjectOption(opt *appsv1alpha1.MultipleClusterObjectOption) *appsv1.MultipleClusterObjectOption {
	if opt == nil {
		return nil
	}
	o := &appsv1.MultipleClusterObjectOption{
		Strategy: appsv1.MultipleClusterObjectStrategy(opt.Strategy),
	}
	if opt.CombinedOption != nil {
		o.CombinedOption = &appsv1.MultipleClusterObjectCombinedOption{
			NewVarSuffix: opt.CombinedOption.NewVarSuffix,
			ValueFormat:  appsv1.MultipleClusterObjectValueFormat(opt.CombinedOption.ValueFormat),
		}
		if opt.CombinedOption.FlattenFormat != nil {
			o.CombinedOption.FlattenFormat = &appsv1.MultipleClusterObjectValueFormatFlatten{
				Delimiter:         opt.CombinedOption.FlattenFormat.Delimiter,
				KeyValueDelimiter: opt.CombinedOption.FlattenFormat.KeyValueDelimiter,
			}
		}
	}
	return o
}

func (c *cmpdConvertor) namedVar(v *appsv1alpha1.NamedVar) *appsv1.NamedVar {
	if v == nil {
		return nil
	}
	return &appsv1.NamedVar{
		Name:   v.Name,
		Option: c.varOption(v.Option),
	}
}

func (c *cmpdConvertor) varOption(opt *appsv1alpha1.VarOption) *appsv1.VarOption {
	if opt == nil {
		return nil
	}
	o := appsv1.VarOption(*opt)
	return &o
}

func (c *cmpdConvertor) volumes(volumes []appsv1alpha1.ComponentVolume) []appsv1.ComponentVolume {
	if len(volumes) == 0 {
		return nil
	}
	newVolumes := make([]appsv1.ComponentVolume, 0)
	for i := range volumes {
		volume := appsv1.ComponentVolume{
			Name:         volumes[i].Name,
			NeedSnapshot: volumes[i].NeedSnapshot,
		}
		if volumes[i].HighWatermark > 0 {
			volume.HighWatermark = &volumes[i].HighWatermark
		}
		newVolumes = append(newVolumes, volume)
	}
	return newVolumes
}

func (c *cmpdConvertor) hostNetwork(hostNetwork *appsv1alpha1.HostNetwork) *appsv1.HostNetwork {
	if hostNetwork == nil {
		return nil
	}
	newHostNetwork := &appsv1.HostNetwork{
		ContainerPorts: make([]appsv1.HostNetworkContainerPort, 0),
	}
	for i := range hostNetwork.ContainerPorts {
		newHostNetwork.ContainerPorts = append(newHostNetwork.ContainerPorts, appsv1.HostNetworkContainerPort{
			Container: hostNetwork.ContainerPorts[i].Container,
			Ports:     hostNetwork.ContainerPorts[i].Ports,
		})
	}
	return newHostNetwork
}

func componentServices(services []appsv1alpha1.ComponentService) []appsv1.ComponentService {
	if len(services) == 0 {
		return nil
	}
	newServices := make([]appsv1.ComponentService, 0)
	for i := range services {
		newServices = append(newServices, appsv1.ComponentService{
			Service: appsv1.Service{
				Name:         services[i].Name,
				ServiceName:  services[i].ServiceName,
				Annotations:  services[i].Annotations,
				Spec:         services[i].Spec,
				RoleSelector: services[i].RoleSelector,
			},
			PodService:           services[i].PodService,
			DisableAutoProvision: services[i].DisableAutoProvision,
		})
	}
	return newServices
}

func (c *cmpdConvertor) configs(configs []appsv1alpha1.ComponentConfigSpec) []appsv1.ComponentConfigTemplate {
	if len(configs) == 0 {
		return nil
	}
	newConfigs := make([]appsv1.ComponentConfigTemplate, 0)
	for i := range configs {
		newConfigs = append(newConfigs, appsv1.ComponentConfigTemplate{
			Name:        configs[i].Name,
			Template:    configs[i].TemplateRef,
			Namespace:   configs[i].Namespace,
			VolumeName:  configs[i].VolumeName,
			DefaultMode: configs[i].DefaultMode,
		})
	}
	return newConfigs
}

func (c *cmpdConvertor) scripts(scripts []appsv1alpha1.ComponentTemplateSpec) []appsv1.ComponentScriptTemplate {
	if len(scripts) == 0 {
		return nil
	}
	newScripts := make([]appsv1.ComponentScriptTemplate, 0)
	for i := range scripts {
		newScripts = append(newScripts, appsv1.ComponentScriptTemplate{
			Name:        scripts[i].Name,
			Template:    scripts[i].TemplateRef,
			Namespace:   scripts[i].Namespace,
			VolumeName:  scripts[i].VolumeName,
			DefaultMode: scripts[i].DefaultMode,
		})
	}
	return newScripts
}

func (c *cmpdConvertor) exporter(exporter *appsv1alpha1.Exporter) *appsv1.MetricExporter {
	if exporter == nil {
		return nil
	}
	return &appsv1.MetricExporter{
		ContainerName: exporter.ContainerName,
		ScrapePath:    exporter.ScrapePath,
		ScrapePort:    exporter.ScrapePort,
		ScrapeScheme:  appsv1.PrometheusScheme(exporter.ScrapeScheme),
	}
}

func (c *cmpdConvertor) replicasLimit(limit *appsv1alpha1.ReplicasLimit) *appsv1.ReplicasLimit {
	if limit == nil {
		return nil
	}
	return &appsv1.ReplicasLimit{
		MinReplicas: limit.MinReplicas,
		MaxReplicas: limit.MaxReplicas,
	}
}

func (c *cmpdConvertor) systemAccounts(accounts []appsv1alpha1.SystemAccount) []appsv1.SystemAccount {
	if len(accounts) == 0 {
		return nil
	}
	newAccounts := make([]appsv1.SystemAccount, 0)
	for i := range accounts {
		account := appsv1.SystemAccount{
			Name:           accounts[i].Name,
			Initialization: accounts[i].InitAccount,
		}
		if len(accounts[i].Statement) > 0 {
			account.Statement = &appsv1.SystemAccountStatement{
				Creation: accounts[i].Statement,
			}
		}
		account.Password = &appsv1.SystemAccountPassword{}
		if accounts[i].SecretRef == nil {
			account.Password.GenerationPolicy = &appsv1.PasswordGenerationPolicy{
				Length:     accounts[i].PasswordGenerationPolicy.Length,
				NumDigits:  accounts[i].PasswordGenerationPolicy.NumDigits,
				NumSymbols: accounts[i].PasswordGenerationPolicy.NumSymbols,
				LetterCase: appsv1.LetterCase(accounts[i].PasswordGenerationPolicy.LetterCase),
			}
			if len(accounts[i].PasswordGenerationPolicy.Seed) > 0 {
				account.Password.GenerationPolicy.Seed = &accounts[i].PasswordGenerationPolicy.Seed
			}
		} else {
			account.Password.SecretRef = &corev1.SecretReference{
				Namespace: accounts[i].SecretRef.Namespace,
				Name:      accounts[i].SecretRef.Name,
			}
		}

		newAccounts = append(newAccounts, account)
	}
	return newAccounts
}

func (c *cmpdConvertor) updateStrategy(strategy *appsv1alpha1.UpdateStrategy) *appsv1.UpdateStrategy {
	if strategy == nil {
		return nil
	}
	s := appsv1.UpdateStrategy(*strategy)
	return &s
}

func (c *cmpdConvertor) roles(roles []appsv1alpha1.ReplicaRole) []appsv1.ReplicaRole {
	if len(roles) == 0 {
		return nil
	}
	newRoles := make([]appsv1.ReplicaRole, 0)
	for i := range roles {
		newRoles = append(newRoles, appsv1.ReplicaRole{
			Name:        roles[i].Name,
			Serviceable: roles[i].Serviceable,
			Writable:    roles[i].Writable,
			Votable:     roles[i].Votable,
		})
	}
	return newRoles
}

func (c *cmpdConvertor) lifecycleActions(actions *appsv1alpha1.ComponentLifecycleActions) *appsv1.ComponentLifecycleActions {
	if actions == nil {
		return nil
	}
	newActions := &appsv1.ComponentLifecycleActions{
		PostProvision: c.lifecycleAction(actions.PostProvision),
		PreTerminate:  c.lifecycleAction(actions.PreTerminate),
		RoleProbe:     c.lifecycleProbe(actions.RoleProbe),
		MemberJoin:    c.lifecycleAction(actions.MemberJoin),
		MemberLeave:   c.lifecycleAction(actions.MemberLeave),
		Readonly:      c.lifecycleAction(actions.Readonly),
		Readwrite:     c.lifecycleAction(actions.Readwrite),
		DataDump:      c.lifecycleAction(actions.DataDump),
		DataLoad:      c.lifecycleAction(actions.DataLoad),
		Reconfigure:   c.lifecycleAction(actions.Reconfigure),
		CreateAccount: c.lifecycleAction(actions.AccountProvision),
	}
	if actions.Switchover != nil {
		newActions.Switchover = c.lifecycleActionCustom(actions.Switchover.WithoutCandidate)
	}
	return newActions
}

func (c *cmpdConvertor) lifecycleAction(handler *appsv1alpha1.LifecycleActionHandler) *appsv1.Action {
	if handler == nil {
		return nil
	}
	if handler.BuiltinHandler != nil {
		return c.lifecycleActionBuiltin(handler.BuiltinHandler)
	}
	return c.lifecycleActionCustom(handler.CustomHandler)
}

func (c *cmpdConvertor) lifecycleProbe(probe *appsv1alpha1.RoleProbe) *appsv1.Probe {
	if probe == nil {
		return nil
	}
	if probe.BuiltinHandler != nil {
		return &appsv1.Probe{
			Action: *c.lifecycleActionBuiltin(probe.BuiltinHandler),
		}
	}
	return &appsv1.Probe{
		Action:              *c.lifecycleActionCustom(probe.CustomHandler),
		InitialDelaySeconds: probe.InitialDelaySeconds,
		PeriodSeconds:       probe.PeriodSeconds,
	}
}

func (c *cmpdConvertor) lifecycleActionBuiltin(handler *appsv1alpha1.BuiltinActionHandlerType) *appsv1.Action {
	return &appsv1.Action{
		// MySQLBuiltinActionHandler              BuiltinActionHandlerType = "mysql"
		// WeSQLBuiltinActionHandler              BuiltinActionHandlerType = "wesql"
		// OceanbaseBuiltinActionHandler          BuiltinActionHandlerType = "oceanbase"
		// RedisBuiltinActionHandler              BuiltinActionHandlerType = "redis"
		// MongoDBBuiltinActionHandler            BuiltinActionHandlerType = "mongodb"
		// ETCDBuiltinActionHandler               BuiltinActionHandlerType = "etcd"
		// PostgresqlBuiltinActionHandler         BuiltinActionHandlerType = "postgresql"
		// OfficialPostgresqlBuiltinActionHandler BuiltinActionHandlerType = "official-postgresql"
		// ApeCloudPostgresqlBuiltinActionHandler BuiltinActionHandlerType = "apecloud-postgresql"
		// PolarDBXBuiltinActionHandler           BuiltinActionHandlerType = "polardbx"
		// CustomActionHandler                    BuiltinActionHandlerType = "custom"
		// UnknownBuiltinActionHandler            BuiltinActionHandlerType = "unknown"
		// TODO: convert BuiltinActionHandler to appsv1.Action
	}
}

func (c *cmpdConvertor) lifecycleActionCustom(handler *appsv1alpha1.Action) *appsv1.Action {
	return &appsv1.Action{
		Exec:           c.execAction(handler),
		TimeoutSeconds: handler.TimeoutSeconds,
		RetryPolicy:    c.retryPolicy(handler.RetryPolicy),
		PreCondition:   c.preCondition(handler.PreCondition),
	}
}

func (c *cmpdConvertor) execAction(action *appsv1alpha1.Action) *appsv1.ExecAction {
	if action == nil {
		return nil
	}
	newAction := &appsv1.ExecAction{
		Env:               action.Env,
		Image:             action.Image,
		TargetPodSelector: appsv1.TargetPodSelector(action.TargetPodSelector),
		MatchingKey:       action.MatchingKey,
		Container:         action.Container,
	}
	if action.Exec != nil {
		newAction.Command = action.Exec.Command
		newAction.Args = action.Exec.Args
	}
	return newAction
}

func (c *cmpdConvertor) retryPolicy(retryPolicy *appsv1alpha1.RetryPolicy) *appsv1.RetryPolicy {
	if retryPolicy == nil {
		return nil
	}
	return &appsv1.RetryPolicy{
		MaxRetries:    retryPolicy.MaxRetries,
		RetryInterval: retryPolicy.RetryInterval,
	}
}

func (c *cmpdConvertor) preCondition(preCondition *appsv1alpha1.PreConditionType) *appsv1.PreCondition {
	if preCondition == nil {
		return nil
	}
	cond := appsv1.PreCondition(*preCondition)
	return &cond
}

func (c *cmpdConvertor) serviceRefDeclarations(serviceRefs []appsv1alpha1.ServiceRefDeclaration) []appsv1.ServiceRefDeclaration {
	if len(serviceRefs) == 0 {
		return nil
	}
	newServiceRefs := make([]appsv1.ServiceRefDeclaration, 0)
	for i := range serviceRefs {
		newServiceRefs = append(newServiceRefs, appsv1.ServiceRefDeclaration{
			Name:                       serviceRefs[i].Name,
			ServiceRefDeclarationSpecs: c.serviceRefDeclarationSpecs(serviceRefs[i].ServiceRefDeclarationSpecs),
			Optional:                   serviceRefs[i].Optional,
		})
	}
	return newServiceRefs
}

func (c *cmpdConvertor) serviceRefDeclarationSpecs(specs []appsv1alpha1.ServiceRefDeclarationSpec) []appsv1.ServiceRefDeclarationSpec {
	if len(specs) == 0 {
		return nil
	}
	newSpecs := make([]appsv1.ServiceRefDeclarationSpec, 0)
	for i := range specs {
		newSpecs = append(newSpecs, appsv1.ServiceRefDeclarationSpec{
			ServiceKind:    specs[i].ServiceKind,
			ServiceVersion: specs[i].ServiceVersion,
		})
	}
	return newSpecs
}
