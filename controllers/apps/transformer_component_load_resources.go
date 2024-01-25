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

package apps

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentLoadResourcesTransformer handles referenced resources validation and load them into context
type componentLoadResourcesTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentLoadResourcesTransformer{}

func (t *componentLoadResourcesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	comp := transCtx.Component

	var err error
	defer func() {
		setProvisioningStartedCondition(&comp.Status.Conditions, comp.Name, comp.Generation, err)
	}()

	clusterName, err := component.GetClusterName(comp)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	cluster := &appsv1alpha1.Cluster{}
	err = transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: clusterName, Namespace: comp.Namespace}, cluster)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	transCtx.Cluster = cluster

	generated := false
	generated, err = isGeneratedComponent(cluster, comp)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if generated {
		return t.transformForGeneratedComponent(transCtx)
	}
	return t.transformForNativeComponent(transCtx)
}

func (t *componentLoadResourcesTransformer) transformForGeneratedComponent(transCtx *componentTransformContext) error {
	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	comp := transCtx.Component

	compDef, synthesizedComp, err := component.BuildSynthesizedComponent4Generated(reqCtx, transCtx.Client, transCtx.Cluster, comp)
	if err != nil {
		message := fmt.Sprintf("build synthesized component for %s failed: %s", comp.Name, err.Error())
		return newRequeueError(requeueDuration, message)
	}
	transCtx.CompDef = compDef
	transCtx.SynthesizeComponent = synthesizedComp

	return nil
}

func (t *componentLoadResourcesTransformer) transformForNativeComponent(transCtx *componentTransformContext) error {
	compDef, err := t.getNCheckCompDef(transCtx)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	if err = t.resolveComponentVersion(transCtx, compDef); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	transCtx.CompDef = compDef

	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	comp := transCtx.Component
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, transCtx.Client, transCtx.Cluster, compDef, comp)
	if err != nil {
		message := fmt.Sprintf("build synthesized component for %s failed: %s", comp.Name, err.Error())
		return newRequeueError(requeueDuration, message)
	}
	transCtx.SynthesizeComponent = synthesizedComp

	return nil
}

func (t *componentLoadResourcesTransformer) getNCheckCompDef(transCtx *componentTransformContext) (*appsv1alpha1.ComponentDefinition, error) {
	compKey := types.NamespacedName{
		Namespace: transCtx.Component.Namespace,
		Name:      transCtx.Component.Spec.CompDef,
	}
	compDef := &appsv1alpha1.ComponentDefinition{}
	if err := transCtx.Client.Get(transCtx.Context, compKey, compDef); err != nil {
		return nil, err
	}
	if compDef.Status.Phase != appsv1alpha1.AvailablePhase {
		return nil, fmt.Errorf("ComponentDefinition referenced is unavailable: %s", compDef.Name)
	}
	return compDef, nil
}

func (t *componentLoadResourcesTransformer) resolveComponentVersion(transCtx *componentTransformContext,
	compDef *appsv1alpha1.ComponentDefinition) error {
	compVersions, err := t.getCompatibleCompVersions(transCtx.Context, compDef)
	if err != nil {
		return err
	}
	if len(compVersions) == 0 {
		return nil
	}
	return t.resolveImagesWithCompVersions(compVersions, transCtx.Component, compDef)
}

func (t *componentLoadResourcesTransformer) getCompatibleCompVersions(ctx context.Context,
	compDef *appsv1alpha1.ComponentDefinition) ([]*appsv1alpha1.ComponentVersion, error) {
	compVersionList := &appsv1alpha1.ComponentVersionList{}
	labels := client.MatchingLabels{
		compDef.Name: compDef.Name,
	}
	if err := t.Client.List(ctx, compVersionList, labels); err != nil {
		return nil, err
	}

	if len(compVersionList.Items) == 0 {
		return nil, nil
	}

	compVersions := make([]*appsv1alpha1.ComponentVersion, 0)
	for i, compVersion := range compVersionList.Items {
		if compVersion.Status.Phase != appsv1alpha1.AvailablePhase {
			return nil, fmt.Errorf("matched ComponentVersion %s is not available", compVersion.Name)
		}
		compVersions = append(compVersions, &compVersionList.Items[i])
	}
	return compVersions, nil
}

func (t *componentLoadResourcesTransformer) resolveImagesWithCompVersions(compVersions []*appsv1alpha1.ComponentVersion,
	comp *appsv1alpha1.Component, compDef *appsv1alpha1.ComponentDefinition) error {
	appsInDef := t.covertImagesFromCompDefinition(compDef)
	appsByUser := t.findMatchedImagesFromCompVersions(compVersions, comp.Spec.ServiceVersion)

	apps := t.checkNMergeImages(comp.Spec.ServiceVersion, appsInDef, appsByUser)

	checkNUpdateImage := func(c *corev1.Container) error {
		var err error
		app, ok := apps[c.Name]
		switch {
		case ok && app.err == nil:
			c.Image = app.image
		case ok:
			err = app.err
		default:
			err = fmt.Errorf("no matched image found for container %s", c.Name)
		}
		return err
	}

	for i := range compDef.Spec.Runtime.InitContainers {
		if err := checkNUpdateImage(&compDef.Spec.Runtime.InitContainers[i]); err != nil {
			return err
		}
	}
	for i := range compDef.Spec.Runtime.Containers {
		if err := checkNUpdateImage(&compDef.Spec.Runtime.Containers[i]); err != nil {
			return err
		}
	}
	return nil
}

func (t *componentLoadResourcesTransformer) covertImagesFromCompDefinition(compDef *appsv1alpha1.ComponentDefinition) map[string]appNameVersionImage {
	apps := make(map[string]appNameVersionImage)
	checkNAdd := func(c *corev1.Container) {
		if len(c.Image) > 0 {
			apps[c.Name] = appNameVersionImage{
				name:    c.Name,
				version: compDef.Spec.ServiceVersion,
				image:   c.Image,
			}
		}
	}
	for i := range compDef.Spec.Runtime.InitContainers {
		checkNAdd(&compDef.Spec.Runtime.InitContainers[i])
	}
	for i := range compDef.Spec.Runtime.Containers {
		checkNAdd(&compDef.Spec.Runtime.Containers[i])
	}
	return apps
}

func (t *componentLoadResourcesTransformer) findMatchedImagesFromCompVersions(
	compVersions []*appsv1alpha1.ComponentVersion, serviceVersion string) map[string]appNameVersionImage {
	appsWithReleases := make(map[string]map[string]appNameVersionImage)
	for _, compVersion := range compVersions {
		for _, release := range compVersion.Spec.Releases {
			if compareServiceVersion(serviceVersion, release.ServiceVersion) {
				for name, image := range release.Images {
					if _, ok := appsWithReleases[name]; !ok {
						appsWithReleases[name] = make(map[string]appNameVersionImage)
					}
					appsWithReleases[name][release.Name] = appNameVersionImage{
						name:    name,
						version: release.ServiceVersion,
						image:   image,
					}
				}
			}
		}
	}
	apps := make(map[string]appNameVersionImage)
	for name, releases := range appsWithReleases {
		names := maps.Keys(releases)
		slices.Sort(names)
		// use the latest release
		apps[name] = releases[names[len(names)-1]]
	}
	return apps
}

func (t *componentLoadResourcesTransformer) checkNMergeImages(serviceVersion string,
	appsInDef, appsByUser map[string]appNameVersionImage) map[string]appNameVersionImage {
	apps := make(map[string]appNameVersionImage)
	merge := func(name string, def, user appNameVersionImage) appNameVersionImage {
		if len(user.name) == 0 {
			if !compareServiceVersion(serviceVersion, def.version) {
				def.err = fmt.Errorf("no matched image found for container %s with required version %s", name, serviceVersion)
			}
			return def
		}
		return user
	}
	for _, name := range append(maps.Keys(appsInDef), maps.Keys(appsByUser)...) {
		apps[name] = merge(name, appsByUser[name], appsByUser[name])
	}
	return apps
}

type appNameVersionImage struct {
	name    string
	version string
	image   string
	err     error
}

func isGeneratedComponent(cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component) (bool, error) {
	compName, err := component.ShortName(cluster.Name, comp.Name)
	if err != nil {
		return false, err
	}
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if compSpec.Name == compName {
			if len(compSpec.ComponentDef) > 0 {
				if !strings.HasPrefix(comp.Spec.CompDef, compSpec.ComponentDef) {
					err = fmt.Errorf("component definitions referred in cluster and component are different: %s vs %s",
						compSpec.ComponentDef, comp.Spec.CompDef)
				}
				return false, err
			}
			return true, nil
		}
	}
	return true, fmt.Errorf("component %s is not found in cluster %s", compName, cluster.Name)
}

// TODO
func compareServiceVersion(required, provide string) bool {
	ret, err := version.MustParseSemantic(required).Compare(provide)
	return err == nil && ret == 0
}
