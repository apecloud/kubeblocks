package lifecycle

import (
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// fixClusterLabelsTransformer fill the class related info to cluster
type fillClass struct {
	cc  clusterRefResources
	cli client.Client
	ctx intctrlutil.RequestCtx
}

func (r *fillClass) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)
	return r.fillClass(r.ctx, cluster, r.cc.cd)
}

func (r *fillClass) fillClass(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster, clusterDefinition appsv1alpha1.ClusterDefinition) error {
	var (
		value                 = cluster.GetAnnotations()[constant.ClassAnnotationKey]
		componentClassMapping = make(map[string]string)
		cmList                corev1.ConfigMapList
	)
	if value != "" {
		if err := json.Unmarshal([]byte(value), &componentClassMapping); err != nil {
			return err
		}
	}

	cmLabels := []client.ListOption{
		client.MatchingLabels{constant.ClusterDefLabelKey: clusterDefinition.Name},
		client.HasLabels{constant.ClassProviderLabelKey},
	}
	if err := r.cli.List(reqCtx.Ctx, &cmList, cmLabels...); err != nil {
		return err
	}
	compClasses, err := class.ParseClasses(&cmList)
	if err != nil {
		return err
	}

	var classFamilyList appsv1alpha1.ClassFamilyList
	if err = r.cli.List(reqCtx.Ctx, &classFamilyList); err != nil {
		return err
	}

	matchClassFamilies := func(comp appsv1alpha1.ClusterComponentSpec) *class.ComponentClass {
		var candidates []class.ClassModelWithFamilyName
		for _, family := range classFamilyList.Items {
			models := family.FindMatchingModels(&comp.Resources)
			for _, model := range models {
				candidates = append(candidates, class.ClassModelWithFamilyName{Family: family.Name, Model: model})
			}
		}
		if len(candidates) == 0 {
			return nil
		}
		sort.Sort(class.ByModelList(candidates))
		candidate := candidates[0]
		cpu, memory := class.GetMinCPUAndMemory(candidate.Model)
		cls := &class.ComponentClass{
			Name:   fmt.Sprintf("%s-%vc%vg", candidate.Family, cpu.AsDec().String(), memory.AsDec().String()),
			CPU:    *cpu,
			Memory: *memory,
		}
		return cls
	}

	matchComponentClass := func(comp appsv1alpha1.ClusterComponentSpec, classes map[string]*class.ComponentClass) *class.ComponentClass {
		filters := class.Filters(make(map[string]resource.Quantity))
		if comp.Resources.Requests.Cpu() != nil {
			filters[corev1.ResourceCPU.String()] = *comp.Resources.Requests.Cpu()
		}
		if comp.Resources.Requests.Memory() != nil {
			filters[corev1.ResourceMemory.String()] = *comp.Resources.Requests.Memory()
		}
		return class.ChooseComponentClasses(classes, filters)
	}

	for idx, comp := range cluster.Spec.ComponentSpecs {
		classes := compClasses[comp.ComponentDefRef]

		var cls *class.ComponentClass
		className, ok := componentClassMapping[comp.Name]
		switch {
		case ok:
			cls = classes[className]
			if cls == nil {
				return fmt.Errorf("unknown component class %s", className)
			}
		case classes != nil:
			cls = matchComponentClass(comp, classes)
			if cls == nil {
				return fmt.Errorf("can not find matching class for component %s", comp.Name)
			}
		case len(classFamilyList.Items) > 0:
			cls = matchClassFamilies(comp)
			if cls == nil {
				return fmt.Errorf("can not find matching class family for component %s", comp.Name)
			}
		}
		if cls == nil {
			// TODO reconsider handling policy for this case
			continue
		}
		componentClassMapping[comp.Name] = cls.Name
		corev1.ResourceList{
			corev1.ResourceCPU:    cls.CPU,
			corev1.ResourceMemory: cls.Memory,
		}.DeepCopyInto(&comp.Resources.Requests)
		var volumes []appsv1alpha1.ClusterComponentVolumeClaimTemplate
		for _, disk := range cls.Storage {
			volume := appsv1alpha1.ClusterComponentVolumeClaimTemplate{
				Name: disk.Name,
				Spec: &corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: disk.Size,
						},
					},
				},
			}
			volumes = append(volumes, volume)
		}
		comp.VolumeClaimTemplates = volumes
		cluster.Spec.ComponentSpecs[idx] = comp
	}
	return nil
}
