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

package factory

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

// BuildRSM builds a ReplicatedStateMachine object based on Cluster, SynthesizedComponent.
func BuildRSM(cluster *appsv1alpha1.Cluster, synthesizeComp *component.SynthesizedComponent) (*workloads.ReplicatedStateMachine, error) {
	commonLabels := constant.GetKBWellKnownLabelsWithCompDef(synthesizeComp.CompDefName, cluster.Name, synthesizeComp.Name)

	// TODO(xingran): Need to review how to set pod labels based on the new ComponentDefinition API. workloadType label has been removed.
	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(commonLabels).
		AddLabelsInMap(constant.GetComponentDefLabel(synthesizeComp.CompDefName)).
		AddLabelsInMap(constant.GetAppVersionLabel(synthesizeComp.CompDefName))

	template := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *synthesizeComp.PodSpec.DeepCopy(),
	}

	monitorAnnotations := getMonitorAnnotations(synthesizeComp)
	rsmName := constant.GenerateRSMNamePattern(cluster.Name, synthesizeComp.Name)
	rsmBuilder := builder.NewReplicatedStateMachineBuilder(cluster.Namespace, rsmName).
		AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
		AddAnnotationsInMap(monitorAnnotations).
		AddLabelsInMap(commonLabels).
		AddLabelsInMap(constant.GetComponentDefLabel(synthesizeComp.CompDefName)).
		AddMatchLabelsInMap(commonLabels).
		SetServiceName(constant.GenerateRSMServiceNamePattern(rsmName)).
		SetReplicas(synthesizeComp.Replicas).
		SetTemplate(template)

	var vcts []corev1.PersistentVolumeClaim
	for _, vct := range synthesizeComp.VolumeClaimTemplates {
		vcts = append(vcts, vctToPVC(vct))
	}
	rsmBuilder.SetVolumeClaimTemplates(vcts...)

	// convert componentDef attributes to rsm attributes. including service, credential, roles, roleProbe, membershipReconfiguration, memberUpdateStrategy, etc.
	convertedRSM, err := component.BuildRSMFrom(cluster, synthesizeComp, rsmBuilder.GetObject())
	if err != nil {
		return nil, err
	}

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	// TODO(xingran): synthesizeComp.VolumeTypes has been removed, and the following code needs to be refactored.
	if len(convertedRSM.Spec.VolumeClaimTemplates) > 0 && len(convertedRSM.GetLabels()) > 0 {
		for index, vct := range convertedRSM.Spec.VolumeClaimTemplates {
			BuildPersistentVolumeClaimLabels(synthesizeComp, &vct, vct.Name)
			convertedRSM.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	if err := processContainersInjection(cluster, synthesizeComp, &convertedRSM.Spec.Template.Spec); err != nil {
		return nil, err
	}

	return convertedRSM, nil
}

func vctToPVC(vct corev1.PersistentVolumeClaimTemplate) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: vct.ObjectMeta,
		Spec:       vct.Spec,
	}
}

// getMonitorAnnotations returns the annotations for the monitor.
func getMonitorAnnotations(synthesizeComp *component.SynthesizedComponent) map[string]string {
	annotations := make(map[string]string, 0)
	falseStr := "false"
	trueStr := "true"
	switch {
	case !synthesizeComp.Monitor.Enable:
		annotations["monitor.kubeblocks.io/scrape"] = falseStr
		annotations["monitor.kubeblocks.io/agamotto"] = falseStr
	case synthesizeComp.Monitor.BuiltIn:
		annotations["monitor.kubeblocks.io/scrape"] = falseStr
		annotations["monitor.kubeblocks.io/agamotto"] = trueStr
	default:
		annotations["monitor.kubeblocks.io/scrape"] = trueStr
		annotations["monitor.kubeblocks.io/path"] = synthesizeComp.Monitor.ScrapePath
		annotations["monitor.kubeblocks.io/port"] = strconv.Itoa(int(synthesizeComp.Monitor.ScrapePort))
		annotations["monitor.kubeblocks.io/scheme"] = "http"
		annotations["monitor.kubeblocks.io/agamotto"] = falseStr
	}
	return rsm.AddAnnotationScope(rsm.HeadlessServiceScope, annotations)
}
