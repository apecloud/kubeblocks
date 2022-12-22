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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

// GetDefaultPodName get the default pod in the cluster
func GetDefaultPodName(dynamic dynamic.Interface, name string, namespace string) (string, error) {
	obj, err := dynamic.Resource(types.ClusterGVR()).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	cluster := &dbaasv1alpha1.Cluster{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cluster); err != nil {
		return "", err
	}

	// travel all components, check type
	for _, c := range cluster.Status.Components {
		if c.ConsensusSetStatus != nil {
			return c.ConsensusSetStatus.Leader.Pod, nil
		}
		// TODO: now we only support consensus set
	}

	return "", fmt.Errorf("failed to find the pod to exec command")
}

// GetClusterTypeByPod gets the cluster type from pod label
func GetClusterTypeByPod(pod *corev1.Pod) (string, error) {
	var clusterType string

	if name, ok := pod.Labels["app.kubernetes.io/name"]; ok {
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

// GetClusterEndpoints gets cluster internal and external endpoints
func GetClusterEndpoints(svcList *corev1.ServiceList, c *dbaasv1alpha1.ClusterComponent) ([]string, []string) {
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

	getExternalIP := func(svc *corev1.Service) string {
		if svc.GetAnnotations()[types.ServiceLBTypeAnnotationKey] != types.ServiceLBTypeAnnotationValue {
			return ""
		}
		return svc.GetAnnotations()[types.ServiceFloatingIPAnnotationKey]
	}

	for _, svc := range svcList.Items {
		if svc.GetLabels()[types.ComponentLabelKey] != c.Name {
			continue
		}
		var (
			internalIP = svc.Spec.ClusterIP
			externalIP = getExternalIP(&svc)
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

func GetVersionByClusterDef(dynamic dynamic.Interface, clusterDef string) (*dbaasv1alpha1.AppVersionList, error) {
	versionList := &dbaasv1alpha1.AppVersionList{}
	objList, err := dynamic.Resource(types.AppVersionGVR()).Namespace("").
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
