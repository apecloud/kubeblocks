/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package parameters

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

func TestComponentParameterReconcileRetriesMissingService(t *testing.T) {
	const namespace = "service-retry"
	template := testparameters.NewComponentTemplateFactory(configSpecName, namespace).
		AddConfigFile(testparameters.MysqlConfigFile, "[mysqld]\netcd_host={{ $.component.serviceReferences.etcd.spec.host.value }}\n").
		GetObject()
	compDef := testapps.NewComponentDefinitionFactory(compDefName).
		SetDefaultSpec().
		AddConfigTemplate(configSpecName, template.Name, namespace, configVolumeName, true).
		AddServiceRef("etcd", "etcd", "3.5").
		GetObject()
	compDef.Status.Phase = appsv1.AvailablePhase
	paramsDef := testparameters.NewParametersDefinitionFactory(paramsDefName).
		SetComponentDefinition(compDef.Name).
		SetTemplateName(configSpecName).
		GetObject()
	paramsDef.Status.Phase = parametersv1alpha1.PDAvailablePhase
	serviceRef := appsv1.ServiceRef{
		Name: "etcd",
		ClusterServiceSelector: &appsv1.ServiceRefClusterSelector{
			Cluster: "etcd",
			Service: &appsv1.ServiceRefServiceSelector{Service: "client", Port: "client"},
		},
	}
	cluster := testapps.NewClusterFactory(namespace, clusterName, "").
		AddComponent(defaultCompName, compDef.Name).
		SetReplicas(1).
		SetServiceRefs([]appsv1.ServiceRef{serviceRef}).
		GetObject()
	comp := testapps.NewComponentFactory(namespace, constant.GenerateClusterComponentName(clusterName, defaultCompName), compDef.Name).
		AddLabels(constant.AppInstanceLabelKey, clusterName).
		AddAnnotations(constant.KBAppClusterUIDKey, "cluster-uid").
		SetReplicas(1).
		GetObject()
	comp.Spec.ServiceRefs = []appsv1.ServiceRef{serviceRef}
	compParam := &parametersv1alpha1.ComponentParameter{
		ObjectMeta: metav1.ObjectMeta{
			Name:       core.GenerateComponentConfigurationName(clusterName, defaultCompName),
			Namespace:  namespace,
			UID:        "parameter-uid",
			Generation: 1,
		},
		Spec: parametersv1alpha1.ComponentParameterSpec{ClusterName: clusterName, ComponentName: defaultCompName},
	}
	reconciler := newComponentParameterRetryReconciler(t, template, compDef, paramsDef, cluster, comp, compParam)
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(compParam)}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("initialize config items: %v", err)
	}
	if err := reconciler.Get(ctx, req.NamespacedName, compParam); err != nil {
		t.Fatal(err)
	}
	if len(compParam.Spec.ConfigItemDetails) != 1 {
		t.Fatalf("expected initialized config item, got %#v", compParam.Spec.ConfigItemDetails)
	}
	_, err := reconciler.Reconcile(ctx, req)
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected missing referenced Service to return a retryable NotFound error, got %v", err)
	}
	configKey := client.ObjectKey{Namespace: namespace, Name: core.GetComponentCfgName(clusterName, defaultCompName, configSpecName)}
	if err := reconciler.Get(ctx, configKey, &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected no rendered config before the referenced Service exists, got %v", err)
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: constant.GenerateClusterServiceName("etcd", "client")},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "client", Port: 2379}}},
	}
	if err := reconciler.Create(ctx, service); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("retry after referenced Service creation: %v", err)
	}
	rendered := &corev1.ConfigMap{}
	if err := reconciler.Get(ctx, configKey, rendered); err != nil {
		t.Fatalf("get rendered config after retry: %v", err)
	}
	if got, want := rendered.Data[testparameters.MysqlConfigFile], "[mysqld]\netcd_host=etcd-client\n"; got != want {
		t.Fatalf("rendered config = %q, want %q", got, want)
	}
}

func TestComponentParameterReconcileMissingTarget(t *testing.T) {
	reconciler := newComponentParameterRetryReconciler(t)
	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: "service-retry", Name: "deleted-component-parameter"},
	})
	if err != nil || result != (ctrl.Result{}) {
		t.Fatalf("missing ComponentParameter should not retry, got result %v and error %v", result, err)
	}
}

func newComponentParameterRetryReconciler(t *testing.T, objects ...client.Object) *ComponentParameterReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, parametersv1alpha1.AddToScheme} {
		if err := addToScheme(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&parametersv1alpha1.ComponentParameter{}).
		WithObjects(objects...).Build()
	return &ComponentParameterReconciler{Client: cli, Scheme: scheme}
}
