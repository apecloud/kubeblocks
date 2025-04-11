package instancetemplate

import (
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func BuildInstanceTemplateExts(itsExt *InstanceSetExt) ([]*InstanceTemplateExt, error) {
	instanceTemplatesMap := itsExt.InstanceTemplates
	templates := make([]*InstanceTemplateExt, 0, len(instanceTemplatesMap))
	for templateName := range instanceTemplatesMap {
		tpl := instanceTemplatesMap[templateName]
		tplExt := buildInstanceTemplateExt(tpl, itsExt.InstanceSet)
		templates = append(templates, tplExt)
	}

	return templates, nil
}

func buildInstanceTemplatesMap(its *workloads.InstanceSet, instancesCompressed *corev1.ConfigMap) map[string]*workloads.InstanceTemplate {
	rtn := make(map[string]*workloads.InstanceTemplate)
	l := BuildInstanceTemplates(its, instancesCompressed)
	for _, t := range l {
		rtn[t.Name] = t
	}
	return rtn
}

// BuildInstanceTemplates builds a complate instance template list,
// i.e. append a pseudo template (which equals to `.spec.template`)
// to the end of the list, to fill up the replica count.
// And also if there is any compressed template, add them too.
//
// It is not guaranteed that the returned list is sorted.
// It is assumed that its spec is valid, e.g. replicasInTemplates < totalReplica.
func BuildInstanceTemplates(its *workloads.InstanceSet, instancesCompressed *corev1.ConfigMap) []*workloads.InstanceTemplate {
	var instanceTemplateList []*workloads.InstanceTemplate
	var replicasInTemplates int32
	instanceTemplates := getInstanceTemplates(its.Spec.Instances, instancesCompressed)
	for i := range instanceTemplates {
		instance := &instanceTemplates[i]
		if instance.Replicas == nil {
			instance.Replicas = ptr.To[int32](1)
		}
		instanceTemplateList = append(instanceTemplateList, instance)
		replicasInTemplates += *instance.Replicas
	}
	totalReplicas := *its.Spec.Replicas
	if replicasInTemplates < totalReplicas {
		replicas := totalReplicas - replicasInTemplates
		instance := &workloads.InstanceTemplate{Replicas: &replicas, Ordinals: its.Spec.DefaultTemplateOrdinals}
		instanceTemplateList = append(instanceTemplateList, instance)
	}

	return instanceTemplateList
}

func BuildInstanceSetExt(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (*InstanceSetExt, error) {
	instancesCompressed, err := findTemplateObject(its, tree)
	if err != nil {
		return nil, err
	}

	instanceTemplateMap := buildInstanceTemplatesMap(its, instancesCompressed)

	return &InstanceSetExt{
		InstanceSet:       its,
		InstanceTemplates: instanceTemplateMap,
	}, nil
}
