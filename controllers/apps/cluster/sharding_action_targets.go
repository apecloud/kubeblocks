/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

KubeBlocks is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

KubeBlocks is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with KubeBlocks.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const shardingActionTargetsGZIPPrefix = "gzip:"

func getShardingActionTargets(comp *appsv1.Component, annotation string) (*shardingActionTargets, bool, error) {
	value, found := comp.Annotations[annotation]
	if !found {
		return nil, false, nil
	}

	data := []byte(value)
	if strings.HasPrefix(value, shardingActionTargetsGZIPPrefix) {
		compressed, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, shardingActionTargetsGZIPPrefix))
		if err != nil {
			return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, comp.Name, err)
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, comp.Name, err)
		}
		data, err = io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, comp.Name, err)
		}
		if closeErr != nil {
			return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, comp.Name, closeErr)
		}
	}

	targets := &shardingActionTargets{}
	if err := json.Unmarshal(data, targets); err != nil {
		return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, comp.Name, err)
	}
	if err := validateShardingActionTargets(targets); err != nil {
		return nil, false, fmt.Errorf("invalid %s annotation on component %s: %w", annotation, comp.Name, err)
	}
	return targets, true, nil
}

func setShardingActionTargets(comp *appsv1.Component, annotation string, targets *shardingActionTargets) error {
	sortShardingActionTargets(targets)
	data, err := json.Marshal(targets)
	if err != nil {
		return err
	}

	withValue := func(value string) map[string]string {
		annotations := make(map[string]string, len(comp.Annotations)+1)
		for key, current := range comp.Annotations {
			annotations[key] = current
		}
		annotations[annotation] = value
		return annotations
	}
	if annotations := withValue(string(data)); apivalidation.ValidateAnnotationsSize(annotations) == nil {
		comp.Annotations = annotations
		return nil
	}

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	value := shardingActionTargetsGZIPPrefix + base64.StdEncoding.EncodeToString(compressed.Bytes())

	annotations := withValue(value)
	if err := apivalidation.ValidateAnnotationsSize(annotations); err != nil {
		return fmt.Errorf("cannot persist targets for component %s: %w", comp.Name, err)
	}
	comp.Annotations = annotations
	return nil
}

func deleteShardingActionTargets(comp *appsv1.Component, annotation string) {
	delete(comp.Annotations, annotation)
}

func sortShardingActionTargets(targets *shardingActionTargets) {
	for i := range targets.Targets {
		sort.Slice(targets.Targets[i].Pods, func(j, k int) bool {
			return targets.Targets[i].Pods[j].Name < targets.Targets[i].Pods[k].Name
		})
	}
	sort.Slice(targets.Targets, func(i, j int) bool {
		return targets.Targets[i].Component < targets.Targets[j].Component
	})
}
