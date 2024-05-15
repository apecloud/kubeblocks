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

package component

import (
	"context"
	"fmt"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// CompatibleCompVersions4Definition returns all component versions that are compatible with specified component definition.
func CompatibleCompVersions4Definition(ctx context.Context, cli client.Reader, compDef *appsv1alpha1.ComponentDefinition) ([]*appsv1alpha1.ComponentVersion, error) {
	compVersionList := &appsv1alpha1.ComponentVersionList{}
	labels := client.MatchingLabels{
		compDef.Name: compDef.Name,
	}
	if err := cli.List(ctx, compVersionList, labels); err != nil {
		return nil, err
	}

	if len(compVersionList.Items) == 0 {
		return nil, nil
	}

	compVersions := make([]*appsv1alpha1.ComponentVersion, 0)
	for i, compVersion := range compVersionList.Items {
		if compVersion.Generation != compVersion.Status.ObservedGeneration {
			return nil, fmt.Errorf("the matched ComponentVersion is not up to date: %s", compVersion.Name)
		}
		if compVersion.Status.Phase != appsv1alpha1.AvailablePhase {
			return nil, fmt.Errorf("the matched ComponentVersion is unavailable: %s", compVersion.Name)
		}
		compVersions = append(compVersions, &compVersionList.Items[i])
	}
	return compVersions, nil
}

// CompareServiceVersion compares whether two service version have the same major, minor and patch version.
func CompareServiceVersion(required, provided string) (bool, error) {
	if len(required) == 0 {
		return true, nil
	}
	rv, err1 := version.ParseSemantic(required)
	if err1 != nil {
		return false, err1
	}
	pv, err2 := version.ParseSemantic(provided)
	if err2 != nil {
		return false, err2
	}
	ret, _ := rv.WithPreRelease("").Compare(pv.WithPreRelease("").String())
	if ret != 0 {
		return false, nil
	}
	if len(rv.PreRelease()) == 0 {
		return true, nil
	}
	// required version has specified the pre-release, so the provided version should match it exactly
	ret, _ = rv.Compare(provided)
	return ret == 0, nil
}

// UpdateCompDefinitionImages4ServiceVersion resolves and updates images for the component definition.
func UpdateCompDefinitionImages4ServiceVersion(ctx context.Context, cli client.Reader,
	compDef *appsv1alpha1.ComponentDefinition, serviceVersion string) error {
	compVersions, err := CompatibleCompVersions4Definition(ctx, cli, compDef)
	if err != nil {
		return err
	}
	if len(compVersions) == 0 {
		return nil
	}
	return resolveImagesWithCompVersions(compDef, compVersions, serviceVersion)
}

func resolveImagesWithCompVersions(compDef *appsv1alpha1.ComponentDefinition,
	compVersions []*appsv1alpha1.ComponentVersion, serviceVersion string) error {
	appsInDef := covertImagesFromCompDefinition(compDef)
	appsByUser, err := findMatchedImagesFromCompVersions(compVersions, serviceVersion)
	if err != nil {
		return err
	}

	apps := checkNMergeImages(serviceVersion, appsInDef, appsByUser)

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

func covertImagesFromCompDefinition(compDef *appsv1alpha1.ComponentDefinition) map[string]appNameVersionImage {
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

func findMatchedImagesFromCompVersions(compVersions []*appsv1alpha1.ComponentVersion, serviceVersion string) (map[string]appNameVersionImage, error) {
	appsWithReleases := make(map[string]map[string]appNameVersionImage)
	for _, compVersion := range compVersions {
		for _, release := range compVersion.Spec.Releases {
			match, err := CompareServiceVersion(serviceVersion, release.ServiceVersion)
			if err != nil {
				return nil, err
			}
			if match {
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
	return apps, nil
}

func checkNMergeImages(serviceVersion string, appsInDef, appsByUser map[string]appNameVersionImage) map[string]appNameVersionImage {
	apps := make(map[string]appNameVersionImage)
	merge := func(name string, def, user appNameVersionImage) appNameVersionImage {
		if len(user.name) == 0 {
			match, err := CompareServiceVersion(serviceVersion, def.version)
			if err != nil {
				def.err = err
			}
			if !match {
				def.err = fmt.Errorf("no matched image found for container %s with required version %s", name, serviceVersion)
			}
			return def
		}
		return user
	}
	for _, name := range append(maps.Keys(appsInDef), maps.Keys(appsByUser)...) {
		apps[name] = merge(name, appsInDef[name], appsByUser[name])
	}
	return apps
}

type appNameVersionImage struct {
	name    string
	version string
	image   string
	err     error
}
