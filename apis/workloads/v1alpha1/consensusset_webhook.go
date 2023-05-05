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

package v1alpha1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var consensussetlog = logf.Log.WithName("consensusset-resource")

func (r *ConsensusSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-workloads-kubeblocks-io-v1alpha1-consensusset,mutating=true,failurePolicy=fail,sideEffects=None,groups=workloads.kubeblocks.io,resources=consensussets,verbs=create;update,versions=v1alpha1,name=mconsensusset.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ConsensusSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ConsensusSet) Default() {
	consensussetlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-workloads-kubeblocks-io-v1alpha1-consensusset,mutating=false,failurePolicy=fail,sideEffects=None,groups=workloads.kubeblocks.io,resources=consensussets,verbs=create;update,versions=v1alpha1,name=vconsensusset.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ConsensusSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ConsensusSet) ValidateCreate() error {
	consensussetlog.Info("validate create", "name", r.Name)

	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ConsensusSet) ValidateUpdate(old runtime.Object) error {
	consensussetlog.Info("validate update", "name", r.Name)

	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ConsensusSet) ValidateDelete() error {
	consensussetlog.Info("validate delete", "name", r.Name)

	return r.validate()
}

func (r *ConsensusSet) validate() error {
	var allErrs field.ErrorList

	// roleObserveQuery and Leader are required
	if r.Spec.Leader.Name == "" {
		allErrs = append(allErrs,
			field.Required(field.NewPath("spec.leader.name"),
				"leader name can't be blank"))
	}

	// Leader.Replicas should not be present or should set to 1
	if *r.Spec.Leader.Replicas != 0 && *r.Spec.Leader.Replicas != 1 {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec.leader.replicas"),
				r.Spec.Leader.Replicas,
				"leader replicas can only be 1"))
	}

	// Leader.replicas + Follower.replicas should be odd
	candidates := int32(1)
	for _, member := range r.Spec.Followers {
		if member.Replicas != nil {
			candidates += *member.Replicas
		}
	}
	if candidates%2 == 0 {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec.candidates(leader.replicas+followers[*].replicas)"),
				candidates,
				"candidates(leader+followers) should be odd"))
	}
	// if spec.replicas is 1, then only Leader should be present. just omit if present

	// if Followers.Replicas present, Leader.Replicas(that is 1) + Followers.Replicas + Learner.Replicas should equal to spec.Replicas
	isFollowerPresent := false
	memberCount := int32(1)
	for _, member := range r.Spec.Followers {
		if member.Replicas != nil && *member.Replicas > 0 {
			isFollowerPresent = true
			memberCount += *member.Replicas
		}
	}
	if isFollowerPresent {
		if r.Spec.Learner != nil && r.Spec.Learner.Replicas != nil {
			memberCount += *r.Spec.Learner.Replicas
		}
		if memberCount != r.Spec.Replicas {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec.Replicas"),
					r.Spec.Replicas,
					"#(members) should be equal to Replicas"))
		}
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{
				Group: "core.kubeblocks.io/v1alpha1",
				Kind:  "ConsensusBlock",
			},
			r.Name, allErrs)
	}

	return nil
}
