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

package consensusset

import (
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
)

type objectGenerationTransformer struct {

}

func (t *objectGenerationTransformer) Transform(dag *graph.DAG) error {
	// get root vertex(i.e. consensus set)
	root, err := model.FindRootVertex(dag)
	if err != nil {
		return err
	}
	csSet, _ := root.Obj.(*workloads.ConsensusSet)

	// generate objects by current spec
	svc := builder.NewServiceBuilder(csSet.Namespace, csSet.Namespace).
		AddLabels(constant.AppInstanceLabelKey, csSet.Name).
		AddLabels(constant.AppManagedByLabelKey, ConsensusSetKind).
		AddAnnotationsInMap(item.Annotations).
		AddSelectors(constant.AppInstanceLabelKey, csSet.Name).
		AddSelectors(constant.AppManagedByLabelKey, ConsensusSetKind).
		AddPorts(csSet.Spec.Service.Ports...).
		SetType(csSet.Spec.Service.Type).
		GetObject()
	builder := builder.NewHeadlessServiceBuilder(csSet.Namespace, csSet.Name + "-headless").
		AddLabels(constant.AppNameLabelKey, component.ClusterDefName).
		AddLabels(constant.AppInstanceLabelKey, cluster.Name).
		AddLabels(constant.AppManagedByLabelKey, constant.AppName).
		AddLabels(constant.KBAppComponentLabelKey, component.Name).
		AddAnnotations("prometheus.io/scrape", strconv.FormatBool(component.Monitor.Enable))
	if component.Monitor.Enable {
		builder.AddAnnotations("prometheus.io/path", component.Monitor.ScrapePath).
			AddAnnotations("prometheus.io/port", strconv.Itoa(int(component.Monitor.ScrapePort))).
			AddAnnotations("prometheus.io/scheme", "http")
	}
	builder.AddSelectors(constant.AppInstanceLabelKey, cluster.Name).
		AddSelectors(constant.AppManagedByLabelKey, constant.AppName).
		AddSelectors(constant.KBAppComponentLabelKey, component.Name)
	for _, container := range component.PodSpec.Containers {
		for _, port := range container.Ports {
			servicePort := corev1.ServicePort{
				Name: port.Name,
				Protocol: port.Protocol,
				Port: port.ContainerPort,
				TargetPort: intstr.FromString(port.Name),
			}
			builder.AddPorts(servicePort)
		}
	}
	headLessSvc := builder.GetObject()
	// svc, err := serviceBuilder.New().setName.setType.Build
	// headLessSvc, err :=
	// sts, err := stsBuilder.New().
	// envConfigMap, err := cmBuilder.New().

	// read cache snapshot
	// compute create/update/delete set
	// update dag by root vertex's status
return nil
}