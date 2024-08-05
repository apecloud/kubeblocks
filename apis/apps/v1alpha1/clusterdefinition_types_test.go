/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
)

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

var _ = Describe("", func() {

	It("test GetTerminalPhases", func() {
		r := ClusterDefinitionStatus{}
		Expect(r.GetTerminalPhases()).Should(ContainElement(AvailablePhase))
	})

	It("test GetStatefulSetWorkload", func() {
		r := &ClusterComponentDefinition{}
		r.WorkloadType = Stateless
		Expect(r.GetStatefulSetWorkload()).Should(BeNil())
		r.WorkloadType = Stateful
		Expect(r.GetStatefulSetWorkload()).Should(BeEquivalentTo(r.StatefulSpec))
		r.WorkloadType = Consensus
		Expect(r.GetStatefulSetWorkload()).Should(BeEquivalentTo(r.ConsensusSpec))
		r.WorkloadType = Replication
		Expect(r.GetStatefulSetWorkload()).Should(BeEquivalentTo(r.ReplicationSpec))
	})

	It("test GetCommonStatefulSpec", func() {
		r := &ClusterComponentDefinition{}
		r.WorkloadType = Stateful
		r.StatefulSpec = &StatefulSetSpec{}
		spec, err := r.GetCommonStatefulSpec()
		Expect(err).Should(BeNil())
		Expect(spec).Should(BeEquivalentTo(r.StatefulSpec))
		r.WorkloadType = Consensus
		r.ConsensusSpec = &ConsensusSetSpec{
			StatefulSetSpec: StatefulSetSpec{},
		}
		spec, err = r.GetCommonStatefulSpec()
		Expect(err).Should(BeNil())
		Expect(spec).Should(BeEquivalentTo(&r.ConsensusSpec.StatefulSetSpec))
		r.WorkloadType = Replication
		r.ReplicationSpec = &ReplicationSetSpec{
			StatefulSetSpec: StatefulSetSpec{},
		}
		spec, err = r.GetCommonStatefulSpec()
		Expect(err).Should(BeNil())
		Expect(spec).Should(BeEquivalentTo(&r.ReplicationSpec.StatefulSetSpec))
	})

	It("test GetUpdateStrategy", func() {
		r := &StatefulSetSpec{}
		r.UpdateStrategy = BestEffortParallelStrategy
		Expect(r.GetUpdateStrategy()).Should(Equal(r.UpdateStrategy))
	})

	It("test finalStsUpdateStrategy", func() {
		r := &StatefulSetSpec{}
		r.UpdateStrategy = ParallelStrategy
		policyType, strategy := r.finalStsUpdateStrategy()
		Expect(policyType).Should(BeEquivalentTo(appsv1.ParallelPodManagement))
		Expect(strategy.Type).Should(BeEquivalentTo(appsv1.RollingUpdateStatefulSetStrategyType))
		r.UpdateStrategy = SerialStrategy
		policyType, strategy = r.finalStsUpdateStrategy()
		Expect(policyType).Should(BeEquivalentTo(appsv1.OrderedReadyPodManagement))
		Expect(strategy.Type).Should(BeEquivalentTo(appsv1.RollingUpdateStatefulSetStrategyType))
		Expect(strategy.RollingUpdate.MaxUnavailable.IntValue()).Should(BeEquivalentTo(1))
	})

	It("test consensus GetUpdateStrategy", func() {
		r := &ConsensusSetSpec{}
		r.UpdateStrategy = BestEffortParallelStrategy
		Expect(r.GetUpdateStrategy()).Should(Equal(r.UpdateStrategy))
	})

	It("test consensus FinalStsUpdateStrategy", func() {
		r := ConsensusSetSpec{}
		policyType, strategy := r.FinalStsUpdateStrategy()
		Expect(policyType).Should(BeEquivalentTo(appsv1.ParallelPodManagement))
		Expect(strategy.Type).Should(BeEquivalentTo(appsv1.OnDeleteStatefulSetStrategyType))
	})

	It("test NewConsensusSetSpec", func() {
		Expect(NewConsensusSetSpec()).ShouldNot(BeNil())
	})

	It("test replication GetUpdateStrategy", func() {
		r := &ReplicationSetSpec{}
		r.UpdateStrategy = BestEffortParallelStrategy
		Expect(r.GetUpdateStrategy()).Should(Equal(r.UpdateStrategy))
	})

	It("test replication FinalStsUpdateStrategy", func() {
		r := ReplicationSetSpec{}
		policyType, strategy := r.FinalStsUpdateStrategy()
		Expect(policyType).Should(BeEquivalentTo(appsv1.ParallelPodManagement))
		Expect(strategy.Type).Should(BeEquivalentTo(appsv1.OnDeleteStatefulSetStrategyType))
	})
})
