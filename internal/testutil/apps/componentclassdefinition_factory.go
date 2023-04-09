package apps

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type MockComponentClassDefinitionFactory struct {
	BaseFactory[appsv1alpha1.ComponentClassDefinition, *appsv1alpha1.ComponentClassDefinition, MockComponentClassDefinitionFactory]
}

func NewComponentClassDefinitionFactory(name, clusterDefinitionRef, componentType string) *MockComponentClassDefinitionFactory {
	f := &MockComponentClassDefinitionFactory{}
	f.init("", name, &appsv1alpha1.ComponentClassDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constant.ClassProviderLabelKey:        "kubeblocks",
				constant.ClusterDefLabelKey:           clusterDefinitionRef,
				constant.KBAppComponentDefRefLabelKey: componentType,
			},
		},
		Spec: appsv1alpha1.ComponentClassDefinitionSpec{
			Groups: []appsv1alpha1.ComponentClassGroup{
				{
					ClassConstraintRef: "kube-class-family-general",
					Template: `
cpu: "{{ or .cpu 1 }}"
memory: "{{ or .memory 4 }}Gi"
storage:
- name: data
  size: "{{ or .dataStorageSize 10 }}Gi"
- name: log
  size: "{{ or .logStorageSize 1 }}Gi"
					`,
					Vars: []string{"cpu", "memory", "dataStorageSize", "logStorageSize"},
					Series: []appsv1alpha1.ComponentClassSeries{
						{
							Name: "general-{{ .cpu }}c{{ .memory }}g",
						},
					},
				},
			},
		},
	}, f)
	return f
}

func (factory *MockComponentClassDefinitionFactory) AddClass(cls appsv1alpha1.ComponentClass) *MockComponentClassDefinitionFactory {
	classes := factory.get().Spec.Groups[0].Series[0].Classes
	classes = append(classes, cls)
	factory.get().Spec.Groups[0].Series[0].Classes = classes
	return factory
}
