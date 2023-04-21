/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package cluster

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	ComponentNameEmpty = ""
)

// GetSimpleInstanceInfos return simple instance info that only contains instance name and role, the default
// instance should be the first element in the returned array.
func GetSimpleInstanceInfos(dynamic dynamic.Interface, name, namespace string) []*InstanceInfo {
	return GetSimpleInstanceInfosForComponent(dynamic, name, ComponentNameEmpty, namespace)
}

// GetSimpleInstanceInfosForComponent return simple instance info that only contains instance name and role,
func GetSimpleInstanceInfosForComponent(dynamic dynamic.Interface, name, componentName, namespace string) []*InstanceInfo {
	// if cluster status contains what we need, return directly
	if infos := getInstanceInfoFromStatus(dynamic, name, componentName, namespace); len(infos) > 0 {
		return infos
	}

	// if cluster status does not contain what we need, try to list all pods and build instance info
	return getInstanceInfoByList(dynamic, name, componentName, namespace)
}

// getInstancesInfoFromCluster get instances info from cluster status
func getInstanceInfoFromStatus(dynamic dynamic.Interface, name, componentName, namespace string) []*InstanceInfo {
	var infos []*InstanceInfo
	cluster, err := GetClusterByName(dynamic, name, namespace)
	if err != nil {
		return nil
	}
	// travel all components, check type
	for compName, c := range cluster.Status.Components {
		// filter by component name
		if len(componentName) > 0 && compName != componentName {
			continue
		}

		var info *InstanceInfo
		// workload type is Consensus
		if c.ConsensusSetStatus != nil {
			buildInfoByStatus := func(status *appsv1alpha1.ConsensusMemberStatus) {
				if status == nil {
					return
				}
				info = &InstanceInfo{Role: status.Name, Name: status.Pod}
				infos = append(infos, info)
			}

			// leader must be first
			buildInfoByStatus(&c.ConsensusSetStatus.Leader)

			// followers
			for _, f := range c.ConsensusSetStatus.Followers {
				buildInfoByStatus(&f)
			}

			// learner
			buildInfoByStatus(c.ConsensusSetStatus.Learner)
		}

		// workload type is Replication
		if c.ReplicationSetStatus != nil {
			buildInfoByStatus := func(status *appsv1alpha1.ReplicationMemberStatus) {
				if status == nil {
					return
				}
				info = &InstanceInfo{Name: status.Pod}
				infos = append(infos, info)
			}
			// primary
			buildInfoByStatus(&c.ReplicationSetStatus.Primary)

			// secondaries
			for _, f := range c.ReplicationSetStatus.Secondaries {
				buildInfoByStatus(&f)
			}
		}
	}
	return infos
}

// getInstanceInfoByList get instances info by list all pods
func getInstanceInfoByList(dynamic dynamic.Interface, name, componentName, namespace string) []*InstanceInfo {
	var infos []*InstanceInfo
	// filter by cluster name
	labels := util.BuildLabelSelectorByNames("", []string{name})
	// filter by component name
	if len(componentName) > 0 {
		labels = util.BuildComponentNameLabels(labels, []string{componentName})
	}

	objs, err := dynamic.Resource(schema.GroupVersionResource{Group: corev1.GroupName, Version: types.K8sCoreAPIVersion, Resource: "pods"}).
		Namespace(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labels})

	if err != nil {
		return nil
	}

	for _, o := range objs.Items {
		infos = append(infos, &InstanceInfo{Name: o.GetName()})
	}
	return infos
}

// FindClusterComp finds component in cluster object based on the component definition name
func FindClusterComp(cluster *appsv1alpha1.Cluster, compDefName string) *appsv1alpha1.ClusterComponentSpec {
	for i, c := range cluster.Spec.ComponentSpecs {
		if c.ComponentDefRef == compDefName {
			return &cluster.Spec.ComponentSpecs[i]
		}
	}
	return nil
}

// GetComponentEndpoints gets component internal and external endpoints
func GetComponentEndpoints(svcList *corev1.ServiceList, c *appsv1alpha1.ClusterComponentSpec) ([]string, []string) {
	var (
		internalEndpoints []string
		externalEndpoints []string
	)

	getEndpoints := func(ip string, ports []corev1.ServicePort) []string {
		var result []string
		for _, port := range ports {
			result = append(result, fmt.Sprintf("%s:%d", ip, port.Port))
		}
		return result
	}

	internalSvcs, externalSvcs := GetComponentServices(svcList, c)
	for _, svc := range internalSvcs {
		dns := fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)
		internalEndpoints = append(internalEndpoints, getEndpoints(dns, svc.Spec.Ports)...)
	}

	for _, svc := range externalSvcs {
		externalEndpoints = append(externalEndpoints, getEndpoints(GetExternalAddr(svc), svc.Spec.Ports)...)
	}
	return internalEndpoints, externalEndpoints
}

// GetComponentServices gets component services
func GetComponentServices(svcList *corev1.ServiceList, c *appsv1alpha1.ClusterComponentSpec) ([]*corev1.Service, []*corev1.Service) {
	if svcList == nil {
		return nil, nil
	}

	var internalSvcs, externalSvcs []*corev1.Service
	for i, svc := range svcList.Items {
		if svc.GetLabels()[constant.KBAppComponentLabelKey] != c.Name {
			continue
		}

		var (
			internalIP   = svc.Spec.ClusterIP
			externalAddr = GetExternalAddr(&svc)
		)
		if svc.Spec.Type == corev1.ServiceTypeClusterIP && internalIP != "" && internalIP != "None" {
			internalSvcs = append(internalSvcs, &svcList.Items[i])
		}
		if externalAddr != "" {
			externalSvcs = append(externalSvcs, &svcList.Items[i])
		}
	}
	return internalSvcs, externalSvcs
}

// GetExternalAddr get external IP from service annotation
func GetExternalAddr(svc *corev1.Service) string {
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if ingress.Hostname != "" {
			return ingress.Hostname
		}

		if ingress.IP != "" {
			return ingress.IP
		}
	}
	if svc.GetAnnotations()[types.ServiceHAVIPTypeAnnotationKey] != types.ServiceHAVIPTypeAnnotationValue {
		return ""
	}
	return svc.GetAnnotations()[types.ServiceFloatingIPAnnotationKey]
}

// GetK8SClientObject gets the client object of k8s,
// obj must be a struct pointer so that obj can be updated with the response.
func GetK8SClientObject(dynamic dynamic.Interface,
	obj client.Object,
	gvr schema.GroupVersionResource,
	namespace,
	name string) error {
	unstructuredObj, err := dynamic.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), obj)
}

func GetClusterDefByName(dynamic dynamic.Interface, name string) (*appsv1alpha1.ClusterDefinition, error) {
	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err := GetK8SClientObject(dynamic, clusterDef, types.ClusterDefGVR(), "", name); err != nil {
		return nil, err
	}
	return clusterDef, nil
}

func GetDefaultCompName(cd *appsv1alpha1.ClusterDefinition) (string, error) {
	if len(cd.Spec.ComponentDefs) == 1 {
		return cd.Spec.ComponentDefs[0].Name, nil
	}
	return "", fmt.Errorf("failed to get the default component definition name")
}

func GetClusterByName(dynamic dynamic.Interface, name string, namespace string) (*appsv1alpha1.Cluster, error) {
	cluster := &appsv1alpha1.Cluster{}
	if err := GetK8SClientObject(dynamic, cluster, types.ClusterGVR(), namespace, name); err != nil {
		return nil, err
	}
	return cluster, nil
}

func GetVersionByClusterDef(dynamic dynamic.Interface, clusterDef string) (*appsv1alpha1.ClusterVersionList, error) {
	versionList := &appsv1alpha1.ClusterVersionList{}
	objList, err := dynamic.Resource(types.ClusterVersionGVR()).Namespace("").
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", constant.ClusterDefLabelKey, clusterDef),
		})
	if err != nil {
		return nil, err
	}
	if objList == nil {
		return nil, fmt.Errorf("failed to find component version referencing cluster definition %s", clusterDef)
	}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(objList.UnstructuredContent(), versionList); err != nil {
		return nil, err
	}
	return versionList, nil
}

func FakeClusterObjs() *ClusterObjects {
	clusterObjs := NewClusterObjects()
	clusterObjs.Cluster = testing.FakeCluster(testing.ClusterName, testing.Namespace)
	clusterObjs.ClusterDef = testing.FakeClusterDef()
	clusterObjs.Pods = testing.FakePods(3, testing.Namespace, testing.ClusterName)
	clusterObjs.Secrets = testing.FakeSecrets(testing.Namespace, testing.ClusterName)
	clusterObjs.Nodes = []*corev1.Node{testing.FakeNode()}
	clusterObjs.Services = testing.FakeServices()
	return clusterObjs
}

func BuildStorageSize(storages []StorageInfo) string {
	var sizes []string
	for _, s := range storages {
		sizes = append(sizes, fmt.Sprintf("%s:%s", s.Name, s.Size))
	}
	return util.CheckEmpty(strings.Join(sizes, "\n"))
}

func BuildStorageClass(storages []StorageInfo) string {
	var scs []string
	for _, s := range storages {
		scs = append(scs, s.StorageClass)
	}
	return util.CheckEmpty(strings.Join(scs, "\n"))
}

// GetLatestVersion get the latest cluster versions that reference the cluster definition
func GetLatestVersion(dynamic dynamic.Interface, clusterDef string) (string, error) {
	versionList, err := GetVersionByClusterDef(dynamic, clusterDef)
	if err != nil {
		return "", err
	}

	// find the latest version to use
	version := findLatestVersion(versionList)
	if version == nil {
		return "", fmt.Errorf("failed to find the latest cluster version referencing current cluster definition %s", clusterDef)
	}
	return version.Name, nil
}

func findLatestVersion(versions *appsv1alpha1.ClusterVersionList) *appsv1alpha1.ClusterVersion {
	if len(versions.Items) == 0 {
		return nil
	}
	if len(versions.Items) == 1 {
		return &versions.Items[0]
	}

	var version *appsv1alpha1.ClusterVersion
	for i, v := range versions.Items {
		if version == nil {
			version = &versions.Items[i]
			continue
		}
		if v.CreationTimestamp.Time.After(version.CreationTimestamp.Time) {
			version = &versions.Items[i]
		}
	}
	return version
}

type CompInfo struct {
	Component       *appsv1alpha1.ClusterComponentSpec
	ComponentStatus *appsv1alpha1.ClusterComponentStatus
	ComponentDef    *appsv1alpha1.ClusterComponentDefinition
}

func (info *CompInfo) InferPodName() (string, error) {
	if info.ComponentStatus == nil {
		return "", fmt.Errorf("component status is missing")
	}
	if info.ComponentStatus.Phase != appsv1alpha1.RunningClusterCompPhase || !*info.ComponentStatus.PodsReady {
		return "", fmt.Errorf("component is not ready, please try later")
	}
	if info.ComponentStatus.ConsensusSetStatus != nil {
		return info.ComponentStatus.ConsensusSetStatus.Leader.Pod, nil
	}
	if info.ComponentStatus.ReplicationSetStatus != nil {
		return info.ComponentStatus.ReplicationSetStatus.Primary.Pod, nil
	}
	return "", fmt.Errorf("cannot infer the pod to connect, please specify the pod name explicitly by `--instance` flag")
}

func FillCompInfoByName(ctx context.Context, dynamic dynamic.Interface, namespace, clusterName, componentName string) (*CompInfo, error) {
	cluster, err := GetClusterByName(dynamic, clusterName, namespace)
	if err != nil {
		return nil, err
	}
	if cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
		return nil, fmt.Errorf("cluster %s is not running, please try later", clusterName)
	}

	compInfo := &CompInfo{}
	// fill component
	if len(componentName) == 0 {
		compInfo.Component = &cluster.Spec.ComponentSpecs[0]
	} else {
		compInfo.Component = cluster.Spec.GetComponentByName(componentName)
	}

	if compInfo.Component == nil {
		return nil, fmt.Errorf("component %s not found in cluster %s", componentName, clusterName)
	}
	// fill component status
	for name, compStatus := range cluster.Status.Components {
		if name == compInfo.Component.Name {
			compInfo.ComponentStatus = &compStatus
			break
		}
	}
	if compInfo.ComponentStatus == nil {
		return nil, fmt.Errorf("componentStatus %s not found in cluster %s", componentName, clusterName)
	}

	// find cluster def
	clusterDef, err := GetClusterDefByName(dynamic, cluster.Spec.ClusterDefRef)
	if err != nil {
		return nil, err
	}
	// find component def by reference
	for _, compDef := range clusterDef.Spec.ComponentDefs {
		if compDef.Name == compInfo.Component.ComponentDefRef {
			compInfo.ComponentDef = &compDef
			break
		}
	}
	if compInfo.ComponentDef == nil {
		return nil, fmt.Errorf("componentDef %s not found in clusterDef %s", compInfo.Component.ComponentDefRef, clusterDef.Name)
	}
	return compInfo, nil
}

func GetPodClusterName(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[constant.AppInstanceLabelKey]
}

func GetPodComponentName(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[constant.KBAppComponentLabelKey]
}

func GetPodWorkloadType(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[constant.WorkloadTypeLabelKey]
}

func GetConfigMapByName(dynamic dynamic.Interface, namespace, name string) (*corev1.ConfigMap, error) {
	cmObj := &corev1.ConfigMap{}
	if err := GetK8SClientObject(dynamic, cmObj, types.ConfigmapGVR(), namespace, name); err != nil {
		return nil, err
	}
	return cmObj, nil
}

func GetConfigConstraintByName(dynamic dynamic.Interface, name string) (*appsv1alpha1.ConfigConstraint, error) {
	ccObj := &appsv1alpha1.ConfigConstraint{}
	if err := GetK8SClientObject(dynamic, ccObj, types.ConfigConstraintGVR(), "", name); err != nil {
		return nil, err
	}
	return ccObj, nil
}
