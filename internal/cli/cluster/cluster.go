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
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const valueNone = "<none>"

type ObjectsGetter struct {
	ClientSet     clientset.Interface
	DynamicClient dynamic.Interface

	Name      string
	Namespace string

	WithClusterDef     bool
	WithClusterVersion bool
	WithConfigMap      bool
	WithPVC            bool
	WithService        bool
	WithSecret         bool
	WithPod            bool
}

func NewClusterObjects() *ClusterObjects {
	return &ClusterObjects{
		Cluster:        &dbaasv1alpha1.Cluster{},
		ClusterDef:     &dbaasv1alpha1.ClusterDefinition{},
		ClusterVersion: &dbaasv1alpha1.ClusterVersion{},
		Nodes:          []*corev1.Node{},
	}
}

// Get all kubernetes objects belonging to the database cluster
func (o *ObjectsGetter) Get() (*ClusterObjects, error) {
	var err error
	objs := NewClusterObjects()
	ctx := context.TODO()
	corev1 := o.ClientSet.CoreV1()
	getResource := func(gvr schema.GroupVersionResource, name string, ns string, res interface{}) error {
		obj, err := o.DynamicClient.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{}, "")
		if err != nil {
			return err
		}
		return runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, res)
	}

	listOpts := func() metav1.ListOptions {
		return metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", types.InstanceLabelKey, o.Name),
		}
	}

	// get cluster
	if err = getResource(types.ClusterGVR(), o.Name, o.Namespace, objs.Cluster); err != nil {
		return nil, err
	}

	// get cluster definition
	if o.WithClusterDef {
		if err = getResource(types.ClusterDefGVR(), objs.Cluster.Spec.ClusterDefRef, "",
			objs.ClusterDef); err != nil {
			return nil, err
		}
	}

	// get cluster version
	if o.WithClusterVersion {
		if err = getResource(types.ClusterVersionGVR(), objs.Cluster.Spec.ClusterVersionRef, "",
			objs.ClusterVersion); err != nil {
			return nil, err
		}
	}

	// get services
	if o.WithService {
		if objs.Services, err = corev1.Services(o.Namespace).List(ctx, listOpts()); err != nil {
			return nil, err
		}
	}

	// get secrets
	if o.WithSecret {
		if objs.Secrets, err = corev1.Secrets(o.Namespace).List(ctx, listOpts()); err != nil {
			return nil, err
		}
	}

	// get configmaps
	if o.WithConfigMap {
		if objs.ConfigMaps, err = corev1.ConfigMaps(o.Namespace).List(ctx, listOpts()); err != nil {
			return nil, err
		}
	}

	// get PVCs
	if o.WithPVC {
		if objs.PVCs, err = corev1.PersistentVolumeClaims(o.Namespace).List(ctx, listOpts()); err != nil {
			return nil, err
		}
	}

	// get pods
	if o.WithPod {
		if objs.Pods, err = corev1.Pods(o.Namespace).List(ctx, listOpts()); err != nil {
			return nil, err
		}
		// get nodes where the pods are located
	podLoop:
		for _, pod := range objs.Pods.Items {
			for _, node := range objs.Nodes {
				if node.Name == pod.Spec.NodeName {
					continue podLoop
				}
			}

			nodeName := pod.Spec.NodeName
			if len(nodeName) == 0 {
				continue
			}

			node, err := corev1.Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			objs.Nodes = append(objs.Nodes, node)
		}
	}
	return objs, nil
}

func (o *ClusterObjects) GetClusterInfo() *ClusterInfo {
	c := o.Cluster
	cluster := &ClusterInfo{
		Name:              c.Name,
		Namespace:         c.Namespace,
		ClusterVersion:    c.Spec.ClusterVersionRef,
		ClusterDefinition: c.Spec.ClusterDefRef,
		TerminationPolicy: string(c.Spec.TerminationPolicy),
		Status:            string(c.Status.Phase),
		Age:               duration.HumanDuration(time.Since(c.CreationTimestamp.Time)),
		InternalEP:        valueNone,
		ExternalEP:        valueNone,
	}

	primaryComponent := FindClusterComp(o.Cluster, o.ClusterDef.Spec.Components[0].TypeName)
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

	for _, cdComp := range o.ClusterDef.Spec.Components {
		c := FindClusterComp(o.Cluster, cdComp.TypeName)
		if c == nil {
			return nil
		}

		if c.Replicas == nil {
			r := cdComp.DefaultReplicas
			c.Replicas = &r
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
			Replicas: fmt.Sprintf("%d / %d", *c.Replicas, len(pods)),
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
			Role:       getLabelVal(pod.Labels, types.RoleLabelKey),
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
