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

package playground

import (
	"context"
	"fmt"
	"sync"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var addToScheme sync.Once

func newFactory() cmdutil.Factory {
	getter := genericclioptions.NewConfigFlags(true)

	// Add CRDs to the scheme. They are missing by default.
	addToScheme.Do(func() {
		if err := apiextv1.AddToScheme(scheme.Scheme); err != nil {
			// This should never happen.
			panic(err)
		}
	})
	return cmdutil.NewFactory(getter)
}

func buildClusterInfo(clusterInfo *ClusterInfo, namespace string, name string) error {
	f := newFactory()
	builder := &builder{}
	clientSet, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}
	builder.clientSet = clientSet

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}
	builder.dynamicClient = dynamicClient
	builder.namespace = namespace
	builder.name = name

	// get cluster
	builder.groupKind = schema.GroupKind{Group: "dbaas.infracreate.com", Kind: "Cluster"}
	err = builder.getClusterObject(clusterInfo)
	if err != nil {
		return err
	}

	if clusterInfo.Cluster == nil {
		return fmt.Errorf("failed to find database cluster %s", name)
	}

	// get statefulset
	builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", name)
	builder.groupKind = schema.GroupKind{Kind: "StatefulSet"}
	err = builder.getClusterObject(clusterInfo)
	if err != nil {
		return err
	}

	// get service
	for _, obj := range clusterInfo.StatefulSets {
		builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", obj.Name)
		builder.groupKind = schema.GroupKind{Kind: "Service"}
		err = builder.getClusterObject(clusterInfo)
		if err != nil {
			return err
		}
	}

	// get secret
	builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", name)
	builder.groupKind = schema.GroupKind{Kind: "Secret"}
	err = builder.getClusterObject(clusterInfo)
	if err != nil {
		return err
	}

	// get pod
	for _, obj := range clusterInfo.StatefulSets {
		builder.label = fmt.Sprintf("app.kubernetes.io/instance=%s", obj.Name)
		builder.groupKind = schema.GroupKind{Kind: "Pod"}
		err = builder.getClusterObject(clusterInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

type builder struct {
	clientSet     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	groupKind     schema.GroupKind
	namespace     string
	label         string
	name          string
}

// getClusterObject get kubernetes object belonging to the database cluster
func (b *builder) getClusterObject(clusterObjs *ClusterInfo) error {
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
	case "StatefulSet":
		stss, err := b.clientSet.AppsV1().StatefulSets(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		clusterObjs.StatefulSets = append(clusterObjs.StatefulSets, stss.Items...)
	case "Deployment":
		deps, err := b.clientSet.AppsV1().Deployments(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		clusterObjs.Deployments = append(clusterObjs.Deployments, deps.Items...)
	case "Secret":
		scts, err := b.clientSet.CoreV1().Secrets(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
		clusterObjs.Secrets = append(clusterObjs.Secrets, scts.Items...)
	case "Cluster":
		gvr := schema.GroupVersionResource{Group: b.groupKind.Group, Resource: "clusters", Version: "v1alpha1"}
		obj, err := b.dynamicClient.Resource(gvr).Namespace(b.namespace).Get(ctx, b.name, metav1.GetOptions{}, "")
		if err != nil {
			return err
		}

		cluster := &dbaasv1alpha1.Cluster{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cluster); err != nil {
			return err
		}

		clusterObjs.Cluster = cluster
		return nil
	}

	return nil
}
