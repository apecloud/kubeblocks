package v1alpha1

import (
	"testing"

	. "github.com/onsi/gomega"
	
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestGetInconsistentComponentsInfoWithoutResult(t *testing.T) {
	g := NewGomegaWithT(t)

	// init appVersion
	clusterVersion := &ClusterVersion{}
	clusterVersionYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:     appversion-1
spec:
  clusterDefinitionRef: cluster-definition-1
  components:
  - type: component1
    podSpec: 
      containers:
      - name: container1.a
      - name: container1.b
  - type: component2
    podSpec: 
      containers:
      - name: container2
  - type: component3
    podSpec: 
      containers:
`
	g.Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).To(Succeed())

	// init clusterDef
	clusterDef := &ClusterDefinition{}
	clusterDefYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     cluster-definition-1
spec:
  components:
  - typeName: component1
    podSpec:
      containers:
      - name: container1.c
  - typeName: component2
    podSpec: 
      containers:
  - typeName: component3
    podSpec: 
      containers:
      - name: container3
`
	g.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).To(Succeed())

	notFoundComponentTypes, noContainersComponents := clusterVersion.GetInconsistentComponentsInfo(clusterDef)
	g.Expect(len(notFoundComponentTypes)).To(Equal(0))
	g.Expect(len(noContainersComponents)).To(Equal(0))
}

func TestGetInconsistentComponentsInfoWithResults(t *testing.T) {
	g := NewGomegaWithT(t)

	// init appVersion
	clusterVersion := &ClusterVersion{}
	clusterVersionYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:     appversion-1
spec:
  clusterDefinitionRef: cluster-definition-1
  components:
  - type: component1
    podSpec: 
      containers:
      - name: container1
  - type: component2
    podSpec: 
      containers:
  - type: component3
    podSpec: 
      containers:
      - name: container3
`
	g.Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).To(Succeed())

	// init clusterDef
	clusterDef := &ClusterDefinition{}
	clusterDefYaml := `
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       ClusterDefinition
metadata:
  name:     cluster-definition-1
spec:
  components:
  - typeName: component1
    podSpec:
      containers:
      - name: container1
  - typeName: component2
    podSpec: 
`
	g.Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDef)).To(Succeed())

	notFoundComponentTypes, noContainersComponents := clusterVersion.GetInconsistentComponentsInfo(clusterDef)
	g.Expect(len(notFoundComponentTypes)).To(Equal(1))
	g.Expect(notFoundComponentTypes[0]).To(Equal("component3"))
	g.Expect(len(noContainersComponents)).To(Equal(1))
	g.Expect(noContainersComponents[0]).To(Equal("component2"))
}
