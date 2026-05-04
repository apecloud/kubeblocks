/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package restore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// RestoreOptions is the DP-owned extension carried by PVC templates and PVCs
// whose spec.dataSourceRef points to a DP Backup.
type RestoreOptions struct {
	// BackupNamespace identifies the source Backup namespace when it differs
	// from the target PVC namespace. Empty means the target PVC namespace.
	BackupNamespace string `json:"backupNamespace,omitempty"`
	// RestoreTime is the PITR timestamp in RFC3339 format.
	RestoreTime string `json:"restoreTime,omitempty"`
	// VolumeSource identifies the backup target volume restored into this PVC.
	VolumeSource string `json:"volumeSource,omitempty"`
	// MountPath identifies the restore container mount path for this PVC.
	MountPath string `json:"mountPath,omitempty"`
	// SourceTargetName identifies the backup source target used by this PVC.
	SourceTargetName string `json:"sourceTargetName,omitempty"`
	// VolumeRestorePolicy controls prepare-data execution order.
	VolumeRestorePolicy dpv1alpha1.VolumeClaimRestorePolicy `json:"volumeRestorePolicy,omitempty"`
	// DeferPostReadyUntilClusterRunning delays post-ready actions until the
	// whole target Cluster is Running.
	DeferPostReadyUntilClusterRunning bool `json:"deferPostReadyUntilClusterRunning,omitempty"`
	// Env is passed through to DP restore actions.
	Env []corev1.EnvVar `json:"env,omitempty"`
	// Parameters is passed through to DP restore actions.
	Parameters []dpv1alpha1.ParameterPair `json:"parameters,omitempty"`
}

// DefaultRestoreOptions returns RestoreOptions with DP API defaults applied.
func DefaultRestoreOptions() RestoreOptions {
	return RestoreOptions{
		VolumeRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicyParallel,
	}
}

// BackupDataSourceRef returns the PVC dataSourceRef form that represents the
// cluster-facing DP restore input API.
func BackupDataSourceRef(name string) *corev1.TypedObjectReference {
	apiGroup := dptypes.DataprotectionAPIGroup
	return &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     dptypes.BackupKind,
		Name:     name,
	}
}

// IsBackupDataSourceRef reports whether ref points to a DP Backup.
func IsBackupDataSourceRef(ref *corev1.TypedObjectReference) bool {
	if ref == nil || ref.APIGroup == nil {
		return false
	}
	return *ref.APIGroup == dptypes.DataprotectionAPIGroup &&
		ref.Kind == dptypes.BackupKind &&
		ref.Name != ""
}

// MarshalRestoreOptions serializes RestoreOptions for
// dataprotection.kubeblocks.io/restore-options.
func MarshalRestoreOptions(options RestoreOptions) (string, error) {
	options.defaultInplace()
	if err := options.Validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(options)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SetRestoreOptions writes options to annotations using the DP restore options
// annotation key. The returned map is safe to assign back to ObjectMeta.
func SetRestoreOptions(annotations map[string]string, options RestoreOptions) (map[string]string, error) {
	value, err := MarshalRestoreOptions(options)
	if err != nil {
		return nil, err
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[dptypes.RestoreOptionsAnnotationKey] = value
	return annotations, nil
}

// ParseRestoreOptions reads RestoreOptions from annotations. Missing annotation
// returns defaulted options.
func ParseRestoreOptions(annotations map[string]string) (RestoreOptions, error) {
	options := DefaultRestoreOptions()
	if annotations == nil || annotations[dptypes.RestoreOptionsAnnotationKey] == "" {
		return options, nil
	}
	decoder := json.NewDecoder(bytes.NewBufferString(annotations[dptypes.RestoreOptionsAnnotationKey]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&options); err != nil {
		return RestoreOptions{}, err
	}
	options.defaultInplace()
	if err := options.Validate(); err != nil {
		return RestoreOptions{}, err
	}
	return options, nil
}

// Validate checks the DP restore options API surface that can be validated
// without looking up cluster objects.
func (o RestoreOptions) Validate() error {
	if o.RestoreTime != "" {
		if _, err := time.Parse(time.RFC3339, o.RestoreTime); err != nil {
			return fmt.Errorf("restoreTime must be RFC3339: %w", err)
		}
	}
	switch o.VolumeRestorePolicy {
	case "", dpv1alpha1.VolumeClaimRestorePolicyParallel, dpv1alpha1.VolumeClaimRestorePolicySerial:
	default:
		return fmt.Errorf("unsupported volumeRestorePolicy %q", o.VolumeRestorePolicy)
	}
	return nil
}

// VolumeConfig returns the restore volume config represented by these options.
func (o RestoreOptions) VolumeConfig() dpv1alpha1.VolumeConfig {
	return dpv1alpha1.VolumeConfig{
		VolumeSource: o.VolumeSource,
		MountPath:    o.MountPath,
	}
}

func (o *RestoreOptions) defaultInplace() {
	if o.VolumeRestorePolicy == "" {
		o.VolumeRestorePolicy = dpv1alpha1.VolumeClaimRestorePolicyParallel
	}
}
