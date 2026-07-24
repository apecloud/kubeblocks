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

package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

// clusterStatusCASVertex records an exclusive Cluster status JSON Patch intent.
// Execution must commit this vertex alone so a successful authority install
// cannot be followed by stale object writes from the same reconciliation.
type clusterStatusCASVertex struct {
	cluster *appsv1.Cluster
	patch   []byte
}

type clusterStatusCASOperation struct {
	Operation string          `json:"op"`
	Path      string          `json:"path"`
	Value     json.RawMessage `json:"value,omitempty"`
}

func (v *clusterStatusCASVertex) String() string {
	if v == nil || v.cluster == nil {
		return "{cluster-status-cas:nil}"
	}
	return fmt.Sprintf("{cluster-status-cas:%s/%s}", v.cluster.Namespace, v.cluster.Name)
}

func addClusterStatusCASVertex(dag *graph.DAG, cluster *appsv1.Cluster, patch []byte) error {
	if dag == nil || dag.Root() == nil {
		return fmt.Errorf("cluster status CAS requires a rooted DAG")
	}
	if cluster == nil || cluster.Namespace == "" || cluster.Name == "" ||
		cluster.UID == "" || cluster.ResourceVersion == "" {
		return fmt.Errorf("cluster status CAS requires exact Cluster identity")
	}
	trimmedPatch := bytes.TrimSpace(patch)
	if err := validateClusterStatusCASPatch(cluster, trimmedPatch); err != nil {
		return err
	}
	vertex := &clusterStatusCASVertex{
		cluster: cluster.DeepCopy(),
		patch:   append([]byte(nil), trimmedPatch...),
	}
	if !dag.AddConnectRoot(vertex) {
		return fmt.Errorf("failed to add Cluster status CAS vertex")
	}
	return nil
}

func validateClusterStatusCASPatch(cluster *appsv1.Cluster, patch []byte) error {
	if len(patch) == 0 || patch[0] != '[' {
		return fmt.Errorf("cluster status CAS requires a valid JSON Patch array")
	}
	operations := []clusterStatusCASOperation{}
	decoder := json.NewDecoder(bytes.NewReader(patch))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&operations); err != nil {
		return fmt.Errorf("cluster status CAS requires a valid JSON Patch array: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("cluster status CAS patch must contain one JSON value")
	}
	if len(operations) == 0 {
		return fmt.Errorf("cluster status CAS patch must not be empty")
	}

	uidTests, resourceVersionTests, statusMutations := 0, 0, 0
	for i, operation := range operations {
		hasValue := len(operation.Value) != 0
		switch operation.Operation {
		case "test":
			if !hasValue {
				return fmt.Errorf("cluster status CAS operation %d test value must not be empty", i)
			}
			switch {
			case operation.Path == "/metadata/uid":
				uidTests++
				if !clusterStatusCASStringValueEquals(operation.Value, string(cluster.UID)) {
					return fmt.Errorf("cluster status CAS UID test must match the Cluster")
				}
			case operation.Path == "/metadata/resourceVersion":
				resourceVersionTests++
				if !clusterStatusCASStringValueEquals(operation.Value, cluster.ResourceVersion) {
					return fmt.Errorf("cluster status CAS resourceVersion test must match the Cluster")
				}
			case isClusterStatusCASStatusPath(operation.Path):
			default:
				return fmt.Errorf("cluster status CAS test path %q is not allowed", operation.Path)
			}
		case "add", "replace":
			if !hasValue {
				return fmt.Errorf("cluster status CAS operation %d value must not be empty", i)
			}
			if !isClusterStatusCASStatusPath(operation.Path) {
				return fmt.Errorf("cluster status CAS mutation path %q is not allowed", operation.Path)
			}
			statusMutations++
		case "remove":
			if hasValue {
				return fmt.Errorf("cluster status CAS remove operation %d must not contain a value", i)
			}
			if !isClusterStatusCASStatusPath(operation.Path) {
				return fmt.Errorf("cluster status CAS mutation path %q is not allowed", operation.Path)
			}
			statusMutations++
		default:
			return fmt.Errorf("cluster status CAS operation %q is not allowed", operation.Operation)
		}
	}
	if uidTests != 1 || resourceVersionTests != 1 {
		return fmt.Errorf("cluster status CAS patch requires exactly one matching UID and resourceVersion test")
	}
	if statusMutations == 0 {
		return fmt.Errorf("cluster status CAS patch requires at least one status mutation")
	}
	return nil
}

func clusterStatusCASStringValueEquals(value json.RawMessage, expected string) bool {
	var decoded string
	return json.Unmarshal(value, &decoded) == nil && decoded == expected
}

func isClusterStatusCASStatusPath(path string) bool {
	return path == "/status" || strings.HasPrefix(path, "/status/")
}

func findExclusiveClusterStatusCASVertex(dag *graph.DAG) (*clusterStatusCASVertex, error) {
	var found *clusterStatusCASVertex
	for _, vertex := range dag.Vertices() {
		cas, ok := vertex.(*clusterStatusCASVertex)
		if !ok {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("multiple exclusive Cluster status CAS intents")
		}
		found = cas
	}
	return found, nil
}
