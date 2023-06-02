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
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestGetInconsistentComponentsInfoWithoutResult(t *testing.T) {
	g := NewGomegaWithT(t)

	// init ClusterVersion
	clusterVersion := &ClusterVersion{}
	clusterVersionYaml := `
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: clusterversion-1
spec:
  clusterDefinitionRef: cluster-definition-1
  componentVersions:
  - componentDefRef: component1
    versionsContext:
      containers:
      - name: container1.a
      - name: container1.b
  - componentDefRef: component2
    versionsContext:
      containers:
      - name: container2
  - componentDefRef: component3
    versionsContext:
      containers:
`
	g.Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).To(Succeed())

	// init clusterDef
	clusterDef := &ClusterDefinition{}
	clusterDefYaml := `
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-1
spec:
  componentDefs:
  - name: component1
    podSpec:
      containers:
      - name: container1.c
  - name: component2
    podSpec:
      containers:
  - name: component3
    podSpec:
      containers:
      - name: container3
`
	g.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).To(Succeed())

	notFoundComponentDefNames, noContainersComponents := clusterVersion.GetInconsistentComponentsInfo(clusterDef)
	g.Expect(len(notFoundComponentDefNames)).To(Equal(0))
	g.Expect(len(noContainersComponents)).To(Equal(0))
}

func TestGetInconsistentComponentsInfoWithResults(t *testing.T) {
	g := NewGomegaWithT(t)

	// init clusterVersion
	clusterVersion := &ClusterVersion{}
	clusterVersionYaml := `
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: clusterversion-2
spec:
  clusterDefinitionRef: cluster-definition-2
  componentVersions:
  - componentDefRef: component1
    versionsContext:
      containers:
      - name: container1
  - componentDefRef: component2
    versionsContext:
      containers:
  - componentDefRef: component3
    versionsContext:
      containers:
      - name: container3
`
	g.Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).To(Succeed())

	// init clusterDef
	clusterDef := &ClusterDefinition{}
	clusterDefYaml := `
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: cluster-definition-2
spec:
  componentDefs:
  - name: component1
    podSpec:
      containers:
      - name: container1
  - name: component2
    podSpec: 
`
	g.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).To(Succeed())

	notFoundComponentDefNames, noContainersComponents := clusterVersion.GetInconsistentComponentsInfo(clusterDef)
	g.Expect(len(notFoundComponentDefNames)).To(Equal(1))
	g.Expect(notFoundComponentDefNames[0]).To(Equal("component3"))
	g.Expect(len(noContainersComponents)).To(Equal(1))
	g.Expect(noContainersComponents[0]).To(Equal("component2"))
}
