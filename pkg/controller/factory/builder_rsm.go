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

// BuildRSMWrapper builds a ReplicatedStateMachine object based on Cluster, SynthesizedComponent.
func BuildRSMWrapper(cluster *appsv1alpha1.Cluster, synthesizeComp *component.SynthesizedComponent) (*workloads.ReplicatedStateMachine, error) {
	if synthesizeComp.ClusterCompDefName == "" {
		// build rsm from new ClusterDefinition API, and convert componentDefinition attributes to rsm attributes.
		return BuildRSMFromConvertor(cluster, synthesizeComp)
	}

	// build rsm from old ClusterDefinition API
	return BuildRSM(cluster, synthesizeComp)
}

// BuildRSMFromConvertor builds a ReplicatedStateMachine object based on the new ComponentDefinition and Component API, and does not depend on the deprecated fields in the SynthesizedComponent.
func BuildRSMFromConvertor(cluster *appsv1alpha1.Cluster, synthesizeComp *component.SynthesizedComponent) (*workloads.ReplicatedStateMachine, error) {
	commonLabels := constant.GetKBWellKnownLabelsWithCompDef(synthesizeComp.CompDefName, cluster.Name, synthesizeComp.Name)

	// TODO(xingran): Need to review how to set pod labels based on the new ComponentDefinition API. workloadType label has been removed.
	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(commonLabels).
		AddLabelsInMap(constant.GetComponentDefLabel(synthesizeComp.CompDefName)).
		AddLabelsInMap(constant.GetAppVersionLabel(synthesizeComp.CompDefName))

	template := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *synthesizeComp.PodSpec,
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

	// TODO(xingran): call convertors to convert componentDef attributes to  attributes. including service, credential, roles, roleProbe, membershipReconfiguration, memberUpdateStrategy, etc.
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

// BuildRSM builds a ReplicatedStateMachine object based on the old CLusterDefinition API, and depends on the the deprecated fields in the SynthesizedComponent.
// TODO(xingran): This function will be deprecated in the future, and the BuildRSMBaseOnCompDef function will be used instead.
func BuildRSM(cluster *appsv1alpha1.Cluster, component *component.SynthesizedComponent) (*workloads.ReplicatedStateMachine, error) {
	commonLabels := constant.GetKBWellKnownLabels(component.ClusterDefName, cluster.Name, component.Name)

	podBuilder := builder.NewPodBuilder("", "").
		AddLabelsInMap(commonLabels).
		AddLabelsInMap(constant.GetClusterCompDefLabel(component.ClusterCompDefName)).
		AddLabelsInMap(constant.GetWorkloadTypeLabel(string(component.WorkloadType)))
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		podBuilder.AddLabelsInMap(constant.GetClusterVersionLabel(cluster.Spec.ClusterVersionRef))
	}
	template := corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *component.PodSpec,
	}

	monitorAnnotations := getMonitorAnnotations(component)
	rsmName := constant.GenerateRSMNamePattern(cluster.Name, component.Name)
	rsmBuilder := builder.NewReplicatedStateMachineBuilder(cluster.Namespace, rsmName).
		AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
		AddAnnotationsInMap(monitorAnnotations).
		AddLabelsInMap(commonLabels).
		AddLabelsInMap(constant.GetClusterCompDefLabel(component.ClusterCompDefName)).
		AddMatchLabelsInMap(commonLabels).
		SetServiceName(constant.GenerateRSMServiceNamePattern(rsmName)).
		SetReplicas(component.Replicas).
		SetTemplate(template)

	var vcts []corev1.PersistentVolumeClaim
	for _, vct := range component.VolumeClaimTemplates {
		vcts = append(vcts, vctToPVC(vct))
	}
	rsmBuilder.SetVolumeClaimTemplates(vcts...)

	if component.StatefulSetWorkload != nil {
		podManagementPolicy, updateStrategy := component.StatefulSetWorkload.FinalStsUpdateStrategy()
		rsmBuilder.SetPodManagementPolicy(podManagementPolicy).SetUpdateStrategy(updateStrategy)
	}

	service, alternativeServices := separateServices(component.Services)
	addServiceCommonLabels(service, commonLabels, component.ClusterCompDefName)
	for i := range alternativeServices {
		addServiceCommonLabels(&alternativeServices[i], commonLabels, component.ClusterCompDefName)
	}
	if service != nil {
		rsmBuilder.SetService(service)
	}
	if len(alternativeServices) == 0 {
		alternativeServices = nil
	}
	alternativeServices = fixService(cluster.Namespace, rsmName, component, alternativeServices...)
	rsmBuilder.SetAlternativeServices(alternativeServices)

	secretName := constant.GenerateDefaultConnCredential(cluster.Name)
	credential := workloads.Credential{
		Username: workloads.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountNameForSecret,
				},
			},
		},
		Password: workloads.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountPasswdForSecret,
				},
			},
		},
	}
	rsmBuilder.SetCredential(credential)

	roles, roleProbe, membershipReconfiguration, memberUpdateStrategy := buildRoleInfo(component)
	rsm := rsmBuilder.SetRoles(roles).
		SetRoleProbe(roleProbe).
		SetMembershipReconfiguration(membershipReconfiguration).
		SetMemberUpdateStrategy(memberUpdateStrategy).
		GetObject()

	// update sts.spec.volumeClaimTemplates[].metadata.labels
	if len(rsm.Spec.VolumeClaimTemplates) > 0 && len(rsm.GetLabels()) > 0 {
		for index, vct := range rsm.Spec.VolumeClaimTemplates {
			BuildPersistentVolumeClaimLabels(component, &vct, vct.Name)
			rsm.Spec.VolumeClaimTemplates[index] = vct
		}
	}

	if err := processContainersInjection(cluster, component, &rsm.Spec.Template.Spec); err != nil {
		return nil, err
	}
	return rsm, nil
}

func vctToPVC(vct corev1.PersistentVolumeClaimTemplate) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: vct.ObjectMeta,
		Spec:       vct.Spec,
	}
}

// addServiceCommonLabels adds labels to the service.
func addServiceCommonLabels(service *corev1.Service, commonLabels map[string]string, compDefName string) {
	if service == nil {
		return
	}
	labels := service.Labels
	if labels == nil {
		labels = make(map[string]string, 0)
	}
	for k, v := range commonLabels {
		labels[k] = v
	}
	labels[constant.AppComponentLabelKey] = compDefName
	service.Labels = labels
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
