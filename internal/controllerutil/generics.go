package controllerutil

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Object a generic representation of various resource object types
type Object interface{}

// PObject pointer of Object
type PObject[T Object] interface {
	*T
	client.Object
	DeepCopy() *T // DeepCopy have a pointer receiver
}

// ObjList a generic representation of various resource list object types
type ObjList[T Object] interface{}

// PObjList pointer of ObjList
type PObjList[T Object, L ObjList[T]] interface {
	*L
	client.ObjectList
}

// ObjListWrapper A wrapper of resource objects, since golang generics currently
// doesn't support fields access use a workaround mentioned in https://github.com/golang/go/issues/48522
type ObjListWrapper[T Object, L ObjList[T]] interface {
	GetItems(l *L) []T
}

// SecretListWrapper ObjListWrapper of corev1.SecretList
type SecretListWrapper struct{}

func (w SecretListWrapper) GetItems(list *corev1.SecretList) []corev1.Secret {
	return list.Items
}

// ServiceListWrapper ObjListWrapper of corev1.ServiceList
type ServiceListWrapper struct{}

func (w ServiceListWrapper) GetItems(list *corev1.ServiceList) []corev1.Service {
	return list.Items
}

// StatefulSetListWrapper ObjListWrapper of appsv1.StatefulSetList
type StatefulSetListWrapper struct{}

func (w StatefulSetListWrapper) GetItems(list *appsv1.StatefulSetList) []appsv1.StatefulSet {
	return list.Items
}

// DeploymentListWrapper ObjListWrapper of appsv1.DeploymentList
type DeploymentListWrapper struct{}

func (w DeploymentListWrapper) GetItems(list *appsv1.DeploymentList) []appsv1.Deployment {
	return list.Items
}

// ConfigMapListWrapper ObjListWrapper of corev1.ConfigMapList
type ConfigMapListWrapper struct{}

func (w ConfigMapListWrapper) GetItems(list *corev1.ConfigMapList) []corev1.ConfigMap {
	return list.Items
}

// PodDisruptionBudgetListWrapper ObjListWrapper of policyv1.PodDisruptionBudgetList
type PodDisruptionBudgetListWrapper struct{}

func (w PodDisruptionBudgetListWrapper) GetItems(list *policyv1.PodDisruptionBudgetList) []policyv1.PodDisruptionBudget {
	return list.Items
}
