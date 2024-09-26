/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

var (
	ordinalRegexpPattern = `-\d+$`
	ordinalRegexp        = regexp.MustCompile(ordinalRegexpPattern)

	multiClusterServicePlacementInMirror = "mirror"
	multiClusterServicePlacementInUnique = "unique"
)

// componentServiceTransformer handles component services.
type componentServiceTransformer struct{}

var _ graph.Transformer = &componentServiceTransformer{}

func (t *componentServiceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create service related objects", "component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	synthesizeComp := transCtx.SynthesizeComponent
	runningServices, err := t.listOwnedServices(transCtx.Context, transCtx.Client, transCtx.Component, synthesizeComp)
	if err != nil {
		return err
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, service := range synthesizeComp.ComponentServices {
		// component controller does not handle the default headless service; the default headless service is managed by the InstanceSet.
		if t.skipDefaultHeadlessSvc(synthesizeComp, &service) {
			continue
		}
		services, err := t.buildCompService(transCtx.Component, synthesizeComp, &service)
		if err != nil {
			return err
		}
		for _, svc := range services {
			if err = t.createOrUpdateService(ctx, dag, graphCli, &service, svc, transCtx.ComponentOrig); err != nil {
				return err
			}
			delete(runningServices, svc.Name)
		}
	}

	for svc := range runningServices {
		graphCli.Delete(dag, runningServices[svc], inDataContext4G())
	}

	return nil
}

func (t *componentServiceTransformer) listOwnedServices(ctx context.Context, cli client.Reader,
	comp *appsv1.Component, synthesizedComp *component.SynthesizedComponent) (map[string]*corev1.Service, error) {
	services, err := component.ListOwnedServices(ctx, cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
	if err != nil {
		return nil, err
	}
	owned := make(map[string]*corev1.Service)
	for i, svc := range services {
		if model.IsOwnerOf(comp, svc) {
			owned[svc.Name] = services[i]
		}
	}
	return owned, nil
}

func (t *componentServiceTransformer) buildCompService(comp *appsv1.Component,
	synthesizeComp *component.SynthesizedComponent, service *appsv1.ComponentService) ([]*corev1.Service, error) {
	if service.DisableAutoProvision != nil && *service.DisableAutoProvision {
		return nil, nil
	}

	if t.isPodService(service) {
		return t.buildPodService(comp, synthesizeComp, service)
	}
	return t.buildServices(comp, synthesizeComp, []*appsv1.ComponentService{service})
}

func (t *componentServiceTransformer) isPodService(service *appsv1.ComponentService) bool {
	return service.PodService != nil && *service.PodService
}

func (t *componentServiceTransformer) buildPodService(comp *appsv1.Component,
	synthesizeComp *component.SynthesizedComponent, service *appsv1.ComponentService) ([]*corev1.Service, error) {
	pods, err := t.podsNameNOrdinal(synthesizeComp)
	if err != nil {
		return nil, err
	}

	services := make([]*appsv1.ComponentService, 0)
	for name, ordinal := range pods {
		svc := service.DeepCopy()
		svc.Name = fmt.Sprintf("%s-%d", service.Name, ordinal)
		if len(service.ServiceName) == 0 {
			svc.ServiceName = fmt.Sprintf("%d", ordinal)
		} else {
			svc.ServiceName = fmt.Sprintf("%s-%d", service.ServiceName, ordinal)
		}
		if svc.Spec.Selector == nil {
			svc.Spec.Selector = make(map[string]string)
		}
		svc.Spec.Selector[constant.KBAppPodNameLabelKey] = name
		services = append(services, svc)
	}
	return t.buildServices(comp, synthesizeComp, services)
}

func (t *componentServiceTransformer) podsNameNOrdinal(synthesizeComp *component.SynthesizedComponent) (map[string]int, error) {
	podNames, err := generatePodNames(synthesizeComp)
	if err != nil {
		return nil, err
	}
	pods := make(map[string]int)
	for _, name := range podNames {
		ordinal, err := func() (int, error) {
			result := ordinalRegexp.FindString(name)
			if len(result) == 0 {
				return 0, fmt.Errorf("invalid pod name: %s", name)
			}
			o, _ := strconv.Atoi(result[1:])
			return o, nil
		}()
		if err != nil {
			return nil, err
		}
		pods[name] = ordinal
	}
	return pods, nil
}

func (t *componentServiceTransformer) buildServices(comp *appsv1.Component,
	synthesizeComp *component.SynthesizedComponent, compServices []*appsv1.ComponentService) ([]*corev1.Service, error) {
	services := make([]*corev1.Service, 0, len(compServices))
	for _, compService := range compServices {
		svc, err := t.buildService(comp, synthesizeComp, compService)
		if err != nil {
			return nil, err
		}
		services = append(services, svc)
	}
	return services, nil
}

func (t *componentServiceTransformer) buildService(comp *appsv1.Component,
	synthesizeComp *component.SynthesizedComponent, service *appsv1.ComponentService) (*corev1.Service, error) {
	var (
		namespace   = synthesizeComp.Namespace
		clusterName = synthesizeComp.ClusterName
		compName    = synthesizeComp.Name
	)

	serviceFullName := constant.GenerateComponentServiceName(synthesizeComp.ClusterName, synthesizeComp.Name, service.ServiceName)
	builder := builder.NewServiceBuilder(namespace, serviceFullName).
		AddLabelsInMap(constant.GetCompLabels(clusterName, compName)).
		AddLabelsInMap(synthesizeComp.DynamicLabels).
		AddLabelsInMap(synthesizeComp.StaticLabels).
		AddAnnotationsInMap(service.Annotations).
		AddAnnotationsInMap(synthesizeComp.DynamicAnnotations).
		AddAnnotationsInMap(synthesizeComp.StaticAnnotations).
		SetSpec(&service.Spec).
		AddSelectorsInMap(t.builtinSelector(comp)).
		Optimize4ExternalTraffic()

	if len(service.RoleSelector) > 0 && (service.PodService == nil || !*service.PodService) {
		if err := t.checkRoleSelector(synthesizeComp, service.Name, service.RoleSelector); err != nil {
			return nil, err
		}
		builder.AddSelector(constant.RoleLabelKey, service.RoleSelector)
	}

	svcObj := builder.GetObject()
	if err := setCompOwnershipNFinalizer(comp, svcObj); err != nil {
		return nil, err
	}
	return svcObj, nil
}

func (t *componentServiceTransformer) builtinSelector(comp *appsv1.Component) map[string]string {
	selectors := map[string]string{
		constant.AppManagedByLabelKey:   "",
		constant.AppInstanceLabelKey:    "",
		constant.KBAppComponentLabelKey: "",
	}
	for _, key := range maps.Keys(selectors) {
		if val, ok := comp.Labels[key]; ok {
			selectors[key] = val
		}
	}
	return selectors
}

func (t *componentServiceTransformer) checkRoleSelector(synthesizeComp *component.SynthesizedComponent,
	name string, roleSelector string) error {
	definedRoles := make(map[string]bool)
	for _, role := range synthesizeComp.Roles {
		definedRoles[strings.ToLower(role.Name)] = true
	}
	if !definedRoles[strings.ToLower(roleSelector)] {
		return fmt.Errorf("role selector for service is not defined, service: %s, role: %s", name, roleSelector)
	}
	return nil
}

func (t *componentServiceTransformer) skipDefaultHeadlessSvc(synthesizeComp *component.SynthesizedComponent, service *appsv1.ComponentService) bool {
	svcName := constant.GenerateComponentServiceName(synthesizeComp.ClusterName, synthesizeComp.Name, service.ServiceName)
	defaultHeadlessSvcName := constant.GenerateDefaultComponentHeadlessServiceName(synthesizeComp.ClusterName, synthesizeComp.Name)
	return svcName == defaultHeadlessSvcName
}

func (t *componentServiceTransformer) createOrUpdateService(ctx graph.TransformContext, dag *graph.DAG,
	graphCli model.GraphClient, compService *appsv1.ComponentService, service *corev1.Service, owner client.Object) error {
	var (
		kind       string
		podService = t.isPodService(compService)
	)

	if service.Annotations != nil {
		kind = service.Annotations[constant.MultiClusterServicePlacementKey]
		delete(service.Annotations, constant.MultiClusterServicePlacementKey)
	}
	if podService && len(kind) > 0 && kind != multiClusterServicePlacementInMirror && kind != multiClusterServicePlacementInUnique {
		return fmt.Errorf("invalid multi-cluster pod-service placement kind %s for service %s", kind, service.Name)
	}

	if podService && kind == multiClusterServicePlacementInUnique {
		// create or update service in unique, by hacking the pod placement strategy.
		ordinal := func() int {
			subs := strings.Split(service.GetName(), "-")
			o, _ := strconv.Atoi(subs[len(subs)-1])
			return o
		}
		multicluster.Assign(ctx.GetContext(), service, ordinal)
	}

	createOrUpdateService := func(service *corev1.Service) error {
		key := types.NamespacedName{
			Namespace: service.Namespace,
			Name:      service.Name,
		}
		originSvc := &corev1.Service{}
		if err := ctx.GetClient().Get(ctx.GetContext(), key, originSvc, inDataContext4C()); err != nil {
			if apierrors.IsNotFound(err) {
				graphCli.Create(dag, service, inDataContext4G())
				return nil
			}
			return err
		}

		// don't update service not owned by the owner, to keep compatible with existed cluster
		if !model.IsOwnerOf(owner, originSvc) {
			return nil
		}

		newSvc := originSvc.DeepCopy()
		newSvc.Spec = service.Spec

		// if skip immutable check, update the service directly
		if skipImmutableCheckForComponentService(originSvc) {
			resolveServiceDefaultFields(&originSvc.Spec, &newSvc.Spec)
			if !reflect.DeepEqual(originSvc, newSvc) {
				graphCli.Update(dag, originSvc, newSvc, inDataContext4G())
			}
			return nil
		}
		// otherwise only support to update the override params defined in cluster.spec.componentSpec[].services

		overrideMutableParams := func(originSvc, newSvc *corev1.Service) {
			newSvc.Spec.Type = originSvc.Spec.Type
			newSvc.Name = originSvc.Name
			newSvc.Spec.Selector = originSvc.Spec.Selector
			newSvc.Annotations = originSvc.Annotations
		}

		// modify mutable field of newSvc to check if it is overridable
		overrideMutableParams(originSvc, newSvc)
		if !reflect.DeepEqual(originSvc, newSvc) {
			// other fields are immutable, we can't update the service
			return nil
		}

		overrideMutableParams(service, newSvc)
		if !reflect.DeepEqual(originSvc, newSvc) {
			graphCli.Update(dag, originSvc, newSvc, inDataContext4G())
		}
		return nil
	}
	return createOrUpdateService(service)
}

func skipImmutableCheckForComponentService(svc *corev1.Service) bool {
	if svc.Annotations == nil {
		return false
	}
	skip, ok := svc.Annotations[constant.SkipImmutableCheckAnnotationKey]
	return ok && strings.ToLower(skip) == "true"
}

func generatePodNames(synthesizeComp *component.SynthesizedComponent) ([]string, error) {
	return component.GenerateAllPodNames(synthesizeComp.Replicas, synthesizeComp.Instances,
		synthesizeComp.OfflineInstances, synthesizeComp.ClusterName, synthesizeComp.Name)
}

func generatePodNamesByITS(its *workloads.InstanceSet) ([]string, error) {
	var templates []instanceset.InstanceTemplate
	for i := range its.Spec.Instances {
		templates = append(templates, &its.Spec.Instances[i])
	}
	return instanceset.GenerateAllInstanceNames(its.Name, *its.Spec.Replicas, templates, its.Spec.OfflineInstances, workloads.Ordinals{})
}
