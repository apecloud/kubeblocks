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

	// Leader is required
	hasHeader := false
	for _, role := range r.Spec.Roles {
		if role.IsLeader && len(role.Name) > 0 {
			hasHeader = true
		}
	}
	if !hasHeader {
		allErrs = append(allErrs,
			field.Required(field.NewPath("spec.roles"),
				"leader is required"))
	}

	for _, role := range r.Spec.Roles {
		if len(role.Name) == 0 {
			allErrs = append(allErrs,
				field.Required(field.NewPath("spec.roles[*].name"),
					"role name can't be empty"))
		}
		if role.AccessMode != NoneMode &&
			role.AccessMode != ReadonlyMode &&
			role.AccessMode != ReadWriteMode {
			allErrs = append(allErrs,
				field.Required(field.NewPath("spec.roles[*].accessMode"),
					"invalid accessMode, should be one of [None, Readonly, ReadWrite]"))
		}
	}

	if r.Spec.RoleObservation.BuiltIn != nil {
		binding := r.Spec.RoleObservation.BuiltIn.BindingType
		if binding != ApeCloudMySQLBinding &&
			binding != ETCDBinding &&
			binding != ZooKeeperBinding &&
			binding != MongoDBBinding {
			allErrs = append(allErrs,
				field.Required(field.NewPath("spec.roleObservation.builtIn.bindingType"),
					"invalid bindingType, should be one of [apecloud-mysql, etcd, zookeeper, mongodb]"))
		}
	}

	if r.Spec.UpdateStrategy != SerialUpdateStrategy &&
		r.Spec.UpdateStrategy != ParallelUpdateStrategy &&
		r.Spec.UpdateStrategy != BestEffortParallelUpdateStrategy {
		allErrs = append(allErrs,
			field.Required(field.NewPath("spec.updateStrategy"),
				"invalid updateStrategy, should be one of [Serial, BestEffortParallel, Parallel]"))
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{
				Group: "workloads.kubeblocks.io/v1alpha1",
				Kind:  "ConsensusSet",
			},
			r.Name, allErrs)
	}

	return nil
}
