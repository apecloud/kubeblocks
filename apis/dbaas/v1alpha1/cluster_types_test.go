/*
Copyright ApeCloud Inc.

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
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestValidateEnabledLogs(t *testing.T) {
	cluster := &Cluster{}
	clusterDef := &ClusterDefinition{}
	clusterByte := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql
spec:
  appVersionRef: app-version-consensus
  clusterDefinitionRef: cluster-definition-consensus
  components:
    - name: wesql-test
      type: replicasets
      enabledLogs: [error, slow]
`
	clusterDefByte := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-consensus
spec:
  type: state.mysql-8
  components:
    - typeName: replicasets
      componentType: Consensus
      logConfigs:
        - name: error
          filePathPattern: /log/mysql/mysqld.err
        - name: slow
          filePathPattern: /log/mysql/*slow.log
      podSpec:
        containers:
          - name: mysql
            imagePullPolicy: IfNotPresent`
	_ = yaml.Unmarshal([]byte(clusterByte), cluster)
	_ = yaml.Unmarshal([]byte(clusterDefByte), clusterDef)
	// normal case
	conditionList := cluster.ValidateEnabledLogs(clusterDef)
	if len(conditionList) != 0 {
		t.Error("Expected empty conditionList")
	}
	// corner case
	cluster.Spec.Components[0].EnabledLogs = []string{"error-test", "slow"}
	conditionList1 := cluster.ValidateEnabledLogs(clusterDef)
	if len(conditionList1) != 1 {
		t.Error("Expected one element conditionList")
	}
}
