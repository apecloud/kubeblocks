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
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestValidateEnabledLogConfigs(t *testing.T) {
	clusterDef := &ClusterDefinition{}
	clusterDefByte := `
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-consensus
spec:
  componentDefs:
    - name: replicasets
      workloadType: Consensus
      logConfigs:
        - name: error
          filePathPattern: /log/mysql/mysqld.err
        - name: slow
          filePathPattern: /log/mysql/*slow.log
      podSpec:
        containers:
          - name: mysql
            imagePullPolicy: IfNotPresent`
	_ = yaml.Unmarshal([]byte(clusterDefByte), clusterDef)
	// normal case
	invalidLogNames := clusterDef.ValidateEnabledLogConfigs("replicasets", []string{"error", "slow"})
	if len(invalidLogNames) != 0 {
		t.Error("Expected empty [] invalidLogNames")
	}
	// corner case
	invalidLogNames1 := clusterDef.ValidateEnabledLogConfigs("replicasets", []string{"error", "slow-test", "audit-test"})
	if len(invalidLogNames1) != 2 {
		t.Error("Expected invalidLogNames are [slow-test, audit-test]")
	}
	// corner case
	invalidLogNames2 := clusterDef.ValidateEnabledLogConfigs("non-exist-type", []string{"error", "slow", "audit"})
	if len(invalidLogNames2) != 3 {
		t.Error("Expected invalidLogNames are [error, slow, audit]")
	}
}

func TestGetComponentDefByName(t *testing.T) {
	componentDefName := "mysqlType"
	clusterDef := &ClusterDefinition{
		Spec: ClusterDefinitionSpec{
			ComponentDefs: []ClusterComponentDefinition{
				{
					Name: componentDefName,
				},
			},
		},
	}
	if clusterDef.GetComponentDefByName(componentDefName) == nil {
		t.Error("function GetComponentDefByName should not return nil")
	}
	componentDefName = "test"
	if clusterDef.GetComponentDefByName(componentDefName) != nil {
		t.Error("function GetComponentDefByName should return nil")
	}
}
