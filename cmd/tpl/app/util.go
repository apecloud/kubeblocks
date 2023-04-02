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

package app

import (
	"reflect"

	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

type RenderedOptions struct {
	ConfigSpec     string
	AllConfigSpecs bool

	// mock cluster object
	Name      string
	Namespace string

	Replicas       int32
	DataVolumeName string
	ComponentName  string

	CPU    string
	Memory string
}

func mockClusterObject(clusterDefObj *appsv1alpha1.ClusterDefinition, renderedOpts RenderedOptions, clusterVersion *appsv1alpha1.ClusterVersion) *appsv1alpha1.Cluster {
	cvReference := ""
	if clusterVersion != nil {
		cvReference = clusterVersion.Name
	}
	factory := testapps.NewClusterFactory(renderedOpts.Namespace, renderedOpts.Name, clusterDefObj.Name, cvReference)
	for _, component := range clusterDefObj.Spec.ComponentDefs {
		factory.AddComponent(component.CharacterType+"-"+RandomString(3), component.Name)
		factory.SetReplicas(renderedOpts.Replicas)
		if renderedOpts.DataVolumeName != "" {
			pvc := testapps.NewPVC("10Gi")
			factory.AddVolumeClaimTemplate(renderedOpts.DataVolumeName, &pvc)
		}
		if renderedOpts.CPU != "" || renderedOpts.Memory != "" {
			factory.SetResources(fromResource(renderedOpts))
		}
	}
	return factory.GetObject()
}

func fromResource(opts RenderedOptions) corev1.ResourceRequirements {
	cpu := opts.CPU
	memory := opts.Memory
	if cpu == "" {
		cpu = "1"
	}
	if memory == "" {
		memory = "1Gi"
	}
	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse(cpu),
			"memory": resource.MustParse(memory),
		},
	}
}

func kindFromResource[T any](resource T) string {
	t := reflect.TypeOf(resource)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func RandomString(n int) string {
	s, _ := password.Generate(n, 0, 0, false, false)
	return s
}
