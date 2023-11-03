package class

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/class"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// GetManager gets a class manager which manages default classes and user custom classes
func GetManager(client dynamic.Interface, cdName string) (*class.Manager, error) {
	selector := fmt.Sprintf("%s=%s,%s", constant.ClusterDefLabelKey, cdName, types.ClassProviderLabelKey)
	classObjs, err := client.Resource(types.ComponentClassDefinitionGVR()).Namespace("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	var classDefinitionList v1alpha1.ComponentClassDefinitionList
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(classObjs.UnstructuredContent(), &classDefinitionList); err != nil {
		return nil, err
	}

	constraintObjs, err := client.Resource(types.ComponentResourceConstraintGVR()).Namespace("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var resourceConstraintList v1alpha1.ComponentResourceConstraintList
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(constraintObjs.UnstructuredContent(), &resourceConstraintList); err != nil {
		return nil, err
	}
	return class.NewManager(classDefinitionList, resourceConstraintList)
}

// GetResourceConstraints gets all resource constraints
func GetResourceConstraints(dynamic dynamic.Interface) (map[string]*v1alpha1.ComponentResourceConstraint, error) {
	objs, err := dynamic.Resource(types.ComponentResourceConstraintGVR()).List(context.TODO(), metav1.ListOptions{
		// LabelSelector: types.ResourceConstraintProviderLabelKey,
	})
	if err != nil {
		return nil, err
	}
	var constraintsList v1alpha1.ComponentResourceConstraintList
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(objs.UnstructuredContent(), &constraintsList); err != nil {
		return nil, err
	}

	result := make(map[string]*v1alpha1.ComponentResourceConstraint)
	for idx := range constraintsList.Items {
		cf := constraintsList.Items[idx]
		if _, ok := cf.GetLabels()[constant.ResourceConstraintProviderLabelKey]; !ok {
			continue
		}
		result[cf.GetName()] = &cf
	}
	return result, nil
}
