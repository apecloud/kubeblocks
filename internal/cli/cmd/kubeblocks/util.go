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

package kubeblocks

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	"github.com/apecloud/kubeblocks/internal/constant"
)

func getGVRByCRD(crd *unstructured.Unstructured) (*schema.GroupVersionResource, error) {
	group, _, err := unstructured.NestedString(crd.Object, "spec", "group")
	if err != nil {
		return nil, nil
	}
	return &schema.GroupVersionResource{
		Group:    group,
		Version:  types.AppsAPIVersion,
		Resource: strings.Split(crd.GetName(), ".")[0],
	}, nil
}

// check if KubeBlocks has been installed
func checkIfKubeBlocksInstalled(client kubernetes.Interface) (bool, string, error) {
	kbDeploys, err := client.AppsV1().Deployments(metav1.NamespaceAll).List(context.TODO(),
		metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=" + types.KubeBlocksChartName})
	if err != nil {
		return false, "", err
	}

	if len(kbDeploys.Items) == 0 {
		return false, "", nil
	}

	var versions []string
	for _, deploy := range kbDeploys.Items {
		labels := deploy.GetLabels()
		if labels == nil {
			continue
		}
		if v, ok := labels["app.kubernetes.io/version"]; ok {
			versions = append(versions, v)
		}
	}
	return true, strings.Join(versions, " "), nil
}

func confirmUninstall(in io.Reader) error {
	const confirmStr = "uninstall-kubeblocks"
	_, err := prompt.NewPrompt(fmt.Sprintf("Please type \"%s\" to confirm:", confirmStr),
		func(input string) error {
			if input != confirmStr {
				return fmt.Errorf("typed \"%s\" does not match \"%s\"", input, confirmStr)
			}
			return nil
		}, in).Run()
	return err
}

func getHelmChartVersions(chart string) ([]*semver.Version, error) {
	errMsg := "failed to find the version information"
	// add repo, if exists, will update it
	if err := helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return nil, errors.Wrap(err, errMsg)
	}

	// get chart versions
	versions, err := helm.GetChartVersions(chart)
	if err != nil {
		return nil, errors.Wrap(err, errMsg)
	}
	return versions, nil
}

// buildResourceLabelSelectors builds labelSelectors that can be used to get all
// KubeBlocks resources and addons resources.
// KubeBlocks has two types of resources: KubeBlocks resources and addon resources,
// KubeBlocks resources are created by KubeBlocks itself, and addon resources are
// created by addons.
//
// KubeBlocks resources are labeled with "app.kubernetes.io/instance=types.KubeBlocksChartName",
// and most addon resources are labeled with "app.kubernetes.io/instance=<addon-prefix>-addon.Name",
// but some addon resources are labeled with "release=<addon-prefix>-addon.Name".
func buildResourceLabelSelectors(addons []*extensionsv1alpha1.Addon) []string {
	var (
		selectors []string
		releases  []string
		instances = []string{types.KubeBlocksChartName}
	)

	// releaseLabelAddons is a list of addons that use "release" label to label its resources
	// TODO: use a better way to avoid hard code, maybe add unified label to all addons
	releaseLabelAddons := []string{"prometheus"}
	for _, addon := range addons {
		addonReleaseName := fmt.Sprintf("%s-%s", types.AddonReleasePrefix, addon.Name)
		if slices.Contains(releaseLabelAddons, addon.Name) {
			releases = append(releases, addonReleaseName)
		} else {
			instances = append(instances, addonReleaseName)
		}
	}

	selectors = append(selectors, util.BuildLabelSelectorByNames("", instances))
	if len(releases) > 0 {
		selectors = append(selectors, fmt.Sprintf("release in (%s)", strings.Join(releases, ",")))
	}
	return selectors
}

// buildAddonLabelSelector builds labelSelector that can be used to get all build-in addons
func buildAddonLabelSelector() string {
	return fmt.Sprintf("%s=%s,%s=%s",
		constant.AppInstanceLabelKey, types.KubeBlocksReleaseName,
		constant.AppNameLabelKey, types.KubeBlocksChartName)
}
