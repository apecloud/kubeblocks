/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"errors"
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
	"github.com/apecloud/kubeblocks/pkg/kbagent"
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

func ResolveInstanceTemplateImages4ServiceVersion(ctx context.Context, cli client.Reader,
	compDef *appsv1.ComponentDefinition, serviceVersion string) (map[string]string, error) {
	compVersions, err := CompatibleCompVersions4Definition(ctx, cli, compDef)
	if err != nil {
		return nil, err
	}
	if len(compVersions) == 0 {
		return nil, nil
	}
	return resolveImagesWithCompVersions4Template(compDef, compVersions, serviceVersion)
}

func resolveImagesWithCompVersions(compDef *appsv1.ComponentDefinition,
	compVersions []*appsv1.ComponentVersion, serviceVersion string) error {
	appsInDef := convertImagesFromCompDefinition(compDef)
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

func resolveImagesWithCompVersions4Template(compDef *appsv1.ComponentDefinition,
	compVersions []*appsv1.ComponentVersion, serviceVersion string) (map[string]string, error) {
	appsInDef := convertImagesFromCompDefinition(compDef)
	appsInVer, err := findMatchedImagesFromCompVersions(compVersions, serviceVersion)
	if err != nil {
		return nil, err
	}

	images := make(map[string]string)
	apps := checkNMergeImages(serviceVersion, appsInDef, appsInVer)

	if err = func() error {
		checkNUpdateImage := func(name, image string) error {
			var err error
			app, ok := apps[name]
			switch {
			case ok && app.err == nil:
				if len(image) == 0 || image != app.image {
					images[name] = app.image
				}
			case ok:
				err = app.err
			default:
				err = fmt.Errorf("no matched image found for container %s", name)
			}
			return err
		}
		for _, c := range compDef.Spec.Runtime.InitContainers {
			if err := checkNUpdateImage(c.Name, c.Image); err != nil {
				return err
			}
		}
		for _, c := range compDef.Spec.Runtime.Containers {
			if err := checkNUpdateImage(c.Name, c.Image); err != nil {
				return err
			}
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	if err = func() error {
		for name, action := range actionsToResolveImage(compDef) {
			if action != nil && action.Exec != nil {
				if app, ok := apps[name]; ok {
					if app.err != nil {
						return app.err
					}
					if image, ok := images[kbagent.ContainerName]; !ok || len(image) == 0 {
						images[kbagent.ContainerName] = app.image
					}
					if image, ok := images[kbagent.ContainerName4Worker]; !ok || len(image) == 0 {
						images[kbagent.ContainerName4Worker] = app.image
					}
				}
			}
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	return images, nil
}

func convertImagesFromCompDefinition(compDef *appsv1.ComponentDefinition) map[string]appNameVersionImage {
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
	if compDef.Spec.LifecycleActions.AvailableProbe != nil {
		actions[normalize("availableProbe")] = &compDef.Spec.LifecycleActions.AvailableProbe.Action
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

	exactMatchedServiceVersion := func(releases map[string]appNameVersionImage) []string {
		names := make([]string, 0)
		for name, r := range releases {
			if r.version == serviceVersion {
				names = append(names, name)
			}
		}
		return names
	}

	apps := make(map[string]appNameVersionImage)
	for appName, releases := range appsWithReleases {
		releaseNames := maps.Keys(releases)
		if names := exactMatchedServiceVersion(releases); len(names) > 0 {
			releaseNames = names
		}
		slices.Sort(releaseNames)
		// use the latest release
		apps[appName] = releases[releaseNames[len(releaseNames)-1]]
	}

	matched := appNameVersionImage{}
	for name, app := range apps {
		if len(matched.version) > 0 && app.version != matched.version {
			return nil, fmt.Errorf("multiple service versions matched: %v, %v", matched, app)
		}
		matched = apps[name]
	}

	return apps, nil
}

func checkNMergeImages(serviceVersion string, appsInDef, appsInVer map[string]appNameVersionImage) map[string]appNameVersionImage {
	apps := make(map[string]appNameVersionImage)
	merge := func(name string, def, ver appNameVersionImage) appNameVersionImage {
		if len(ver.name) == 0 {
			// if not required, fallback to image in cmpd directly
			if !def.required {
				return def
			}
			match, err := CompareServiceVersion(serviceVersion, def.version)
			if err != nil {
				def.err = fmt.Errorf("failed to compare service version (service version: %s, def version: %s): %w", serviceVersion, def.version, err)
			}
			if !match {
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

// ErrCompDefinitionNServiceVersionUnmatched is wrapped into the error returned by
// ResolveCompDefinitionNServiceVersion when no component definition matches the requested
// name (or name prefix / regex pattern) and service version. The mismatch is deterministic
// w.r.t. the requested inputs and the current set of component definitions and versions,
// so callers may check it with errors.Is to classify the failure as terminal.
var ErrCompDefinitionNServiceVersionUnmatched = errors.New("no matched component definition found")

// ResolveCompDefinitionNServiceVersion resolves and returns the specific component definition object and the service version supported.
func ResolveCompDefinitionNServiceVersion(ctx context.Context, cli client.Reader, compDefName, serviceVersion string) (*appsv1.ComponentDefinition, string, error) {
	var (
		compDef *appsv1.ComponentDefinition
	)
	compDefs, err := listCompDefinitionsWithPattern(ctx, cli, compDefName)
	if err != nil {
		return compDef, serviceVersion, err
	}

	// mapping from <service version> to <[]*appsv1.ComponentDefinition>
	serviceVersionToCompDefs, err := serviceVersionToCompDefinitions(ctx, cli, compDefs, serviceVersion)
	if err != nil {
		return compDef, serviceVersion, err
	}

	// use specified service version or the latest.
	if len(serviceVersion) == 0 {
		serviceVersions := maps.Keys(serviceVersionToCompDefs)
		if len(serviceVersions) > 0 {
			slices.SortFunc(serviceVersions, serviceVersionComparator)
			serviceVersion = serviceVersions[len(serviceVersions)-1]
		}
	}

	// component definitions that support the service version
	compatibleCompDefs := serviceVersionToCompDefs[serviceVersion]
	if len(compatibleCompDefs) == 0 {
		return compDef, serviceVersion, fmt.Errorf(`%w with componentDef "%s" and serviceVersion "%s"`, ErrCompDefinitionNServiceVersionUnmatched, compDefName, serviceVersion)
	}

	// choose the latest one
	compatibleCompDefNames := maps.Keys(compatibleCompDefs)
	slices.Sort(compatibleCompDefNames)
	compatibleCompDefName := compatibleCompDefNames[len(compatibleCompDefNames)-1]

	return compatibleCompDefs[compatibleCompDefName], serviceVersion, nil
}

// listCompDefinitionsWithPattern returns all component definitions whose names match the given pattern
func listCompDefinitionsWithPattern(ctx context.Context, cli client.Reader, name string) ([]*appsv1.ComponentDefinition, error) {
	compDefList := &appsv1.ComponentDefinitionList{}
	if err := cli.List(ctx, compDefList); err != nil {
		return nil, err
	}
	compDefsFullyMatched := make([]*appsv1.ComponentDefinition, 0)
	compDefsPatternMatched := make([]*appsv1.ComponentDefinition, 0)
	for i, item := range compDefList.Items {
		if item.Name == name {
			compDefsFullyMatched = append(compDefsFullyMatched, &compDefList.Items[i])
		}
		if PrefixOrRegexMatched(item.Name, name) {
			compDefsPatternMatched = append(compDefsPatternMatched, &compDefList.Items[i])
		}
	}
	if len(compDefsFullyMatched) > 0 {
		return compDefsFullyMatched, nil
	}
	return compDefsPatternMatched, nil
}

func serviceVersionToCompDefinitions(ctx context.Context, cli client.Reader,
	compDefs []*appsv1.ComponentDefinition, serviceVersion string) (map[string]map[string]*appsv1.ComponentDefinition, error) {
	result := make(map[string]map[string]*appsv1.ComponentDefinition)

	insert := func(version string, compDef *appsv1.ComponentDefinition) {
		if _, ok := result[version]; !ok {
			result[version] = make(map[string]*appsv1.ComponentDefinition)
		}
		result[version][compDef.Name] = compDef
	}

	checkedInsert := func(version string, compDef *appsv1.ComponentDefinition) error {
		match, err := CompareServiceVersion(serviceVersion, version)
		if err == nil && match {
			insert(version, compDef)
		}
		return err
	}

	for _, compDef := range compDefs {
		compVersions, err := CompatibleCompVersions4Definition(ctx, cli, compDef)
		if err != nil {
			return nil, err
		}

		serviceVersions := sets.New[string]()
		// add definition's service version as default, in case there is no component versions provided
		if compDef.Spec.ServiceVersion != "" {
			serviceVersions.Insert(compDef.Spec.ServiceVersion)
		}
		for _, compVersion := range compVersions {
			serviceVersions = serviceVersions.Union(compatibleServiceVersions4Definition(compDef, compVersion))
		}

		for version := range serviceVersions {
			if err = checkedInsert(version, compDef); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// compatibleServiceVersions4Definition returns all service versions that are compatible with specified component definition.
func compatibleServiceVersions4Definition(compDef *appsv1.ComponentDefinition, compVersion *appsv1.ComponentVersion) sets.Set[string] {
	match := func(pattern string) bool {
		return PrefixOrRegexMatched(compDef.Name, pattern)
	}
	releases := make(map[string]bool, 0)
	for _, rule := range compVersion.Spec.CompatibilityRules {
		if slices.IndexFunc(rule.CompDefs, match) >= 0 {
			for _, release := range rule.Releases {
				releases[release] = true
			}
		}
	}
	serviceVersions := sets.New[string]()
	for _, release := range compVersion.Spec.Releases {
		if releases[release.Name] {
			serviceVersions = serviceVersions.Insert(release.ServiceVersion)
		}
	}
	return serviceVersions
}

func serviceVersionComparator(a, b string) int {
	if len(a) == 0 {
		return -1
	}
	if len(b) == 0 {
		return 1
	}
	v, err1 := version.ParseSemantic(a)
	if err1 != nil {
		panic(fmt.Sprintf("runtime error - invalid service version in comparator: %s", err1.Error()))
	}
	ret, err2 := v.Compare(b)
	if err2 != nil {
		panic(fmt.Sprintf("runtime error - invalid service version in comparator: %s", err2.Error()))
	}
	return ret
}
