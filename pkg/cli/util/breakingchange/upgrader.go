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

package breakingchange

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

var upgradeHandlerMapper = map[string]upgradeHandlerRecorder{}

type upgradeHandlerRecorder struct {
	formVersions []string
	handler      upgradeHandler
}

type upgradeHandler interface {
	// snapshot the resources of old version and return a map which key is namespace and value is the resources in the namespace.
	snapshot(dynamic dynamic.Interface) (map[string][]unstructured.Unstructured, error)

	// transform the resources of old version to new resources of the new version.
	transform(dynamic dynamic.Interface, resourcesMap map[string][]unstructured.Unstructured) error
}

// registerUpgradeHandler registers the breakingChange handlers to upgradeHandlerMapper.
// the version format should contain "MAJOR.MINOR", such as "0.6".
func registerUpgradeHandler(fromVersions []string, toVersion string, upgradeHandler upgradeHandler) {
	formatErr := func(version string) error {
		return fmt.Errorf("the version %s is incorrect format, at least contains MAJOR and MINOR, such as MAJOR.MINOR", version)
	}

	var majorMinorFromVersions []string
	for _, v := range fromVersions {
		majorMinorFromVersion := getMajorMinorVersion(v)
		if majorMinorFromVersion == "" {
			panic(formatErr(v))
		}
		majorMinorFromVersions = append(majorMinorFromVersions, majorMinorFromVersion)
	}

	majorMinorToVersion := getMajorMinorVersion(toVersion)
	if majorMinorToVersion == "" {
		panic(formatErr(toVersion))
	}
	upgradeHandlerMapper[majorMinorToVersion] = upgradeHandlerRecorder{
		formVersions: majorMinorFromVersions,
		handler:      upgradeHandler,
	}
}

// getUpgradeHandler gets the upgrade handler according to fromVersion and toVersion from upgradeHandlerMapper.
func getUpgradeHandler(fromVersion, toVersion string) upgradeHandler {
	majorMinorFromVersion := getMajorMinorVersion(fromVersion)
	majorMinorToVersion := getMajorMinorVersion(toVersion)
	handlerRecorder, ok := upgradeHandlerMapper[majorMinorToVersion]
	if !ok {
		return nil
	}
	// if no upgrade handler found, ignore
	if !slices.Contains(handlerRecorder.formVersions, majorMinorFromVersion) {
		return nil
	}
	return handlerRecorder.handler
}

// ValidateUpgradeVersion verifies the legality of the upgraded version.
func ValidateUpgradeVersion(fromVersion, toVersion string) error {
	handler := getUpgradeHandler(fromVersion, toVersion)
	if handler != nil {
		// if exists upgrade handler, validation pass.
		return nil
	}
	fromVersionSlice := strings.Split(fromVersion, ".")
	toVersionSlice := strings.Split(toVersion, ".")
	if len(fromVersionSlice) < 2 || len(toVersionSlice) < 2 {
		panic("unreachable, incorrect version format")
	}
	// can not upgrade across major versions by default.
	if fromVersionSlice[0] != toVersionSlice[0] {
		return fmt.Errorf("cannot upgrade across major versions")
	}
	fromMinorVersion, err := strconv.Atoi(fromVersionSlice[1])
	if err != nil {
		return err
	}
	toMinorVersion, err := strconv.Atoi(toVersionSlice[1])
	if err != nil {
		return err
	}
	if (toMinorVersion - fromMinorVersion) > 1 {
		return fmt.Errorf("cannot upgrade across 1 minor version, you can upgrade to %s.%d.0 first", fromVersionSlice[0], toMinorVersion-1)
	}
	return nil
}

func getMajorMinorVersion(version string) string {
	vs := strings.Split(version, ".")
	if len(vs) < 2 {
		return ""
	}
	return vs[0] + vs[1]
}

func fillResourcesMap(dynamic dynamic.Interface, resourcesMap map[string][]unstructured.Unstructured, gvr schema.GroupVersionResource, listOptions metav1.ListOptions) error {
	// get custom resources
	objList, err := dynamic.Resource(gvr).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}
	for _, v := range objList.Items {
		namespace := v.GetNamespace()
		objArr := resourcesMap[namespace]
		objArr = append(objArr, v)
		resourcesMap[namespace] = objArr
	}
	return nil
}

// Breaking Change Upgrader.
// will handle the breaking change before upgrading to target version.

type Upgrader struct {
	Dynamic      dynamic.Interface
	FromVersion  string
	ToVersion    string
	ResourcesMap map[string][]unstructured.Unstructured
}

func (u *Upgrader) getWorkdir() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		fmt.Printf("can not get the current user: %v\n", err)
		return "", err
	}
	return fmt.Sprintf("%s/%s-%s-%s", currentUser.HomeDir, constant.AppName, u.FromVersion, u.ToVersion), nil
}

func (u *Upgrader) fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}
func (u *Upgrader) getUnstructuredFromFile(filePath string) (*unstructured.Unstructured, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	obj := map[string]interface{}{}
	if err = yaml.Unmarshal(content, &obj); err != nil {
		return nil, fmt.Errorf("unmarshal content of %s failed: %s", filePath, err.Error())
	}
	return &unstructured.Unstructured{Object: obj}, nil
}

func (u *Upgrader) SaveOldResources() error {
	handler := getUpgradeHandler(u.FromVersion, u.ToVersion)
	// if no upgrade handler found, ignore
	if handler == nil {
		return nil
	}
	objsMap, err := handler.snapshot(u.Dynamic)
	if err != nil {
		return err
	}

	workDir, err := u.getWorkdir()
	if err != nil {
		return err
	}
	fmt.Printf("\nTransform breaking changes in %s\n", workDir)
	u.ResourcesMap = objsMap
	// save to tmp work dir
	for namespace, objs := range objsMap {
		currWorkDir := fmt.Sprintf("%s/%s", workDir, namespace)
		if err = os.MkdirAll(currWorkDir, os.ModePerm); err != nil {
			return err
		}
		for i, v := range objs {
			filePath := fmt.Sprintf("%s/%s.yaml", currWorkDir, v.GetName())
			if u.fileExists(filePath) {
				obj, err := u.getUnstructuredFromFile(filePath)
				if err != nil {
					return err
				}
				objs[i] = *obj
				continue
			}
			// clear managedFields
			v.SetManagedFields(nil)
			yamlBytes, err := yaml.Marshal(v.Object)
			if err != nil {
				return err
			}
			err = os.WriteFile(filePath, yamlBytes, 0644)
			if err != nil {
				return err
			}
		}
		objsMap[namespace] = objs
	}
	return nil
}

func (u *Upgrader) TransformResourcesAndClear() error {
	handler := getUpgradeHandler(u.FromVersion, u.ToVersion)
	// if no upgrade handler found, ignore
	if handler == nil {
		return nil
	}
	if err := handler.transform(u.Dynamic, u.ResourcesMap); err != nil {
		return err
	}
	workDir, err := u.getWorkdir()
	if err != nil {
		return err
	}
	fmt.Printf("\nTransform breaking changes successfully, remove %s\n", workDir)
	// clear the tmp work dir
	return os.RemoveAll(workDir)
}
