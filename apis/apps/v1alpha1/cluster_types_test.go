/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("", func() {
	It("test GetMinAvailable", func() {
		prefer := intstr.IntOrString{}
		clusterCompSpec := &ClusterComponentSpec{}
		Expect(clusterCompSpec.GetMinAvailable(nil)).Should(BeNil())
		clusterCompSpec.NoCreatePDB = false
		clusterCompSpec.Replicas = 1
		Expect(clusterCompSpec.GetMinAvailable(&prefer).IntVal).Should(BeEquivalentTo(0))
		clusterCompSpec.Replicas = 2
		Expect(clusterCompSpec.GetMinAvailable(&prefer).IntVal).Should(BeEquivalentTo(1))
		clusterCompSpec.Replicas = 3
		Expect(clusterCompSpec.GetMinAvailable(&prefer)).Should(BeEquivalentTo(&prefer))
	})

	It("test toVolumeClaimTemplate", func() {
		r := ClusterComponentVolumeClaimTemplate{}
		r.Name = "test-name"
		Expect(r.toVolumeClaimTemplate().ObjectMeta.Name).Should(BeEquivalentTo(r.Name))
	})

	It("test ToV1PersistentVolumeClaimSpec", func() {
		r := PersistentVolumeClaimSpec{}
		pvcSpec := r.ToV1PersistentVolumeClaimSpec()
		Expect(pvcSpec.AccessModes).Should(BeEquivalentTo(r.AccessModes))
		Expect(pvcSpec.Resources).Should(BeEquivalentTo(r.Resources))
		Expect(pvcSpec.StorageClassName).Should(BeEquivalentTo(r.getStorageClassName(viper.GetString(constant.CfgKeyDefaultStorageClass))))
		Expect(pvcSpec.VolumeMode).Should(BeEquivalentTo(r.VolumeMode))
	})

	It("test ToV1PersistentVolumeClaimSpec with default storage class", func() {
		scName := "test-sc"
		viper.Set(constant.CfgKeyDefaultStorageClass, scName)
		r := PersistentVolumeClaimSpec{}
		pvcSpec := r.ToV1PersistentVolumeClaimSpec()
		Expect(pvcSpec.StorageClassName).Should(BeEquivalentTo(&scName))
		viper.Set(constant.CfgKeyDefaultStorageClass, "")
	})

	It("test ToV1PersistentVolumeClaimSpec with volume mode", func() {
		for _, mode := range []corev1.PersistentVolumeMode{corev1.PersistentVolumeBlock, corev1.PersistentVolumeFilesystem} {
			r := PersistentVolumeClaimSpec{
				VolumeMode: &mode,
			}
			pvcSpec := r.ToV1PersistentVolumeClaimSpec()
			Expect(pvcSpec.VolumeMode).Should(BeEquivalentTo(r.VolumeMode))
		}
	})

	It("test getStorageClassName", func() {
		preferSC := "prefer-sc"
		r := PersistentVolumeClaimSpec{}
		r.StorageClassName = nil
		Expect(r.getStorageClassName(preferSC)).Should(BeEquivalentTo(&preferSC))
		scName := "test-sc"
		r.StorageClassName = &scName
		Expect(r.getStorageClassName(preferSC)).Should(BeEquivalentTo(&scName))
	})

	It("test IsDeleting", func() {
		r := Cluster{}
		Expect(r.IsDeleting()).Should(Equal(false))

		r.Spec.TerminationPolicy = DoNotTerminate
		Expect(r.IsDeleting()).Should(Equal(false))

		r.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		r.Spec.TerminationPolicy = ""
		Expect(r.IsDeleting()).Should(Equal(true))

		r.Spec.TerminationPolicy = DoNotTerminate
		Expect(r.IsDeleting()).Should(Equal(false))
	})

	It("test IsUpdating", func() {
		r := Cluster{}
		r.Status = ClusterStatus{
			ObservedGeneration: int64(0),
		}
		r.Generation = 1
		Expect(r.IsUpdating()).Should(Equal(true))
	})

	It("test IsStatusUpdating", func() {
		r := Cluster{}
		r.Status = ClusterStatus{
			ObservedGeneration: int64(1),
		}
		r.Generation = 1
		Expect(r.IsStatusUpdating()).Should(Equal(true))
	})

	It("test GetVolumeClaimNames", func() {
		r := Cluster{}
		clusterName := "test-cluster"
		r.Name = clusterName
		Expect(r.GetVolumeClaimNames("")).Should(BeNil())
		compName := "test-comp"
		comp := ClusterComponentSpec{}
		comp.Name = compName
		r.Spec.ComponentSpecs = []ClusterComponentSpec{
			comp,
		}
		Expect(r.GetVolumeClaimNames("")).Should(BeNil())
		comp.VolumeClaimTemplates = []ClusterComponentVolumeClaimTemplate{
			{
				Name: compName,
				Spec: PersistentVolumeClaimSpec{},
			},
		}
		comp.Replicas = 1
		r.Spec.ComponentSpecs = []ClusterComponentSpec{
			comp,
		}
		Expect(r.GetVolumeClaimNames(compName)).ShouldNot(BeNil())
		Expect(r.GetVolumeClaimNames(compName)).Should(ContainElement(fmt.Sprintf("%s-%s-%s-%d", compName, r.Name, compName, 0)))
	})

	It("test GetDefNameMappingComponents", func() {
		r := ClusterSpec{}
		key := "comp-def-ref"
		comp := ClusterComponentSpec{}
		comp.ComponentDefRef = key
		r.ComponentSpecs = []ClusterComponentSpec{comp}
		Expect(r.GetDefNameMappingComponents()[key]).Should(ContainElement(comp))
	})

	It("test SetComponentStatus", func() {
		r := ClusterStatus{}
		status := ClusterComponentStatus{}
		compName := "test-comp"
		r.Components = map[string]ClusterComponentStatus{
			compName: {
				Phase:                "",
				Message:              nil,
				PodsReady:            nil,
				PodsReadyTime:        nil,
				ConsensusSetStatus:   nil,
				ReplicationSetStatus: nil,
			},
		}
		r.SetComponentStatus(compName, status)
		Expect(r.Components[compName]).Should(Equal(status))
	})

	It("test checkedInitComponentsMap", func() {
		r := ClusterStatus{}
		r.Components = nil
		r.checkedInitComponentsMap()
		Expect(r.Components).ShouldNot(BeNil())
	})

	It("test ToVolumeClaimTemplates", func() {
		r := ClusterComponentSpec{}
		r.VolumeClaimTemplates = []ClusterComponentVolumeClaimTemplate{{
			Name: "",
			Spec: PersistentVolumeClaimSpec{},
		}}
		Expect(r.ToVolumeClaimTemplates()).Should(HaveLen(1))
	})

	It("test GetClusterUpRunningPhases", func() {
		Expect(GetClusterUpRunningPhases()).Should(ContainElements([]ClusterPhase{
			RunningClusterPhase,
			AbnormalClusterPhase,
			FailedClusterPhase,
		}))
	})

	It("GetComponentTerminalPhases", func() {
		Expect(GetComponentTerminalPhases()).Should(ContainElements([]ClusterComponentPhase{
			RunningClusterCompPhase,
			StoppedClusterCompPhase,
			FailedClusterCompPhase,
			AbnormalClusterCompPhase,
		}))
	})

	It("GetComponentUpRunningPhase", func() {
		Expect(GetComponentUpRunningPhase()).Should(ContainElements([]ClusterComponentPhase{
			RunningClusterCompPhase,
			AbnormalClusterCompPhase,
			FailedClusterCompPhase,
		}))
	})

	It("ComponentPodsAreReady", func() {
		ready := true
		Expect(ComponentPodsAreReady(&ready)).Should(BeTrue())
	})
})

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
	if err := cluster.Spec.ValidateEnabledLogs(clusterDef); err != nil {
		t.Error("Expected empty conditionList")
	}
	// corner case
	cluster.Spec.ComponentSpecs[0].EnabledLogs = []string{"error-test", "slow"}
	if err := cluster.Spec.ValidateEnabledLogs(clusterDef); err == nil {
		t.Error("Expected one element conditionList")
	}
}

func TestGetMessage(t *testing.T) {
	podKey := "Pod/test-01"
	compStatus := ClusterComponentStatus{
		Message: map[string]string{
			podKey: "failed Scheduled",
		},
	}
	message := compStatus.GetMessage()
	message[podKey] = "insufficient cpu"
	if compStatus.Message[podKey] == message[podKey] {
		t.Error("Expected component status message not changed")
	}
}

func TestSetMessage(t *testing.T) {
	podKey := "Pod/test-01"
	compStatus := ClusterComponentStatus{}
	compStatus.SetMessage(
		map[string]string{
			podKey: "failed Scheduled",
		})
	if compStatus.Message[podKey] != "failed Scheduled" {
		t.Error(`Expected get message "failed Scheduled"`)
	}
}

func TestSetAndGetObjectMessage(t *testing.T) {
	componentStatus := ClusterComponentStatus{}
	val := "insufficient cpu"
	componentStatus.SetObjectMessage("Pod", "test-01", val)
	message := componentStatus.GetObjectMessage("Pod", "test-01")
	if message != val {
		t.Errorf(`Expected get message "%s"`, val)
	}
}

func TestSetObjectMessage(t *testing.T) {
	componentStatus := ClusterComponentStatus{}
	messageMap := ComponentMessageMap{
		"Pod/test-01": "failed Scheduled",
	}
	val := "insufficient memory"
	messageMap.SetObjectMessage("Pod", "test-01", val)
	componentStatus.SetMessage(messageMap)
	if componentStatus.GetObjectMessage("Pod", "test-01") != val {
		t.Errorf(`Expected get message "%s"`, val)
	}
}

func TestGetComponentOrName(t *testing.T) {
	var (
		componentDefName = "mysqlType"
		componentName    = "mysql"
	)
	cluster := Cluster{
		Spec: ClusterSpec{
			ComponentSpecs: []ClusterComponentSpec{
				{Name: componentName, ComponentDefRef: componentDefName},
			},
		},
	}
	compDefName := cluster.Spec.GetComponentDefRefName(componentName)
	if compDefName != componentDefName {
		t.Errorf(`function GetComponentDefRefName should return %s`, componentDefName)
	}
	component := cluster.Spec.GetComponentByName(componentName)
	if component == nil {
		t.Errorf("function GetComponentByName should not return nil")
	}
	componentName = "mysql1"
	compDefName = cluster.Spec.GetComponentDefRefName(componentName)
	if compDefName != "" {
		t.Errorf(`function GetComponentDefRefName should return ""`)
	}
	component = cluster.Spec.GetComponentByName(componentName)
	if component != nil {
		t.Error("function GetComponentByName should return nil")
	}
}
