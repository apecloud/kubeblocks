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
