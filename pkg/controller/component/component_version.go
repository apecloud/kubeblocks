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
	"reflect"
	"slices"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// CompatibleCompVersions4Definition returns all component versions that are compatible with specified component definition.
func CompatibleCompVersions4Definition(ctx context.Context, cli client.Reader, compDef *appsv1.ComponentDefinition) ([]*appsv1.ComponentVersion, error) {
	compVersionList := &appsv1.ComponentVersionList{}
	labels := client.MatchingLabels{
		compDef.Name: compDef.Name,
	}
	if err := cli.List(ctx, compVersionList, labels); err != nil {
		return nil, err
	}

	if len(compVersionList.Items) == 0 {
		return nil, nil
	}

	compVersions := make([]*appsv1.ComponentVersion, 0)
	for i, compVersion := range compVersionList.Items {
		if compVersion.Generation != compVersion.Status.ObservedGeneration {
			return nil, fmt.Errorf("the matched ComponentVersion is not up to date: %s", compVersion.Name)
		}
		if compVersion.Status.Phase != appsv1.AvailablePhase {
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
	compDef *appsv1.ComponentDefinition, serviceVersion string) error {
	compVersions, err := CompatibleCompVersions4Definition(ctx, cli, compDef)
	if err != nil {
		return err
	}
	if len(compVersions) == 0 {
		return nil
	}
	return resolveImagesWithCompVersions(compDef, compVersions, serviceVersion)
}

func resolveImagesWithCompVersions(compDef *appsv1.ComponentDefinition,
	compVersions []*appsv1.ComponentVersion, serviceVersion string) error {
	appsInDef := covertImagesFromCompDefinition(compDef)
	appsInVer, err := findMatchedImagesFromCompVersions(compVersions, serviceVersion)
	if err != nil {
		return err
	}

	apps := checkNMergeImages(serviceVersion, appsInDef, appsInVer)

	if err = func() error {
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
	}(); err != nil {
		return err
	}

	if err = func() error {
		for name, action := range actionsToResolveImage(compDef) {
			if action != nil && action.Exec != nil {
				if app, ok := apps[name]; ok {
					if app.err != nil {
						return app.err
					}
					action.Exec.Image = app.image
				}
			}
		}
		return nil
	}(); err != nil {
		return err
	}

	return nil
}

func covertImagesFromCompDefinition(compDef *appsv1.ComponentDefinition) map[string]appNameVersionImage {
	apps := make(map[string]appNameVersionImage)

	// containers
	checkNAdd := func(c *corev1.Container) {
		if len(c.Image) > 0 {
			apps[c.Name] = appNameVersionImage{
				name:     c.Name,
				version:  compDef.Spec.ServiceVersion,
				image:    c.Image,
				required: true,
			}
		}
	}
	for i := range compDef.Spec.Runtime.InitContainers {
		checkNAdd(&compDef.Spec.Runtime.InitContainers[i])
	}
	for i := range compDef.Spec.Runtime.Containers {
		checkNAdd(&compDef.Spec.Runtime.Containers[i])
	}

	// actions
	for name, action := range actionsToResolveImage(compDef) {
		if action != nil && action.Exec != nil {
			apps[name] = appNameVersionImage{
				name:     name,
				version:  compDef.Spec.ServiceVersion,
				image:    action.Exec.Image,
				required: false,
			}
		}
	}

	return apps
}

func actionsToResolveImage(compDef *appsv1.ComponentDefinition) map[string]*appsv1.Action {
	if compDef.Spec.LifecycleActions == nil {
		return nil
	}

	normalize := strings.ToLower
	actions := map[string]*appsv1.Action{
		normalize("postProvision"):    compDef.Spec.LifecycleActions.PostProvision,
		normalize("preTerminate"):     compDef.Spec.LifecycleActions.PreTerminate,
		normalize("switchover"):       compDef.Spec.LifecycleActions.Switchover,
		normalize("memberJoin"):       compDef.Spec.LifecycleActions.MemberJoin,
		normalize("memberLeave"):      compDef.Spec.LifecycleActions.MemberLeave,
		normalize("readonly"):         compDef.Spec.LifecycleActions.Readonly,
		normalize("readwrite"):        compDef.Spec.LifecycleActions.Readwrite,
		normalize("dataDump"):         compDef.Spec.LifecycleActions.DataDump,
		normalize("dataLoad"):         compDef.Spec.LifecycleActions.DataLoad,
		normalize("reconfigure"):      compDef.Spec.LifecycleActions.Reconfigure,
		normalize("accountProvision"): compDef.Spec.LifecycleActions.AccountProvision,
	}
	if compDef.Spec.LifecycleActions.RoleProbe != nil {
		actions[normalize("roleProbe")] = &compDef.Spec.LifecycleActions.RoleProbe.Action
	}
	return actions
}

func findMatchedImagesFromCompVersions(compVersions []*appsv1.ComponentVersion, serviceVersion string) (map[string]appNameVersionImage, error) {
	normalize := func() func(string) (bool, string) {
		names := sets.New[string]()
		tp := reflect.TypeOf(appsv1.ComponentLifecycleActions{})
		for i := 0; i < tp.NumField(); i++ {
			names.Insert(strings.ToLower(tp.Field(i).Name))
		}
		return func(name string) (bool, string) {
			l := strings.ToLower(name)
			if names.Has(l) {
				return true, l
			}
			return false, name
		}
	}()

	appsWithReleases := make(map[string]map[string]appNameVersionImage)
	for _, compVersion := range compVersions {
		for _, release := range compVersion.Spec.Releases {
			match, err := CompareServiceVersion(serviceVersion, release.ServiceVersion)
			if err != nil {
				return nil, err
			}
			if match {
				for name, image := range release.Images {
					isAction, appName := normalize(name)
					if _, ok := appsWithReleases[appName]; !ok {
						appsWithReleases[appName] = make(map[string]appNameVersionImage)
					}
					appsWithReleases[appName][release.Name] = appNameVersionImage{
						name:     appName,
						version:  release.ServiceVersion,
						image:    image,
						required: !isAction,
					}
				}
			}
		}
	}

	apps := make(map[string]appNameVersionImage)
	for appName, releases := range appsWithReleases {
		releaseNames := maps.Keys(releases)
		slices.Sort(releaseNames)
		// use the latest release
		apps[appName] = releases[releaseNames[len(releaseNames)-1]]
	}
	return apps, nil
}

func checkNMergeImages(serviceVersion string, appsInDef, appsInVer map[string]appNameVersionImage) map[string]appNameVersionImage {
	apps := make(map[string]appNameVersionImage)
	merge := func(name string, def, ver appNameVersionImage) appNameVersionImage {
		if len(ver.name) == 0 {
			match, err := CompareServiceVersion(serviceVersion, def.version)
			if err != nil {
				def.err = fmt.Errorf("failed to compare service version (service version: %s, def version: %s): %w", serviceVersion, def.version, err)
			}
			if !match && def.required {
				def.err = fmt.Errorf("no matched image found for container %s with required version %s", name, serviceVersion)
			}
			return def
		}
		return ver
	}
	for _, name := range append(maps.Keys(appsInDef), maps.Keys(appsInVer)...) {
		apps[name] = merge(name, appsInDef[name], appsInVer[name])
	}
	return apps
}

type appNameVersionImage struct {
	name     string
	version  string
	image    string
	err      error
	required bool
}
