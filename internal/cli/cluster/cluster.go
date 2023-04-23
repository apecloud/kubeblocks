/*
Copyright ApeCloud, Inc.

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/util/resource"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type GetOptions struct {
	WithClusterDef     bool
	WithClusterVersion bool
	WithConfigMap      bool
	WithPVC            bool
	WithService        bool
	WithSecret         bool
	WithPod            bool
	WithEvent          bool
	WithDataProtection bool
}

type ObjectsGetter struct {
	Client    clientset.Interface
	Dynamic   dynamic.Interface
	Name      string
	Namespace string
	GetOptions
}

func NewClusterObjects() *ClusterObjects {
	return &ClusterObjects{
		Cluster: &appsv1alpha1.Cluster{},
		Nodes:   []*corev1.Node{},
	}
}

func listResources[T any](dynamic dynamic.Interface, gvr schema.GroupVersionResource, ns string, opts metav1.ListOptions, items *[]T) error {
	if *items == nil {
		*items = []T{}
	}
	obj, err := dynamic.Resource(gvr).Namespace(ns).List(context.TODO(), opts)
	if err != nil {
		return err
	}
	for _, i := range obj.Items {
		var object T
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(i.Object, &object); err != nil {
			return err
		}
		*items = append(*items, object)
	}
	return nil
}

// Get all kubernetes objects belonging to the database cluster
func (o *ObjectsGetter) Get() (*ClusterObjects, error) {
	var err error
	objs := NewClusterObjects()
	ctx := context.TODO()
	corev1 := o.Client.CoreV1()
	getResource := func(gvr schema.GroupVersionResource, name string, ns string, res interface{}) error {
		obj, err := o.Dynamic.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{}, "")
		if err != nil {
			return err
		}
		return runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, res)
	}

	listOpts := func() metav1.ListOptions {
		return metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
				constant.AppInstanceLabelKey, o.Name,
				constant.AppManagedByLabelKey, constant.AppName),
		}
	}

	// get cluster
	if err = getResource(types.ClusterGVR(), o.Name, o.Namespace, objs.Cluster); err != nil {
		return nil, err
	}

	// wrap the cluster phase if the latest ops request is processing
	latestOpsProcessedCondition := meta.FindStatusCondition(objs.Cluster.Status.Conditions, appsv1alpha1.ConditionTypeLatestOpsRequestProcessed)
	if latestOpsProcessedCondition != nil && latestOpsProcessedCondition.Status == metav1.ConditionFalse {
		objs.Cluster.Status.Phase = appsv1alpha1.ClusterPhase(latestOpsProcessedCondition.Reason)
	}

	// get cluster definition
	if o.WithClusterDef {
		cd := &appsv1alpha1.ClusterDefinition{}
		if err = getResource(types.ClusterDefGVR(), objs.Cluster.Spec.ClusterDefRef, "", cd); err != nil {
			return nil, err
		}
		objs.ClusterDef = cd
	}

	// get cluster version
	if o.WithClusterVersion {
		v := &appsv1alpha1.ClusterVersion{}
		if err = getResource(types.ClusterVersionGVR(), objs.Cluster.Spec.ClusterVersionRef, "", v); err != nil {
			return nil, err
		}
		objs.ClusterVersion = v
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

	// get events
	if o.WithEvent {
		// get all events about cluster
		if objs.Events, err = corev1.Events(o.Namespace).Search(scheme.Scheme, objs.Cluster); err != nil {
			return nil, err
		}

		// get all events about pods
		for _, pod := range objs.Pods.Items {
			events, err := corev1.Events(o.Namespace).Search(scheme.Scheme, &pod)
			if err != nil {
				return nil, err
			}
			if objs.Events == nil {
				objs.Events = events
			} else {
				objs.Events.Items = append(objs.Events.Items, events.Items...)
			}
		}
	}
	if o.WithDataProtection {
		dplistOpts := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s",
				constant.AppInstanceLabelKey, o.Name),
		}
		if err := listResources(o.Dynamic, types.BackupPolicyGVR(), o.Namespace, dplistOpts, &objs.BackupPolicies); err != nil {
			return nil, err
		}
		if err := listResources(o.Dynamic, types.BackupGVR(), o.Namespace, dplistOpts, &objs.Backups); err != nil {
			return nil, err
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
		CreatedTime:       util.TimeFormat(&c.CreationTimestamp),
		InternalEP:        types.None,
		ExternalEP:        types.None,
		Labels:            util.CombineLabels(c.Labels),
	}

	if o.ClusterDef == nil {
		return cluster
	}

	primaryComponent := FindClusterComp(o.Cluster, o.ClusterDef.Spec.ComponentDefs[0].Name)
	internalEndpoints, externalEndpoints := GetComponentEndpoints(o.Services, primaryComponent)
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
	for _, c := range o.Cluster.Spec.ComponentSpecs {
		// get all pods belonging to current component
		var pods []*corev1.Pod
		for _, p := range o.Pods.Items {
			if n, ok := p.Labels[constant.KBAppComponentLabelKey]; ok && n == c.Name {
				pods = append(pods, &p)
			}
		}

		// current component has no pod corresponding to it
		if len(pods) == 0 {
			continue
		}

		image := types.None
		if len(pods) > 0 {
			image = pods[0].Spec.Containers[0].Image
		}

		running, waiting, succeeded, failed := util.GetPodStatus(pods)
		comp := &ComponentInfo{
			Name:      c.Name,
			NameSpace: o.Cluster.Namespace,
			Type:      c.ComponentDefRef,
			Cluster:   o.Cluster.Name,
			Replicas:  fmt.Sprintf("%d / %d", c.Replicas, len(pods)),
			Status:    fmt.Sprintf("%d / %d / %d / %d ", running, waiting, succeeded, failed),
			Image:     image,
		}
		comp.CPU, comp.Memory = getResourceInfo(c.Resources.Requests, c.Resources.Limits)
		comp.Storage = o.getStorageInfo(&c)
		comps = append(comps, comp)
	}
	return comps
}

func (o *ClusterObjects) GetInstanceInfo() []*InstanceInfo {
	var instances []*InstanceInfo
	for _, pod := range o.Pods.Items {
		instance := &InstanceInfo{
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Cluster:     getLabelVal(pod.Labels, constant.AppInstanceLabelKey),
			Component:   getLabelVal(pod.Labels, constant.KBAppComponentLabelKey),
			Status:      string(pod.Status.Phase),
			Role:        getLabelVal(pod.Labels, constant.RoleLabelKey),
			AccessMode:  getLabelVal(pod.Labels, constant.ConsensusSetAccessModeLabelKey),
			CreatedTime: util.TimeFormat(&pod.CreationTimestamp),
		}

		var component *appsv1alpha1.ClusterComponentSpec
		for i, c := range o.Cluster.Spec.ComponentSpecs {
			if c.Name == instance.Component {
				component = &o.Cluster.Spec.ComponentSpecs[i]
			}
		}
		instance.Storage = o.getStorageInfo(component)
		getInstanceNodeInfo(o.Nodes, &pod, instance)
		instance.CPU, instance.Memory = getResourceInfo(resource.PodRequestsAndLimits(&pod))
		instances = append(instances, instance)
	}
	return instances
}

func (o *ClusterObjects) getStorageInfo(component *appsv1alpha1.ClusterComponentSpec) []StorageInfo {
	if component == nil {
		return nil
	}

	getClassName := func(vcTpl *appsv1alpha1.ClusterComponentVolumeClaimTemplate) string {
		if vcTpl.Spec.StorageClassName != nil {
			return *vcTpl.Spec.StorageClassName
		}

		if o.PVCs == nil {
			return types.None
		}

		// get storage class name from PVC
		for _, pvc := range o.PVCs.Items {
			labels := pvc.Labels
			if len(labels) == 0 {
				continue
			}

			if labels[constant.KBAppComponentLabelKey] != component.Name {
				continue
			}

			if labels[constant.VolumeClaimTemplateNameLabelKey] != vcTpl.Name {
				continue
			}
			return *pvc.Spec.StorageClassName
		}

		return types.None
	}

	var infos []StorageInfo
	for _, vcTpl := range component.VolumeClaimTemplates {
		s := StorageInfo{
			Name: vcTpl.Name,
		}
		val := vcTpl.Spec.Resources.Requests[corev1.ResourceStorage]
		s.StorageClass = getClassName(&vcTpl)
		s.Size = val.String()
		s.AccessMode = getAccessModes(vcTpl.Spec.AccessModes)
		infos = append(infos, s)
	}
	return infos
}

func getInstanceNodeInfo(nodes []*corev1.Node, pod *corev1.Pod, i *InstanceInfo) {
	i.Node, i.Region, i.AZ = types.None, types.None, types.None
	if pod.Spec.NodeName == "" {
		return
	}

	i.Node = strings.Join([]string{pod.Spec.NodeName, pod.Status.HostIP}, "/")
	node := util.GetNodeByName(nodes, pod.Spec.NodeName)
	if node == nil {
		return
	}

	i.Region = getLabelVal(node.Labels, constant.RegionLabelKey)
	i.AZ = getLabelVal(node.Labels, constant.ZoneLabelKey)
}

func getResourceInfo(reqs, limits corev1.ResourceList) (string, string) {
	var cpu, mem string
	names := []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory}
	for _, name := range names {
		res := types.None
		limit, req := limits[name], reqs[name]

		// if request is empty and limit is not, set limit to request
		if util.ResourceIsEmpty(&req) && !util.ResourceIsEmpty(&limit) {
			req = limit
		}

		// if both limit and request are empty, only output none
		if !util.ResourceIsEmpty(&limit) || !util.ResourceIsEmpty(&req) {
			res = fmt.Sprintf("%s / %s", req.String(), limit.String())
		}

		switch name {
		case corev1.ResourceCPU:
			cpu = res
		case corev1.ResourceMemory:
			mem = res
		}
	}
	return cpu, mem
}

func getLabelVal(labels map[string]string, key string) string {
	val := labels[key]
	if len(val) == 0 {
		return types.None
	}
	return val
}

func getAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
	modes = removeDuplicateAccessModes(modes)
	var modesStr []string
	if containsAccessMode(modes, corev1.ReadWriteOnce) {
		modesStr = append(modesStr, "RWO")
	}
	if containsAccessMode(modes, corev1.ReadOnlyMany) {
		modesStr = append(modesStr, "ROX")
	}
	if containsAccessMode(modes, corev1.ReadWriteMany) {
		modesStr = append(modesStr, "RWX")
	}
	return strings.Join(modesStr, ",")
}

func removeDuplicateAccessModes(modes []corev1.PersistentVolumeAccessMode) []corev1.PersistentVolumeAccessMode {
	var accessModes []corev1.PersistentVolumeAccessMode
	for _, m := range modes {
		if !containsAccessMode(accessModes, m) {
			accessModes = append(accessModes, m)
		}
	}
	return accessModes
}

func containsAccessMode(modes []corev1.PersistentVolumeAccessMode, mode corev1.PersistentVolumeAccessMode) bool {
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}
