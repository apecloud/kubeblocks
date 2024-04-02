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

package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var (
	clusterdefinitionlog = logf.Log.WithName("clusterdefinition-resource")
)

// DefaultRoleProbeTimeoutAfterPodsReady the default role probe timeout for application when all pods of component are ready.
// default values are 60 seconds.
const DefaultRoleProbeTimeoutAfterPodsReady int32 = 60

func (r *ClusterDefinition) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-apps-kubeblocks-io-v1alpha1-clusterdefinition,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=clusterdefinitions,verbs=create;update,versions=v1alpha1,name=mclusterdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ClusterDefinition{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ClusterDefinition) Default() {
	clusterdefinitionlog.Info("default", "name", r.Name)
	for i := range r.Spec.ComponentDefs {
		probes := r.Spec.ComponentDefs[i].Probes
		if probes == nil {
			continue
		}
		if probes.RoleProbe != nil {
			// set default values
			if probes.RoleProbeTimeoutAfterPodsReady == 0 {
				probes.RoleProbeTimeoutAfterPodsReady = DefaultRoleProbeTimeoutAfterPodsReady
			}
		} else {
			// if component does not support RoleProbe, reset RoleProbeTimeoutAtPodsReady to zero
			if probes.RoleProbeTimeoutAfterPodsReady != 0 {
				probes.RoleProbeTimeoutAfterPodsReady = 0
			}
		}
		// set to CloneVolume if deprecated value used
		if r.Spec.ComponentDefs[i].HorizontalScalePolicy != nil &&
			r.Spec.ComponentDefs[i].HorizontalScalePolicy.Type == HScaleDataClonePolicyFromSnapshot {
			r.Spec.ComponentDefs[i].HorizontalScalePolicy.Type = HScaleDataClonePolicyCloneVolume
		}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-clusterdefinition,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=clusterdefinitions,verbs=create;update,versions=v1alpha1,name=vclusterdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ClusterDefinition{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateCreate() (admission.Warnings, error) {
	clusterdefinitionlog.Info("validate create", "name", r.Name)
	return nil, r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	clusterdefinitionlog.Info("validate update", "name", r.Name)
	return nil, r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateDelete() (admission.Warnings, error) {
	clusterdefinitionlog.Info("validate delete", "name", r.Name)
	return nil, nil
}

// Validate ClusterDefinition.spec is legal
func (r *ClusterDefinition) validate() error {
	var (
		allErrs field.ErrorList
	)
	// clusterDefinition components to map
	componentMap := make(map[string]struct{})
	for _, v := range r.Spec.ComponentDefs {
		componentMap[v.Name] = struct{}{}
	}

	r.validateComponents(&allErrs)
	r.validateLogFilePatternPrefix(&allErrs)

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: APIVersion, Kind: ClusterDefinitionKind},
			r.Name, allErrs)
	}
	return nil
}

// validateLogsPatternPrefix validate spec.components[*].logConfigs[*].filePathPattern
func (r *ClusterDefinition) validateLogFilePatternPrefix(allErrs *field.ErrorList) {
	for idx1, component := range r.Spec.ComponentDefs {
		if len(component.LogConfigs) == 0 {
			continue
		}
		volumeMounts := component.PodSpec.Containers[0].VolumeMounts
		for idx2, logConfig := range component.LogConfigs {
			flag := false
			for _, v := range volumeMounts {
				if strings.HasPrefix(logConfig.FilePathPattern, v.MountPath) {
					flag = true
					break
				}
			}
			if !flag {
				*allErrs = append(*allErrs, field.Required(field.NewPath(fmt.Sprintf("spec.components[%d].logConfigs[%d].filePathPattern", idx1, idx2)),
					fmt.Sprintf("filePathPattern %s should have a prefix string which in container VolumeMounts", logConfig.FilePathPattern)))
			}
		}
	}
}

// ValidateComponents validate spec.components is legal.
func (r *ClusterDefinition) validateComponents(allErrs *field.ErrorList) {

	validateSystemAccount := func(component *ClusterComponentDefinition) {
		sysAccountSpec := component.SystemAccounts
		if sysAccountSpec != nil {
			sysAccountSpec.validate(allErrs)
		}
	}

	validateConsensus := func(component *ClusterComponentDefinition) {
		consensusSpec := component.ConsensusSpec
		// roleObserveQuery and Leader are required
		if consensusSpec.Leader.Name == "" {
			*allErrs = append(*allErrs,
				field.Required(field.NewPath("spec.components[*].consensusSpec.leader.name"),
					"leader name can't be blank when workloadType is Consensus"))
		}

		// Leader.Replicas should not be present or should set to 1
		if *consensusSpec.Leader.Replicas != 0 && *consensusSpec.Leader.Replicas != 1 {
			*allErrs = append(*allErrs,
				field.Invalid(field.NewPath("spec.components[*].consensusSpec.leader.replicas"),
					consensusSpec.Leader.Replicas,
					"leader replicas can only be 1"))
		}

		// Leader.replicas + Follower.replicas should be odd
		candidates := int32(1)
		for _, member := range consensusSpec.Followers {
			if member.Replicas != nil {
				candidates += *member.Replicas
			}
		}
		if candidates%2 == 0 {
			*allErrs = append(*allErrs,
				field.Invalid(field.NewPath("spec.components[*].consensusSpec.candidates(leader.replicas+followers[*].replicas)"),
					candidates,
					"candidates(leader+followers) should be odd"))
		}
		// if component.replicas is 1, then only Leader should be present. just omit if present

		// if Followers.Replicas present, Leader.Replicas(that is 1) + Followers.Replicas + Learner.Replicas should equal to component.defaultReplicas
	}

	for _, component := range r.Spec.ComponentDefs {
		for _, compRef := range component.ComponentDefRef {
			compRef.validate(allErrs, r)
		}

		if err := r.validateConfigSpec(component); err != nil {
			*allErrs = append(*allErrs, field.Duplicate(field.NewPath("spec.components[*].configSpec.configTemplateRefs"), err))
			continue
		}

		// validate system account defined in spec.components[].systemAccounts
		validateSystemAccount(&component)

		switch component.WorkloadType {
		case Consensus:
			// if consensus
			consensusSpec := component.ConsensusSpec
			if consensusSpec == nil {
				*allErrs = append(*allErrs,
					field.Required(field.NewPath("spec.components[*].consensusSpec"),
						"consensusSpec is required when workloadType=Consensus"))
				continue
			}
			validateConsensus(&component)
		case Replication:
		default:
			continue
		}
	}
}

// validate validates spec.components[].systemAccounts
func (r *SystemAccountSpec) validate(allErrs *field.ErrorList) {
	accountName := make(map[AccountName]bool)
	for _, sysAccount := range r.Accounts {
		// validate provision policy
		provisionPolicy := sysAccount.ProvisionPolicy
		if provisionPolicy.Type == CreateByStmt && sysAccount.ProvisionPolicy.Statements == nil {
			*allErrs = append(*allErrs,
				field.Invalid(field.NewPath("spec.components[*].systemAccounts.accounts.provisionPolicy.statements"),
					sysAccount.Name, "statements should not be empty when provisionPolicy = CreateByStmt."))
			continue
		}

		if sysAccount.ProvisionPolicy.Statements != nil {
			updateStmt := sysAccount.ProvisionPolicy.Statements.UpdateStatement
			deletionStmt := sysAccount.ProvisionPolicy.Statements.DeletionStatement
			if len(updateStmt) == 0 && len(deletionStmt) == 0 {
				*allErrs = append(*allErrs,
					field.Invalid(field.NewPath("spec.components[*].systemAccounts.accounts.provisionPolicy.statements"),
						sysAccount.Name, "either statements.update or statements.deletion should be specified."))
				continue
			}
		}

		if provisionPolicy.Type == ReferToExisting && sysAccount.ProvisionPolicy.SecretRef == nil {
			*allErrs = append(*allErrs,
				field.Invalid(field.NewPath("spec.components[*].systemAccounts.accounts.provisionPolicy.secretRef"),
					sysAccount.Name, "SecretRef should not be empty when provisionPolicy = ReferToExisting. "))
			continue
		}
		// account names should be unique
		if _, exists := accountName[sysAccount.Name]; exists {
			*allErrs = append(*allErrs,
				field.Invalid(field.NewPath("spec.components[*].systemAccounts.accounts"),
					sysAccount.Name, "duplicated system account names are not allowed."))
			continue
		} else {
			accountName[sysAccount.Name] = true
		}
	}

	passwdConfig := r.PasswordConfig
	if passwdConfig.Length < passwdConfig.NumDigits+passwdConfig.NumSymbols {
		*allErrs = append(*allErrs,
			field.Invalid(field.NewPath("spec.components[*].systemAccounts.passwordConfig"),
				passwdConfig, "numDigits plus numSymbols exceeds password length. "))
	}
}

func (r *ClusterDefinition) validateConfigSpec(component ClusterComponentDefinition) error {
	if len(component.ConfigSpecs) <= 1 && len(component.ScriptSpecs) <= 1 {
		return nil
	}
	return validateConfigTemplateList(component.ConfigSpecs)
}

func validateConfigTemplateList(ctpls []ComponentConfigSpec) error {
	var (
		volumeSet = map[string]struct{}{}
		cmSet     = map[string]struct{}{}
		tplSet    = map[string]struct{}{}
	)

	for _, tpl := range ctpls {
		if len(tpl.VolumeName) == 0 {
			return errors.Errorf("ConfigTemplate.VolumeName not empty.")
		}
		if _, ok := tplSet[tpl.Name]; ok {
			return errors.Errorf("configTemplate[%s] already existed.", tpl.Name)
		}
		if _, ok := volumeSet[tpl.VolumeName]; ok {
			return errors.Errorf("volume[%s] already existed.", tpl.VolumeName)
		}
		if _, ok := cmSet[tpl.TemplateRef]; ok {
			return errors.Errorf("configmap[%s] already existed.", tpl.TemplateRef)
		}
		tplSet[tpl.Name] = struct{}{}
		cmSet[tpl.TemplateRef] = struct{}{}
		volumeSet[tpl.VolumeName] = struct{}{}
	}
	return nil
}

func (r ComponentDefRef) validate(allErrs *field.ErrorList, clusterDef *ClusterDefinition) {
	if len(r.ComponentDefName) == 0 {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("componentDefName"), r.ComponentDefName, "componentDefName cannot be empty"))
	}

	for _, env := range r.ComponentRefEnvs {
		if len(env.Value) > 0 && env.ValueFrom != nil {
			*allErrs = append(*allErrs, field.Invalid(field.NewPath("componentRefEnv[*].value"), env.Value, "value and valueFrom cannot be set at the same time"))
		}
		if len(env.Value) == 0 && env.ValueFrom == nil {
			*allErrs = append(*allErrs, field.Invalid(field.NewPath("componentRefEnv[*].value"), env.Value, "value and valueFrom cannot be empty at the same time"))
		}
		if env.ValueFrom == nil {
			continue
		}
		valueFrom := env.ValueFrom
		switch valueFrom.Type {
		case FromFieldRef:
			if len(valueFrom.FieldPath) == 0 {
				*allErrs = append(*allErrs, field.Invalid(field.NewPath("componentRefEnv[*].valueFrom"), valueFrom.FieldPath, "fieldRef cannot be empty"))
			}
		case FromHeadlessServiceRef:
			if len(valueFrom.FieldPath) > 0 {
				*allErrs = append(*allErrs, field.Invalid(field.NewPath("componentRefEnv[*].valueFrom"), valueFrom, "headlessServiceRef cannot set fieldPath"))
			}
		}
		// get the componentDef by name
		compDefName := r.ComponentDefName
		compDef := clusterDef.GetComponentDefByName(compDefName)
		if compDef == nil {
			*allErrs = append(*allErrs, field.Invalid(field.NewPath("componentRefEnv[*].componentDefName"), valueFrom, "componentDefName is invalid"))
		} else if env.ValueFrom.Type == FromHeadlessServiceRef && compDef.WorkloadType == Stateless {
			*allErrs = append(*allErrs, field.Invalid(field.NewPath("componentRefEnv[*].valueFrom"), valueFrom, "headlessServiceRef is only valid for statefulset"))
		}
	}
}
