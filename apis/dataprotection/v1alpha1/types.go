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
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	corev1 "k8s.io/api/core/v1"
)

// Phase defines the BackupPolicy and ActionSet CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable}
type Phase string

const (
	AvailablePhase   Phase = "Available"
	UnavailablePhase Phase = "Unavailable"
)

func (p Phase) IsAvailable() bool {
	return p == AvailablePhase
}

// BackupRepoPhase denotes different stages for the `BackupRepo`.
//
// +enum
// +kubebuilder:validation:Enum={PreChecking,Failed,Ready,Deleting}
type BackupRepoPhase string

const (
	// BackupRepoPreChecking indicates the backup repository is being pre-checked.
	BackupRepoPreChecking BackupRepoPhase = "PreChecking"
	// BackupRepoFailed indicates the pre-check has been failed.
	BackupRepoFailed BackupRepoPhase = "Failed"
	// BackupRepoReady indicates the backup repository is ready for use.
	BackupRepoReady BackupRepoPhase = "Ready"
	// BackupRepoDeleting indicates the backup repository is being deleted.
	BackupRepoDeleting BackupRepoPhase = "Deleting"
)

// RetentionPeriod represents a duration in the format "1y2mo3w4d5h6m", where
// y=year, mo=month, w=week, d=day, h=hour, m=minute.
type RetentionPeriod string

// ToDuration converts the RetentionPeriod to time.Duration.
func (r RetentionPeriod) ToDuration() (time.Duration, error) {
	if len(r.String()) == 0 {
		return time.Duration(0), nil
	}

	minutes, err := r.toMinutes()
	if err != nil {
		return time.Duration(0), err
	}
	return time.Minute * time.Duration(minutes), nil
}

func (r RetentionPeriod) String() string {
	return string(r)
}

func (r RetentionPeriod) toMinutes() (int, error) {
	d, err := r.parseDuration()
	if err != nil {
		return 0, err
	}
	minutes := d.Minutes
	minutes += d.Hours * 60
	minutes += d.Days * 24 * 60
	minutes += d.Weeks * 7 * 24 * 60
	minutes += d.Months * 30 * 24 * 60
	minutes += d.Years * 365 * 24 * 60
	return minutes, nil
}

type duration struct {
	Minutes int
	Hours   int
	Days    int
	Weeks   int
	Months  int
	Years   int
}

var errInvalidDuration = errors.New("invalid duration provided")

// parseDuration parses a duration from a string. The format is `6y5m234d37h`
func (r RetentionPeriod) parseDuration() (duration, error) {
	var (
		d   duration
		num int
		err error
	)

	s := strings.TrimSpace(r.String())
	for s != "" {
		num, s, err = r.nextNumber(s)
		if err != nil {
			return duration{}, err
		}

		if len(s) == 0 {
			return duration{}, errInvalidDuration
		}

		if len(s) > 1 && s[0] == 'm' && s[1] == 'o' {
			d.Months = num
			s = s[2:]
			continue
		}

		switch s[0] {
		case 'y':
			d.Years = num
		case 'w':
			d.Weeks = num
		case 'd':
			d.Days = num
		case 'h':
			d.Hours = num
		case 'm':
			d.Minutes = num
		default:
			return duration{}, errInvalidDuration
		}
		s = s[1:]
	}
	return d, nil
}

func (r RetentionPeriod) nextNumber(input string) (num int, rest string, err error) {
	if len(input) == 0 {
		return 0, "", nil
	}

	var (
		n        string
		negative bool
	)

	if input[0] == '-' {
		negative = true
		input = input[1:]
	}

	for i, s := range input {
		if !unicode.IsNumber(s) {
			rest = input[i:]
			break
		}

		n += string(s)
	}

	if len(n) == 0 {
		return 0, input, errInvalidDuration
	}

	num, err = strconv.Atoi(n)
	if err != nil {
		return 0, input, err
	}

	if negative {
		num = -num
	}
	return num, rest, nil
}

// RestorePhase The current phase. Valid values are Running, Completed, Failed, AsDataSource.
// +enum
// +kubebuilder:validation:Enum={Running,Completed,Failed,AsDataSource}
type RestorePhase string

const (
	RestorePhaseRunning      RestorePhase = "Running"
	RestorePhaseCompleted    RestorePhase = "Completed"
	RestorePhaseFailed       RestorePhase = "Failed"
	RestorePhaseAsDataSource RestorePhase = "AsDataSource"
)

// RestoreActionStatus the status of restore action.
// +enum
// +kubebuilder:validation:Enum={Processing,Completed,Failed}
type RestoreActionStatus string

const (
	RestoreActionProcessing RestoreActionStatus = "Processing"
	RestoreActionCompleted  RestoreActionStatus = "Completed"
	RestoreActionFailed     RestoreActionStatus = "Failed"
)

type RestoreStage string

const (
	PrepareData RestoreStage = "prepareData"
	PostReady   RestoreStage = "postReady"
)

// VolumeClaimRestorePolicy defines restore policy for persistent volume claim.
// Supported policies are as follows:
//
// 1. Parallel: parallel recovery of persistent volume claim.
// 2. Serial: restore the persistent volume claim in sequence, and wait until the previous persistent volume claim is restored before restoring a new one.
//
// +enum
// +kubebuilder:validation:Enum={Parallel,Serial}
type VolumeClaimRestorePolicy string

const (
	VolumeClaimRestorePolicyParallel VolumeClaimRestorePolicy = "Parallel"
	VolumeClaimRestorePolicySerial   VolumeClaimRestorePolicy = "Serial"
)

type DataRestorePolicy string

const (
	OneToOneRestorePolicy  DataRestorePolicy = "OneToOne"
	OneToManyRestorePolicy DataRestorePolicy = "OneToMany"
)

const (
	DefaultEncryptionAlgorithm = "AES-256-CFB"
)

// EncryptionConfig defines the parameters for encrypting backup data.
type EncryptionConfig struct {
	// Specifies the encryption algorithm. Currently supported algorithms are:
	//
	// - AES-128-CFB
	// - AES-192-CFB
	// - AES-256-CFB
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=AES-256-CFB
	// +kubebuilder:validation:Enum={AES-128-CFB,AES-192-CFB,AES-256-CFB}
	Algorithm string `json:"algorithm"`

	// Selects the key of a secret in the current namespace, the value of the secret
	// is used as the encryption key.
	//
	// +kubebuilder:validation:Required
	PassPhraseSecretKeyRef *corev1.SecretKeySelector `json:"passPhraseSecretKeyRef"`
}
