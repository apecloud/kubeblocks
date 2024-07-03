package instanceset

import (
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// TODO: remove these after extract the Schema of the API types from Kubeblocks into a separate Go package.

// InstanceSetExt serves as a Public Struct,
// used as the type for the input parameters of BuildInstanceTemplateExts.
type InstanceSetExt struct {
	Its               *workloads.InstanceSet
	InstanceTemplates []*workloads.InstanceTemplate
}

// InstanceTemplateExt serves as a Public Struct,
// used as the type for the construction results returned by BuildInstanceTemplateExts.
type InstanceTemplateExt struct {
	Name     string
	Replicas int32
	corev1.PodTemplateSpec
	VolumeClaimTemplates []corev1.PersistentVolumeClaim
}

// BuildInstanceTemplateExts serves as a Public API, through which users can obtain InstanceTemplateExt objects
// processed by the buildInstanceTemplateExts function.
// Its main purpose is to acquire the PodTemplate rendered by InstanceTemplate.
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

// BuildInstanceTemplates serves as a Public API, allowing users to construct InstanceTemplates.
// The constructed InstanceTemplates can be used as part of the input for BuildInstanceTemplateExts.
func BuildInstanceTemplates(totalReplicas int32, instances []workloads.InstanceTemplate, instancesCompressed *corev1.ConfigMap) []*workloads.InstanceTemplate {
	return buildInstanceTemplates(totalReplicas, instances, instancesCompressed)
}
