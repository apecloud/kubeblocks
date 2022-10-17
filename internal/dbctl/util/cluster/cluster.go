/*
Copyright 2022 The KubeBlocks Authors

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

package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

func NewClusterObjects() *types.ClusterObjects {
	return &types.ClusterObjects{
		Cluster:    &dbaasv1alpha1.Cluster{},
		ClusterDef: &dbaasv1alpha1.ClusterDefinition{},
		AppVersion: &dbaasv1alpha1.AppVersion{},
		Pods:       []corev1.Pod{},
		Services:   []corev1.Service{},
		Secrets:    []corev1.Secret{},
	}
}

// GetAllObjects get all kubernetes objects belonging to the database cluster
func GetAllObjects(clientSet clientset.Interface, dynamicClient dynamic.Interface, namespace string, name string, objs *types.ClusterObjects) error {
	var err error
	builder := &builder{
		namespace:     namespace,
		name:          name,
		clientSet:     clientSet,
		dynamicClient: dynamicClient,
	}

	// get cluster
	if err = builder.withGK(types.ClusterGK()).
		do(objs); err != nil {
		return err
	}

	// get cluster definition
	if err = builder.withGK(types.ClusterDefGK()).
		withName(objs.Cluster.Spec.ClusterDefRef).
		do(objs); err != nil {
		return err
	}

	// get appversion
	if err = builder.withGK(types.AppVersionGR()).
		withName(objs.Cluster.Spec.AppVersionRef).
		do(objs); err != nil {
		return err
	}

	// get service
	if err = builder.withLabel(InstanceLabel(name)).
		withGK(schema.GroupKind{Kind: "Service"}).
		do(objs); err != nil {
		return err
	}

	// get secret
	if err = builder.withLabel(InstanceLabel(name)).
		withGK(schema.GroupKind{Kind: "Secret"}).
		do(objs); err != nil {
		return err
	}

	// get pod
	if err = builder.withLabel(InstanceLabel(name)).
		withGK(schema.GroupKind{Kind: "Pod"}).
		do(objs); err != nil {
		return err
	}

	return nil
}

func InstanceLabel(name string) string {
	return fmt.Sprintf("%s=%s", types.InstanceLabelKey, name)
}

type builder struct {
	namespace string
	label     string
	name      string

	clientSet     clientset.Interface
	dynamicClient dynamic.Interface
	groupKind     schema.GroupKind
}

// Do get kubernetes object belonging to the database cluster
func (b *builder) do(clusterObjs *types.ClusterObjects) error {
	ctx := context.Background()
	listOpts := metav1.ListOptions{
		LabelSelector: b.label,
	}

	kind := b.groupKind.Kind
	switch kind {
	case "Pod":
		pods, err := b.clientSet.CoreV1().Pods(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		clusterObjs.Pods = append(clusterObjs.Pods, pods.Items...)
	case "Service":
		svrs, err := b.clientSet.CoreV1().Services(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		clusterObjs.Services = append(clusterObjs.Services, svrs.Items...)
	case "Secret":
		scts, err := b.clientSet.CoreV1().Secrets(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		clusterObjs.Secrets = append(clusterObjs.Secrets, scts.Items...)
	case types.KindCluster:
		return getClusterResource(b.dynamicClient, types.ClusterGVR(), b.namespace, b.name, clusterObjs.Cluster)
	case types.KindClusterDef:
		// ClusterDefinition is cluster scope, so namespace is empty
		return getClusterResource(b.dynamicClient, types.ClusterDefGVR(), "", b.name, clusterObjs.ClusterDef)
	case types.KindAppVersion:
		// AppVersion is cluster scope, so namespace is empty
		return getClusterResource(b.dynamicClient, types.AppVersionGVR(), "", b.name, clusterObjs.AppVersion)
	}

	return nil
}

func (b *builder) withLabel(l string) *builder {
	b.label = l
	return b
}

func (b *builder) withGK(gk schema.GroupKind) *builder {
	b.groupKind = gk
	return b
}

func (b *builder) withName(name string) *builder {
	b.name = name
	return b
}

func getClusterResource(client dynamic.Interface, gvr schema.GroupVersionResource, name string, namespace string, res interface{}) error {
	obj, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{}, "")
	if err != nil {
		return err
	}

	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, res); err != nil {
		return err
	}
	return nil
}
