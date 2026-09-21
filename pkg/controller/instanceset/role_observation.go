/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instanceset

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// RoleObservation is the last role result accepted for one Pod. The Pod name
// is the annotation owner, so it is intentionally not duplicated here.
type RoleObservation struct {
	Role                 string  `json:"role,omitempty"`
	RoleDefined          bool    `json:"roleDefined"`
	EventVersion         string  `json:"eventVersion,omitempty"`
	AuthoritativeVersion *uint64 `json:"authoritativeVersion,omitempty"`
}

func EncodeRoleObservation(observation RoleObservation) (string, error) {
	data, err := json.Marshal(observation)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func DecodeRoleObservation(pod *corev1.Pod) (RoleObservation, bool, error) {
	if pod == nil || pod.Annotations == nil {
		return RoleObservation{}, false, nil
	}
	value := pod.Annotations[constant.RoleObservationAnnotationKey]
	if value == "" {
		return RoleObservation{}, false, nil
	}
	var observation RoleObservation
	if err := json.Unmarshal([]byte(value), &observation); err != nil {
		return RoleObservation{}, false, fmt.Errorf("decode role observation for Pod %s: %w", pod.Name, err)
	}
	return observation, true, nil
}

// RoleLabelClaims returns the role label that each observed Pod should carry.
// A missing map entry means the controller has no accepted observation for the
// Pod and must leave its existing label alone. An empty value means an accepted
// observation says that the Pod must not carry a role label.
func RoleLabelClaims(roles []workloads.ReplicaRole, pods []*corev1.Pod) (map[string]string, error) {
	roleMap := make(map[string]workloads.ReplicaRole, len(roles))
	for _, role := range roles {
		roleMap[strings.ToLower(role.Name)] = role
	}

	type candidate struct {
		podName     string
		role        workloads.ReplicaRole
		observation RoleObservation
	}
	claims := make(map[string]string)
	exclusive := make(map[string][]candidate)
	for _, pod := range pods {
		observation, ok, err := DecodeRoleObservation(pod)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if !observation.RoleDefined {
			claims[pod.Name] = ""
			continue
		}
		role, ok := roleMap[strings.ToLower(observation.Role)]
		if !ok {
			claims[pod.Name] = ""
			continue
		}
		candidate := candidate{podName: pod.Name, role: role, observation: observation}
		if role.IsExclusive {
			exclusive[strings.ToLower(role.Name)] = append(exclusive[strings.ToLower(role.Name)], candidate)
			continue
		}
		claims[pod.Name] = role.Name
	}

	for _, candidates := range exclusive {
		winner := candidates[0]
		for _, candidate := range candidates[1:] {
			if newerRoleObservation(candidate.observation, candidate.podName, winner.observation, winner.podName) {
				winner = candidate
			}
		}
		claims[winner.podName] = winner.role.Name
		for _, candidate := range candidates {
			if candidate.podName != winner.podName {
				claims[candidate.podName] = ""
			}
		}
	}
	return claims, nil
}

func newerRoleObservation(a RoleObservation, aPod string, b RoleObservation, bPod string) bool {
	if a.AuthoritativeVersion != nil || b.AuthoritativeVersion != nil {
		switch {
		case a.AuthoritativeVersion != nil && b.AuthoritativeVersion == nil:
			return true
		case a.AuthoritativeVersion == nil && b.AuthoritativeVersion != nil:
			return false
		case *a.AuthoritativeVersion != *b.AuthoritativeVersion:
			return *a.AuthoritativeVersion > *b.AuthoritativeVersion
		}
	} else {
		aVersion, aOK := parseEventVersion(a.EventVersion)
		bVersion, bOK := parseEventVersion(b.EventVersion)
		if aOK && bOK && aVersion != bVersion {
			return aVersion > bVersion
		}
		if a.EventVersion != b.EventVersion {
			return a.EventVersion > b.EventVersion
		}
	}
	return aPod > bPod
}

func parseEventVersion(version string) (int64, bool) {
	if strings.HasPrefix(version, "obs:") {
		value, err := strconv.ParseInt(strings.TrimPrefix(version, "obs:"), 10, 64)
		return value, err == nil
	}
	value, err := strconv.ParseInt(version, 10, 64)
	return value, err == nil
}
