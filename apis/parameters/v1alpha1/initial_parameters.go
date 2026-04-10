/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"encoding/json"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const (
	// initParameterKey identifies the Cluster metadata entry used to carry
	// initialization overlays for parameterized config generation.
	//
	// The current transport is Cluster.metadata.annotations[initParameterKey].
	initParameterKey = "config.kubeblocks.io/init-parameters"
)

// InitialParameters describes the initialization overlays keyed by cluster sub-resource name,
// such as a component or sharding item name.
type InitialParameters map[string]ParameterInputs

// EncodeInitialParameters serializes the initialization payload.
func EncodeInitialParameters(v InitialParameters) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DecodeInitialParameters deserializes the initialization payload.
// Empty input returns an empty value without error.
func DecodeInitialParameters(raw string) (InitialParameters, error) {
	if raw == "" {
		return InitialParameters{}, nil
	}
	ret := InitialParameters{}
	if err := json.Unmarshal([]byte(raw), &ret); err != nil {
		return nil, err
	}
	if ret == nil {
		return InitialParameters{}, nil
	}
	return ret, nil
}

// ParseInitialParameters extracts and deserializes initialization overlays from the Cluster object.
func ParseInitialParameters(cluster *appsv1.Cluster) (InitialParameters, error) {
	if cluster == nil || len(cluster.Annotations) == 0 {
		return InitialParameters{}, nil
	}
	return DecodeInitialParameters(cluster.Annotations[initParameterKey])
}

// SetInitialParameters serializes and stores the initialization payload on the given Cluster object.
// Passing an empty payload clears the metadata entry.
func SetInitialParameters(cluster *appsv1.Cluster, params InitialParameters) error {
	if cluster == nil {
		return nil
	}
	if len(params) == 0 {
		if len(cluster.Annotations) == 0 {
			return nil
		}
		delete(cluster.Annotations, initParameterKey)
		if len(cluster.Annotations) == 0 {
			cluster.Annotations = nil
		}
		return nil
	}
	raw, err := EncodeInitialParameters(params)
	if err != nil {
		return err
	}
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	cluster.Annotations[initParameterKey] = raw
	return nil
}

// Get returns the initialization overlay for the given sub-resource name.
func (v InitialParameters) Get(name string) *ParameterInputs {
	if len(v) == 0 {
		return nil
	}
	spec, ok := v[name]
	if !ok {
		return nil
	}
	out := spec
	return &out
}
