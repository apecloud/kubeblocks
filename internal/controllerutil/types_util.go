package controllerutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func GetNamespacedName(obj metav1.Object) types.NamespacedName {
	if obj == nil {
		return types.NamespacedName{}
	}

	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}
