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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/constant"
)

// GetSimpleInstanceInfos return simple instance info that only contains instance name and role, the default
// instance should be the first element in the returned array.
func GetSimpleInstanceInfos(dynamic dynamic.Interface, name string, namespace string) []*InstanceInfo {
	var infos []*InstanceInfo
	cluster, err := GetClusterByName(dynamic, name, namespace)
	if err != nil {
		return nil
	}

	// travel all components, check type
	for _, c := range cluster.Status.Components {
		var info *InstanceInfo
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

	// if cluster status does not contain what we need, try to get all instances
	objs, err := dynamic.Resource(schema.GroupVersionResource{Group: corev1.GroupName, Version: types.K8sCoreAPIVersion, Resource: "pods"}).
		Namespace(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: util.BuildLabelSelectorByNames("", []string{cluster.Name}),
	})
	if err != nil {
		return nil
	}
	for _, o := range objs.Items {
		infos = append(infos, &InstanceInfo{Name: o.GetName()})
	}

	return infos
}

// GetClusterTypeByPod gets the cluster type from pod label
func GetClusterTypeByPod(pod *corev1.Pod) (string, error) {
	var clusterType string

	if name, ok := pod.Labels[intctrlutil.AppNameLabelKey]; ok {
		clusterType = strings.Split(name, "-")[0]
	}

	if clusterType == "" {
		return "", fmt.Errorf("failed to get the cluster type")
	}

	return clusterType, nil
}

// GetAllCluster get all clusters in current namespace
func GetAllCluster(client dynamic.Interface, namespace string, clusters *appsv1alpha1.ClusterList) error {
	objs, err := client.Resource(types.ClusterGVR()).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(objs.UnstructuredContent(), clusters)
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
		if svc.GetLabels()[intctrlutil.KBAppComponentLabelKey] != c.Name {
			continue
		}

		var (
			internalIP   = svc.Spec.ClusterIP
			externalAddr = GetExternalAddr(&svc)
		)
		if internalIP != "" && internalIP != "None" {
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
	if svc.GetAnnotations()[types.ServiceLBTypeAnnotationKey] != types.ServiceLBTypeAnnotationValue {
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
			LabelSelector: fmt.Sprintf("%s=%s", intctrlutil.ClusterDefLabelKey, clusterDef),
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
		return "", fmt.Errorf("failed to find latest cluster version referencing current cluster definition %s", clusterDef)
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
