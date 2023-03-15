/*
Copyright ApeCloud, Inc.

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
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestValidateConnectionCredential(t *testing.T) {
	clusterDefByte := `
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-conn-credential
spec:
  connectionCredential:
    endpoint: $(SVC_FQDN):$(SVC_PORT_mysql)
    host: $(SVC_FQDN)
    password: $(RANDOM_PASSWD)
    port: $(SVC_PORT_mysql)
    username: root
  componentDefs:
    - name: replicasets
      workloadType: Replication
      podSpec:
        containers:
          - name: mysql
            imagePullPolicy: IfNotPresent`

	newClusterDef := func(svc string) *ClusterDefinition {
		clusterDef := &ClusterDefinition{}
		_ = yaml.Unmarshal([]byte(clusterDefByte+svc), clusterDef)
		return clusterDef
	}

	{
		// w/o service defined
		clusterDef := newClusterDef("")
		invalidPortNames := clusterDef.ValidateConnectionCredential()
		if len(invalidPortNames) != 0 {
			t.Errorf("Expected empty []invalidPortNames, actual: %s", strings.Join(invalidPortNames, ","))
		}
	}
	{
		// w/ service defined, but no port name
		clusterDef := newClusterDef(`
      service:
        ports:
        - port: 3306`)
		invalidPortNames := clusterDef.ValidateConnectionCredential()
		if len(invalidPortNames) != 2 || invalidPortNames[0] != "$(SVC_PORT_mysql)" {
			t.Errorf("Expected two []invalidPortNames, actual: %s", strings.Join(invalidPortNames, ","))
		}
	}
	{
		// w/ service defined, port name is inconsistent
		clusterDef := newClusterDef(`
      service:
        ports:
        - port: 3306
          name: redis`)
		invalidPortNames := clusterDef.ValidateConnectionCredential()
		if len(invalidPortNames) != 2 || invalidPortNames[0] != "$(SVC_PORT_mysql)" {
			t.Errorf("Expected two []invalidPortNames, actual: %s", strings.Join(invalidPortNames, ","))
		}
	}
	{
		// pass
		clusterDef := newClusterDef(`
      service:
        ports:
        - port: 3306
          name: mysql`)
		invalidPortNames := clusterDef.ValidateConnectionCredential()
		if len(invalidPortNames) != 0 {
			t.Errorf("Expected empty []invalidPortNames, actual: %s", strings.Join(invalidPortNames, ","))
		}
	}
}

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
