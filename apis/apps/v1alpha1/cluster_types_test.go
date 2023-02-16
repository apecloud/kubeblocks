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
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestValidateEnabledLogs(t *testing.T) {
	cluster := &Cluster{}
	clusterDef := &ClusterDefinition{}
	clusterByte := `
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql
spec:
  clusterVersionRef: cluster-version-consensus
  clusterDefinitionRef: cluster-definition-consensus
  componentSpecs:
    - name: wesql-test
      componentDefRef: replicasets
      enabledLogs: [error, slow]
`
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
	_ = yaml.Unmarshal([]byte(clusterByte), cluster)
	_ = yaml.Unmarshal([]byte(clusterDefByte), clusterDef)
	// normal case
	if err := cluster.ValidateEnabledLogs(clusterDef); err != nil {
		t.Error("Expected empty conditionList")
	}
	// corner case
	cluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error-test", "slow"}
	if err := cluster.ValidateEnabledLogs(clusterDef); err == nil {
		t.Error("Expected one element conditionList")
	}
}

func TestGetMessage(t *testing.T) {
	podKey := "Pod/test-01"
	statusComponent := ClusterStatusComponent{
		Message: map[string]string{
			podKey: "failed Scheduled",
		},
	}
	message := statusComponent.GetMessage()
	message[podKey] = "insufficient cpu"
	if statusComponent.Message[podKey] == message[podKey] {
		t.Error("Expected status component message not changed")
	}
}

func TestSetMessage(t *testing.T) {
	podKey := "Pod/test-01"
	statusComponent := ClusterStatusComponent{}
	statusComponent.SetMessage(
		map[string]string{
			podKey: "failed Scheduled",
		})
	if statusComponent.Message[podKey] != "failed Scheduled" {
		t.Error(`Expected get message "failed Scheduled"`)
	}
}

func TestSetAndGetObjectMessage(t *testing.T) {
	messageMap := ComponentMessageMap{
		"Pod/test-01": "failed Scheduled",
	}
	messageMap.SetObjectMessage("Pod", "test-01", "insufficient cpu")
	message := messageMap.GetObjectMessage("Pod", "test-01")
	if message != "insufficient cpu" {
		t.Error(`Expected get message "insufficient cpu"`)
	}
}

func TestGetComponentOrTypeName(t *testing.T) {
	var (
		componentType = "mysqlType"
		componentName = "mysql"
	)
	cluster := Cluster{
		Spec: ClusterSpec{
			ComponentSpecs: []ClusterComponent{
				{Name: componentName, ComponentDefRef: componentType},
			},
		},
	}
	typeName := cluster.GetComponentTypeName(componentName)
	if typeName != componentType {
		t.Errorf(`function GetComponentTypeName should return %s`, componentType)
	}
	component := cluster.GetComponentByName(componentName)
	if component == nil {
		t.Errorf("function GetComponentByName should not return nil")
	}
	componentName = "mysql1"
	typeName = cluster.GetComponentTypeName(componentName)
	if typeName != "" {
		t.Errorf(`function GetComponentTypeName should return ""`)
	}
	component = cluster.GetComponentByName(componentName)
	if component != nil {
		t.Error("function GetComponentByName should return nil")
	}
}
