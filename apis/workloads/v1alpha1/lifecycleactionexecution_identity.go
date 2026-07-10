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

package v1alpha1

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"regexp"
)

var invocationKeyPattern = regexp.MustCompile(`^[a-z2-7]{52}$`)

type canonicalObjectIdentityRef struct {
	APIGroup  string `json:"apiGroup"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

type canonicalReconfigureContext struct {
	ConfigName                   string `json:"configName"`
	TargetConfigHash             string `json:"targetConfigHash"`
	ComponentParameterGeneration int64  `json:"componentParameterGeneration"`
	OperationUID                 string `json:"operationUID"`
}

type canonicalLifecycleActionContext struct {
	Type        string                      `json:"type"`
	Reconfigure canonicalReconfigureContext `json:"reconfigure"`
}

type canonicalInvocationIdentity struct {
	IdentityVersion string                          `json:"identityVersion"`
	SourceRef       canonicalObjectIdentityRef      `json:"sourceRef"`
	WorkloadRef     canonicalObjectIdentityRef      `json:"workloadRef"`
	ActionName      string                          `json:"actionName"`
	Context         canonicalLifecycleActionContext `json:"context"`
}

type canonicalClusterContext struct {
	Type      string `json:"type"`
	Placement string `json:"placement"`
}

type canonicalPodTarget struct {
	ClusterContext canonicalClusterContext `json:"clusterContext"`
	Namespace      string                  `json:"namespace"`
	ComponentName  string                  `json:"componentName"`
	InstanceName   string                  `json:"instanceName"`
	PodName        string                  `json:"podName"`
	PodUID         string                  `json:"podUID"`
}

type canonicalLifecycleActionTarget struct {
	Type string             `json:"type"`
	Pod  canonicalPodTarget `json:"pod"`
}

type canonicalExecutionNameIdentity struct {
	NameVersion   string                         `json:"nameVersion"`
	InvocationKey string                         `json:"invocationKey"`
	Target        canonicalLifecycleActionTarget `json:"target"`
	Attempt       int32                          `json:"attempt"`
}

func ComputeLifecycleActionInvocationKey(spec LifecycleActionExecutionSpec) (string, []byte, error) {
	if err := validateInvocationIdentityEnvelope(spec); err != nil {
		return "", nil, err
	}
	envelope := canonicalInvocationIdentity{
		IdentityVersion: spec.IdentityVersion,
		SourceRef:       canonicalRef(spec.SourceRef),
		WorkloadRef:     canonicalRef(spec.WorkloadRef),
		ActionName:      spec.ActionName,
		Context: canonicalLifecycleActionContext{
			Type: string(spec.Context.Type),
			Reconfigure: canonicalReconfigureContext{
				ConfigName:                   spec.Context.Reconfigure.ConfigName,
				TargetConfigHash:             spec.Context.Reconfigure.TargetConfigHash,
				ComponentParameterGeneration: spec.Context.Reconfigure.ComponentParameterGeneration,
				OperationUID:                 string(spec.Context.Reconfigure.OperationUID),
			},
		},
	}
	return marshalIdentity(envelope)
}

func ComputeLifecycleActionExecutionName(spec LifecycleActionExecutionSpec) (string, []byte, error) {
	if err := validateExecutionNameEnvelope(spec); err != nil {
		return "", nil, err
	}
	placement := ""
	if spec.Target.Pod.ClusterContext.Placement != nil {
		placement = *spec.Target.Pod.ClusterContext.Placement
	}
	envelope := canonicalExecutionNameIdentity{
		NameVersion:   LifecycleActionExecutionNameVersionV1,
		InvocationKey: spec.InvocationKey,
		Target: canonicalLifecycleActionTarget{
			Type: string(spec.Target.Type),
			Pod: canonicalPodTarget{
				ClusterContext: canonicalClusterContext{Type: string(spec.Target.Pod.ClusterContext.Type), Placement: placement},
				Namespace:      spec.Target.Pod.Namespace,
				ComponentName:  spec.Target.Pod.ComponentName,
				InstanceName:   spec.Target.Pod.InstanceName,
				PodName:        spec.Target.Pod.PodName,
				PodUID:         string(spec.Target.Pod.PodUID),
			},
		},
		Attempt: spec.Attempt,
	}
	digest, canonical, err := marshalIdentity(envelope)
	if err != nil {
		return "", nil, err
	}
	return LifecycleActionExecutionNamePrefix + digest, canonical, nil
}

func validateInvocationIdentityEnvelope(spec LifecycleActionExecutionSpec) error {
	if spec.IdentityVersion != LifecycleActionIdentityVersionV1 {
		return fmt.Errorf("unsupported identity version %q", spec.IdentityVersion)
	}
	if err := validateObjectIdentityRef("sourceRef", spec.SourceRef); err != nil {
		return err
	}
	if err := validateObjectIdentityRef("workloadRef", spec.WorkloadRef); err != nil {
		return err
	}
	if spec.ActionName == "" {
		return fmt.Errorf("actionName is required")
	}
	if spec.Context.Type != LifecycleActionContextTypeReconfigure || spec.Context.Reconfigure == nil {
		return fmt.Errorf("reconfigure context is required")
	}
	reconfigure := spec.Context.Reconfigure
	if reconfigure.ConfigName == "" || reconfigure.TargetConfigHash == "" || reconfigure.ComponentParameterGeneration < 1 {
		return fmt.Errorf("reconfigure context is incomplete")
	}
	return nil
}

func validateObjectIdentityRef(name string, ref ObjectIdentityRef) error {
	if ref.APIGroup == "" || ref.Kind == "" || ref.Namespace == "" || ref.Name == "" || ref.UID == "" {
		return fmt.Errorf("%s is incomplete", name)
	}
	return nil
}

func validateExecutionNameEnvelope(spec LifecycleActionExecutionSpec) error {
	if !invocationKeyPattern.MatchString(spec.InvocationKey) {
		return fmt.Errorf("invocation key format is invalid")
	}
	if spec.Attempt < 1 {
		return fmt.Errorf("attempt must be positive")
	}
	if spec.Target.Type != LifecycleActionTargetTypePod || spec.Target.Pod == nil {
		return fmt.Errorf("pod target is required")
	}
	pod := spec.Target.Pod
	if pod.Namespace == "" || pod.ComponentName == "" || pod.InstanceName == "" || pod.PodName == "" || pod.PodUID == "" {
		return fmt.Errorf("pod target is incomplete")
	}
	switch pod.ClusterContext.Type {
	case LifecycleActionClusterContextLocal:
		if pod.ClusterContext.Placement != nil {
			return fmt.Errorf("local target cannot set placement")
		}
	case LifecycleActionClusterContextPlacement:
		if pod.ClusterContext.Placement == nil || *pod.ClusterContext.Placement == "" {
			return fmt.Errorf("placement target requires placement")
		}
	default:
		return fmt.Errorf("unknown cluster context %q", pod.ClusterContext.Type)
	}
	return nil
}

func canonicalRef(ref ObjectIdentityRef) canonicalObjectIdentityRef {
	return canonicalObjectIdentityRef{
		APIGroup: ref.APIGroup, Kind: ref.Kind, Namespace: ref.Namespace, Name: ref.Name, UID: string(ref.UID),
	}
}

func marshalIdentity(value any) (string, []byte, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(canonical)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:])
	return lowerASCII(encoded), canonical, nil
}

func lowerASCII(value string) string {
	result := []byte(value)
	for i := range result {
		if result[i] >= 'A' && result[i] <= 'Z' {
			result[i] += 'a' - 'A'
		}
	}
	return string(result)
}
