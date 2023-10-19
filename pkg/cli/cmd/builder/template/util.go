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
	"strings"

	"github.com/sethvargo/go-password/password"
	"helm.sh/helm/v3/pkg/cli/values"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/testing"
	"github.com/apecloud/kubeblocks/pkg/cli/util/helm"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	"github.com/apecloud/kubeblocks/version"
)

type RenderedOptions struct {
	ConfigSpec string

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
			fields := strings.SplitN(renderedOpts.DataVolumeName, ":", 2)
			if len(fields) == 1 {
				fields = append(fields, "10Gi")
			}
			pvcSpec := testapps.NewPVCSpec(fields[1])
			factory.AddVolumeClaimTemplate(fields[0], pvcSpec)
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

func HelmTemplate(helmPath string, helmOutput string) error {
	o := helm.InstallOpts{
		Name:      testing.KubeBlocksChartName,
		Chart:     helmPath,
		Namespace: "default",
		Version:   version.DefaultKubeBlocksVersion,

		DryRun:    func() *bool { r := true; return &r }(),
		OutputDir: helmOutput,
		ValueOpts: &values.Options{Values: []string{}},
	}
	_, err := o.Install(helm.NewFakeConfig("default"))
	return err
}

func checkAndFillPortProtocol(clusterDefComponents []appsv1alpha1.ClusterComponentDefinition) {
	// set a default protocol with 'TCP' to avoid failure in BuildHeadlessSvc
	for i := range clusterDefComponents {
		for j := range clusterDefComponents[i].PodSpec.Containers {
			container := &clusterDefComponents[i].PodSpec.Containers[j]
			for k := range container.Ports {
				port := &container.Ports[k]
				if port.Protocol == "" {
					port.Protocol = corev1.ProtocolTCP
				}
			}
		}
	}
}
