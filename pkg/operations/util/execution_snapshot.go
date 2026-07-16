/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package util

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	opsDefinitionSpecHashDomain = "kubeblocks.io/ops-definition-spec/v1"
	opsRequestSpecHashDomain    = "kubeblocks.io/ops-request-spec/v1"
	managedJobSpecHashDomain    = "kubeblocks.io/managed-job-spec/v1"
)

// CanonicalObjectHash returns a domain-separated SHA-256 hash of a JSON API value.
// encoding/json sorts map keys, which makes Kubernetes API structs stable across reconciles.
func CanonicalObjectHash(domain string, value any) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("hash domain must not be empty")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(data)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// OpsDefinitionSpecHash binds an action attempt to an exact OpsDefinition spec.
func OpsDefinitionSpecHash(value any) (string, error) {
	return CanonicalObjectHash(opsDefinitionSpecHashDomain, value)
}

// OpsRequestSpecHash binds an action attempt to an exact OpsRequest spec.
func OpsRequestSpecHash(value any) (string, error) {
	return CanonicalObjectHash(opsRequestSpecHashDomain, value)
}

// ManagedJobSpecHash binds a managed action to the API-server-defaulted Job spec selected before dispatch.
func ManagedJobSpecHash(value any) (string, error) {
	return CanonicalObjectHash(managedJobSpecHashDomain, value)
}

// ManagedJobSelectorValue returns the full-UID selector shared by a managed Job and its Pods.
func ManagedJobSelectorValue(opsRequestUID string, taskIndex int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%d", opsRequestUID, taskIndex)))
	// Unpadded base32 preserves all 256 bits in a 52-character label value.
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
}
