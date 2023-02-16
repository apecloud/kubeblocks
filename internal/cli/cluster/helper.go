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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// GetSimpleInstanceInfos return simple instance info that only contains instance name and role, the default
// instance should be the first element in the returned array.
func GetSimpleInstanceInfos(dynamic dynamic.Interface, name string, namespace string) []*InstanceInfo {
	var infos []*InstanceInfo
	obj, err := dynamic.Resource(types.ClusterGVR()).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil
	}

	cluster := &dbaasv1alpha1.Cluster{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cluster); err != nil {
		return nil
	}

	// travel all components, check type
	for _, c := range cluster.Status.Components {
		var info *InstanceInfo
		if c.ConsensusSetStatus != nil {
			buildInfoByStatus := func(status *dbaasv1alpha1.ConsensusMemberStatus) {
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
			buildInfoByStatus := func(status *dbaasv1alpha1.ReplicationMemberStatus) {
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

		// TODO: now we only support consensus set
	}

	// if cluster status does not contain what we need, try to get all instances
	objs, err := dynamic.Resource(schema.GroupVersionResource{Group: corev1.GroupName, Version: types.VersionV1, Resource: "pods"}).
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

	if name, ok := pod.Labels[types.NameLabelKey]; ok {
		clusterType = strings.Split(name, "-")[0]
	}

	if clusterType == "" {
		return "", fmt.Errorf("failed to get the cluster type")
	}

	return clusterType, nil
}

// GetAllCluster get all clusters in current namespace
func GetAllCluster(client dynamic.Interface, namespace string, clusters *dbaasv1alpha1.ClusterList) error {
	objs, err := client.Resource(types.ClusterGVR()).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(objs.UnstructuredContent(), clusters)
}

// FindClusterComp finds component in cluster object based on the component type name
func FindClusterComp(cluster *dbaasv1alpha1.Cluster, typeName string) *dbaasv1alpha1.ClusterComponent {
	for i, c := range cluster.Spec.Components {
		if c.Type == typeName {
			return &cluster.Spec.Components[i]
		}
	}
	return nil
}

// GetComponentEndpoints gets component internal and external endpoints
func GetComponentEndpoints(svcList *corev1.ServiceList, c *dbaasv1alpha1.ClusterComponent) ([]string, []string) {
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

	svcs := GetComponentServices(svcList, c)
	for _, svc := range svcs {
		var (
			internalIP = svc.Spec.ClusterIP
			externalIP = GetExternalIP(svc)
		)
		if internalIP != "" && internalIP != "None" {
			internalEndpoints = append(internalEndpoints, getEndpoints(internalIP, svc.Spec.Ports)...)
		}
		if externalIP != "" && externalIP != "None" {
			externalEndpoints = append(externalEndpoints, getEndpoints(externalIP, svc.Spec.Ports)...)
		}
	}
	return internalEndpoints, externalEndpoints
}

// GetComponentServices gets component services
func GetComponentServices(svcList *corev1.ServiceList, c *dbaasv1alpha1.ClusterComponent) []*corev1.Service {
	if svcList == nil {
		return nil
	}

	var svcs []*corev1.Service
	for i, svc := range svcList.Items {
		if svc.GetLabels()[types.ComponentLabelKey] != c.Name {
			continue
		}
		svcs = append(svcs, &svcList.Items[i])
	}
	return svcs
}

// GetExternalIP get external IP from service annotation
func GetExternalIP(svc *corev1.Service) string {
	if svc.GetAnnotations()[types.ServiceLBTypeAnnotationKey] != types.ServiceLBTypeAnnotationValue {
		return ""
	}
	return svc.GetAnnotations()[types.ServiceFloatingIPAnnotationKey]
}

func GetClusterDefByName(dynamic dynamic.Interface, name string) (*dbaasv1alpha1.ClusterDefinition, error) {
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	obj, err := dynamic.Resource(types.ClusterDefGVR()).Namespace("").
		Get(context.TODO(), name, metav1.GetOptions{}, "")
	if err != nil {
		return nil, err
	}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, clusterDef); err != nil {
		return nil, err
	}
	return clusterDef, nil
}

func GetDefaultCompTypeName(cd *dbaasv1alpha1.ClusterDefinition) (string, error) {
	if len(cd.Spec.Components) == 1 {
		return cd.Spec.Components[0].TypeName, nil
	}
	return "", fmt.Errorf("failed to get the default component type")
}

func GetClusterByName(dynamic dynamic.Interface, name string, namespace string) (*dbaasv1alpha1.Cluster, error) {
	cluster := &dbaasv1alpha1.Cluster{}
	obj, err := dynamic.Resource(types.ClusterGVR()).Namespace(namespace).
		Get(context.TODO(), name, metav1.GetOptions{}, "")
	if err != nil {
		return nil, err
	}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}

func GetVersionByClusterDef(dynamic dynamic.Interface, clusterDef string) (*dbaasv1alpha1.ClusterVersionList, error) {
	versionList := &dbaasv1alpha1.ClusterVersionList{}
	objList, err := dynamic.Resource(types.ClusterVersionGVR()).Namespace("").
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", types.ClusterDefLabelKey, clusterDef),
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

func findLatestVersion(versions *dbaasv1alpha1.ClusterVersionList) *dbaasv1alpha1.ClusterVersion {
	if len(versions.Items) == 0 {
		return nil
	}
	if len(versions.Items) == 1 {
		return &versions.Items[0]
	}

	var version *dbaasv1alpha1.ClusterVersion
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
