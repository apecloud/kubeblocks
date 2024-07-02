package instanceset

import (
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// TODO: remove these after extract the Schema of the API types from Kubeblocks into a separate Go package.

type InstanceSetExt struct {
	Its               *workloads.InstanceSet
	InstanceTemplates []*workloads.InstanceTemplate
}

type InstanceTemplateExt struct {
	Name     string
	Replicas int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

func BuildInstanceTemplateExts(itsExt *InstanceSetExt) []*InstanceTemplateExt {
	itsExts := buildInstanceTemplateExts(&instanceSetExt{
		its:               itsExt.Its,
		instanceTemplates: itsExt.InstanceTemplates,
	})
	var instanceTemplateExts []*InstanceTemplateExt
	for _, itsExt := range itsExts {
		instanceTemplateExts = append(instanceTemplateExts, (*InstanceTemplateExt)(itsExt))
	}
	return instanceTemplateExts
}

func BuildInstanceTemplates(totalReplicas int32, instances []workloads.InstanceTemplate, instancesCompressed *corev1.ConfigMap) []*workloads.InstanceTemplate {
	return buildInstanceTemplates(totalReplicas, instances, instancesCompressed)
}
