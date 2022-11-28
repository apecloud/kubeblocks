/*
Copyright ApeCloud Inc.

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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/resource"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
)

const valueNone = "<none>"

type ObjectsGetter struct {
	Name          string
	Namespace     string
	ClientSet     clientset.Interface
	DynamicClient dynamic.Interface

	WithAppVersion bool
	WithConfigMap  bool
}

func NewClusterObjects() *ClusterObjects {
	return &ClusterObjects{
		Cluster:    &dbaasv1alpha1.Cluster{},
		ClusterDef: &dbaasv1alpha1.ClusterDefinition{},
		AppVersion: &dbaasv1alpha1.AppVersion{},

		Nodes: []*corev1.Node{},
	}
}

// Get all kubernetes objects belonging to the database cluster
func (o *ObjectsGetter) Get(objs *ClusterObjects) error {
	var err error
	builder := &builder{
		namespace:     o.Namespace,
		name:          o.Name,
		clientSet:     o.ClientSet,
		dynamicClient: o.DynamicClient,
	}

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
	if o.WithAppVersion {
		if err = builder.withGK(types.AppVersionGK()).
			withName(objs.Cluster.Spec.AppVersionRef).
			do(objs); err != nil {
			return err
		}
	}

	// get service
	instLabel := makeInstanceLabel(o.Name)
	if err = builder.withLabel(instLabel).
		withGK(schema.GroupKind{Kind: "Service"}).
		do(objs); err != nil {
		return err
	}

	// get secret
	if err = builder.withLabel(instLabel).
		withGK(schema.GroupKind{Kind: "Secret"}).
		do(objs); err != nil {
		return err
	}

	// get configmap
	if o.WithConfigMap {
		if err = builder.withLabel(instLabel).
			withGK(schema.GroupKind{Kind: "ConfigMap"}).
			do(objs); err != nil {
			return err
		}
	}

	// get pod
	if err = builder.withLabel(instLabel).
		withGK(schema.GroupKind{Kind: "Pod"}).
		do(objs); err != nil {
		return err
	}

	// get nodes where the pods are located
podLoop:
	for _, pod := range objs.Pods.Items {
		for _, node := range objs.Nodes {
			if node.Name == pod.Spec.NodeName {
				break podLoop
			}
		}

		if err = builder.withName(pod.Spec.NodeName).
			withGK(schema.GroupKind{Kind: "Node"}).
			do(objs); err != nil {
			return err
		}
	}

	return nil
}

func makeInstanceLabel(name string) string {
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
func (b *builder) do(clusterObjs *ClusterObjects) error {
	var err error
	ctx := context.TODO()
	listOpts := metav1.ListOptions{
		LabelSelector: b.label,
	}

	kind := b.groupKind.Kind
	switch kind {
	case "Pod":
		clusterObjs.Pods, err = b.clientSet.CoreV1().Pods(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
	case "Service":
		clusterObjs.Services, err = b.clientSet.CoreV1().Services(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
	case "Secret":
		clusterObjs.Secrets, err = b.clientSet.CoreV1().Secrets(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
	case "ConfigMap":
		clusterObjs.ConfigMaps, err = b.clientSet.CoreV1().ConfigMaps(b.namespace).List(ctx, listOpts)
		if err != nil {
			return err
		}
	case "Node":
		if len(b.name) == 0 {
			return nil
		}
		node, err := b.clientSet.CoreV1().Nodes().Get(ctx, b.name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clusterObjs.Nodes = append(clusterObjs.Nodes, node)
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

func getClusterResource(client dynamic.Interface, gvr schema.GroupVersionResource, namespace string, name string, res interface{}) error {
	obj, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{}, "")
	if err != nil {
		return err
	}

	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, res); err != nil {
		return err
	}
	return nil
}

func (o *ClusterObjects) GetClusterInfo() *ClusterInfo {
	c := o.Cluster
	cluster := &ClusterInfo{
		Name:              c.Name,
		Namespace:         c.Namespace,
		AppVersion:        c.Spec.AppVersionRef,
		ClusterDefinition: c.Spec.ClusterDefRef,
		TerminationPolicy: string(c.Spec.TerminationPolicy),
		Status:            string(c.Status.Phase),
		Age:               duration.HumanDuration(time.Since(c.CreationTimestamp.Time)),
		InternalEP:        valueNone,
		ExternalEP:        valueNone,
	}

	primaryComponent := FindCompInCluster(o.Cluster, o.ClusterDef.Spec.Components[0].TypeName)
	internalEndpoints, externalEndpoints := GetClusterEndpoints(o.Services, primaryComponent)
	if len(internalEndpoints) > 0 {
		cluster.InternalEP = strings.Join(internalEndpoints, ",")
	}
	if len(externalEndpoints) > 0 {
		cluster.ExternalEP = strings.Join(externalEndpoints, ",")
	}
	return cluster
}

func (o *ClusterObjects) GetComponentInfo() []*ComponentInfo {
	var comps []*ComponentInfo

	for _, compInClusterDef := range o.ClusterDef.Spec.Components {
		c := FindCompInCluster(o.Cluster, compInClusterDef.TypeName)
		if c == nil {
			return nil
		}

		replicas := c.Replicas
		if replicas == 0 {
			replicas = compInClusterDef.DefaultReplicas
		}

		var pods []*corev1.Pod
		for _, p := range o.Pods.Items {
			if n, ok := p.Labels[types.ComponentLabelKey]; ok && n == c.Name {
				pods = append(pods, &p)
			}
		}

		image := valueNone
		if len(pods) > 0 {
			image = pods[0].Spec.Containers[0].Image
		}

		running, waiting, succeeded, failed := util.GetPodStatus(pods)
		comp := &ComponentInfo{
			Name:     c.Name,
			Type:     c.Type,
			Cluster:  o.Cluster.Name,
			Replicas: fmt.Sprintf("%d / %d", replicas, len(pods)),
			Status:   fmt.Sprintf("%d / %d / %d / %d ", running, waiting, succeeded, failed),
			Image:    image,
		}
		comps = append(comps, comp)
	}
	return comps
}

func (o *ClusterObjects) GetInstanceInfo() []*InstanceInfo {
	var instances []*InstanceInfo

	for _, pod := range o.Pods.Items {
		instance := &InstanceInfo{
			Name:       pod.Name,
			Cluster:    getLabelVal(pod.Labels, types.InstanceLabelKey),
			Component:  getLabelVal(pod.Labels, types.ComponentLabelKey),
			Status:     string(pod.Status.Phase),
			Role:       getLabelVal(pod.Labels, types.ConsensusSetRoleLabelKey),
			AccessMode: getLabelVal(pod.Labels, types.ConsensusSetAccessModeLabelKey),
			Age:        duration.HumanDuration(time.Since(pod.CreationTimestamp.Time)),
		}

		var component *dbaasv1alpha1.ClusterComponent
		for i, c := range o.Cluster.Spec.Components {
			if c.Name == instance.Component {
				component = &o.Cluster.Spec.Components[i]
			}
		}
		getInstanceStorageInfo(component, instance)
		getInstanceNodeInfo(o.Nodes, &pod, instance)
		getInstanceResourceInfo(&pod, instance)
		instances = append(instances, instance)
	}
	return instances
}

func getInstanceStorageInfo(component *dbaasv1alpha1.ClusterComponent, i *InstanceInfo) {
	if component == nil {
		i.Storage = valueNone
		return
	}

	var volumes []string
	vcTmpls := component.VolumeClaimTemplates
	for _, vcTmpl := range vcTmpls {
		val := vcTmpl.Spec.Resources.Requests[corev1.ResourceStorage]
		volumes = append(volumes, fmt.Sprintf("%s/%s", vcTmpl.Name, val.String()))
	}
	i.Storage = strings.Join(volumes, ",")
}

func getInstanceNodeInfo(nodes []*corev1.Node, pod *corev1.Pod, i *InstanceInfo) {
	i.Node = valueNone
	i.Region = valueNone
	i.AZ = valueNone

	if pod.Spec.NodeName == "" {
		return
	}

	i.Node = strings.Join([]string{pod.Spec.NodeName, pod.Status.HostIP}, "/")
	node := util.GetNodeByName(nodes, pod.Spec.NodeName)
	if node == nil {
		return
	}

	i.Region = getLabelVal(node.Labels, types.RegionLabelKey)
	i.AZ = getLabelVal(node.Labels, types.ZoneLabelKey)
}

func getInstanceResourceInfo(pod *corev1.Pod, i *InstanceInfo) {
	reqs, limits := resource.PodRequestsAndLimits(pod)
	names := []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory}
	for _, name := range names {
		res := valueNone
		limit := limits[name]
		req := reqs[name]

		// if both limit and request are empty, only output none
		if !util.ResourceIsEmpty(&limit) || !util.ResourceIsEmpty(&req) {
			res = fmt.Sprintf("%s / %s", req.String(), limit.String())
		}

		switch name {
		case corev1.ResourceCPU:
			i.CPU = res
		case corev1.ResourceMemory:
			i.Memory = res
		}
	}
}

func getLabelVal(labels map[string]string, key string) string {
	val := labels[key]
	if len(val) == 0 {
		return valueNone
	}
	return val
}
