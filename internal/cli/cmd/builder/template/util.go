/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package template

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
			pvcSpec := testapps.NewPVCSpec("10Gi")
			factory.AddVolumeClaimTemplate(renderedOpts.DataVolumeName, pvcSpec)
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
