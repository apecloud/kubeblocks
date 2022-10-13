/*
Copyright 2022 The KubeBlocks Authors

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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	clusterdefinitionlog = logf.Log.WithName("clusterdefinition-resource")
)

func (r *ClusterDefinition) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-dbaas-infracreate-com-v1alpha1-clusterdefinition,mutating=true,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=clusterdefinitions,verbs=create;update,versions=v1alpha1,name=mclusterdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ClusterDefinition{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ClusterDefinition) Default() {
	clusterdefinitionlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-dbaas-infracreate-com-v1alpha1-clusterdefinition,mutating=false,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=clusterdefinitions,verbs=create;update,versions=v1alpha1,name=vclusterdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ClusterDefinition{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateCreate() error {
	clusterdefinitionlog.Info("validate create", "name", r.Name)
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateUpdate(old runtime.Object) error {
	clusterdefinitionlog.Info("validate update", "name", r.Name)
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateDelete() error {
	clusterdefinitionlog.Info("validate delete", "name", r.Name)
	return nil
}

// Validate ClusterDefinition.spec is legal
func (r *ClusterDefinition) validate() error {
	var (
		allErrs field.ErrorList
	)
	// clusterDefinition components to map
	componentMap := make(map[string]struct{})
	for _, v := range r.Spec.Components {
		componentMap[v.TypeName] = struct{}{}
	}

	r.validateComponents(&allErrs)

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: APIVersion, Kind: ClusterDefinitionKind},
			r.Name, allErrs)
	}
	return nil
}

// ValidateComponents validate spec.components is legal
func (r *ClusterDefinition) validateComponents(allErrs *field.ErrorList) {
	// TODO typeName duplication validate

	for _, component := range r.Spec.Components {
		if component.ComponentType != Consensus {
			continue
		}

		// if consensus
		consensusSpec := component.ConsensusSpec

		// roleObserveQuery and Leader are required
		if strings.TrimSpace(consensusSpec.Leader.Name) == "" {
			*allErrs = append(*allErrs,
				field.Required(field.NewPath("spec.components[*].consensusSpec.leader.name"),
					"leader name can't be blank when componentType is Consensus"))
		}

		// Leader.Replicas should not present or should set to 1
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
		isFollowerPresent := false
		memberCount := int32(1)
		for _, member := range consensusSpec.Followers {
			if member.Replicas != nil && *member.Replicas > 0 {
				isFollowerPresent = true
				memberCount += *member.Replicas
			}
		}
		if isFollowerPresent {
			if consensusSpec.Leader.Replicas != nil {
				memberCount += *consensusSpec.Learner.Replicas
			}
			if memberCount != int32(component.DefaultReplicas) {
				*allErrs = append(*allErrs,
					field.Invalid(field.NewPath("spec.components[*].consensusSpec.defaultReplicas"),
						component.DefaultReplicas,
						"#(members) should be equal to defaultReplicas"))
			}
		}
	}
}
