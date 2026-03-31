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

// InitParameters describes the initialization overlays keyed by cluster sub-resource name,
// such as a component or sharding item name.
type InitParameters map[string]ParameterValues

// EncodeInitParameters serializes the initialization payload.
func EncodeInitParameters(v InitParameters) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DecodeInitParameters deserializes the initialization payload.
// Empty input returns an empty value without error.
func DecodeInitParameters(raw string) (InitParameters, error) {
	if raw == "" {
		return InitParameters{}, nil
	}
	ret := InitParameters{}
	if err := json.Unmarshal([]byte(raw), &ret); err != nil {
		return nil, err
	}
	if ret == nil {
		return InitParameters{}, nil
	}
	return ret, nil
}

// ParseInitParameters extracts and deserializes initialization overlays from the Cluster object.
func ParseInitParameters(cluster *appsv1.Cluster) (InitParameters, error) {
	if cluster == nil || len(cluster.Annotations) == 0 {
		return InitParameters{}, nil
	}
	return DecodeInitParameters(cluster.Annotations[initParameterKey])
}

// SetInitParameters serializes and stores the initialization payload on the given Cluster object.
// Passing an empty payload clears the metadata entry.
func SetInitParameters(cluster *appsv1.Cluster, params InitParameters) error {
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
	raw, err := EncodeInitParameters(params)
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
func (v InitParameters) Get(name string) *ParameterValues {
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
